package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/user/ai-workflow/internal/observability"
	"github.com/user/ai-workflow/internal/observability/logctx"
)

type stringSliceFlag []string

func (f *stringSliceFlag) String() string {
	return strings.Join(*f, " ")
}

func (f *stringSliceFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

type rpcEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type acpClient struct {
	reader *bufio.Reader
	writer io.Writer
	logger *slog.Logger
	mu     sync.Mutex
	nextID int64
}

func newACPClient(r io.Reader, w io.Writer, logger *slog.Logger) *acpClient {
	return &acpClient{
		reader: bufio.NewReader(r),
		writer: w,
		logger: logger,
	}
}

func (c *acpClient) sendRequest(ctx context.Context, method string, params any) (int64, error) {
	id := atomic.AddInt64(&c.nextID, 1)
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	writeCtx := ctx
	rpcID := fmt.Sprintf("%d", id)
	err := c.writeMessage(msg)
	if err != nil {
		if c.logger != nil {
			c.logger.ErrorContext(writeCtx, "send json-rpc request failed",
				"event", "acp.rpc.send_error",
				"method", method,
				"rpc_id", rpcID,
				"kind", "request",
				"direction", "out",
				"err", err,
			)
		}
		return id, err
	}
	if c.logger != nil {
		c.logger.DebugContext(writeCtx, "send json-rpc request",
			"event", "acp.rpc.send",
			"method", method,
			"rpc_id", rpcID,
			"kind", "request",
			"direction", "out",
		)
	}
	return id, nil
}

func (c *acpClient) sendNotification(ctx context.Context, method string, params any) error {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	writeCtx := ctx
	err := c.writeMessage(msg)
	if err != nil {
		if c.logger != nil {
			c.logger.ErrorContext(writeCtx, "send json-rpc notification failed",
				"event", "acp.rpc.send_error",
				"method", method,
				"kind", "notification",
				"direction", "out",
				"err", err,
			)
		}
		return err
	}
	if c.logger != nil {
		c.logger.DebugContext(writeCtx, "send json-rpc notification",
			"event", "acp.rpc.send",
			"method", method,
			"kind", "notification",
			"direction", "out",
		)
	}
	return nil
}

func (c *acpClient) sendResult(ctx context.Context, id json.RawMessage, result any) error {
	idValue, err := decodeID(id)
	if err != nil {
		return err
	}
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      idValue,
		"result":  result,
	}
	writeCtx := ctx
	rpcID, _ := normalizeID(id)
	if rpcID == "" {
		rpcID = "<unknown>"
	}
	err = c.writeMessage(msg)
	if err != nil {
		if c.logger != nil {
			c.logger.ErrorContext(writeCtx, "send json-rpc response failed",
				"event", "acp.rpc.send_error",
				"rpc_id", rpcID,
				"kind", "response",
				"direction", "out",
				"err", err,
			)
		}
		return err
	}
	if c.logger != nil {
		c.logger.DebugContext(writeCtx, "send json-rpc response",
			"event", "acp.rpc.send",
			"rpc_id", rpcID,
			"kind", "response",
			"direction", "out",
		)
	}
	return nil
}

func (c *acpClient) sendError(ctx context.Context, id json.RawMessage, code int, message string) error {
	idValue, err := decodeID(id)
	if err != nil {
		return err
	}
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      idValue,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	writeCtx := ctx
	rpcID, _ := normalizeID(id)
	if rpcID == "" {
		rpcID = "<unknown>"
	}
	err = c.writeMessage(msg)
	if err != nil {
		if c.logger != nil {
			c.logger.ErrorContext(writeCtx, "send json-rpc error response failed",
				"event", "acp.rpc.send_error",
				"rpc_id", rpcID,
				"kind", "response",
				"direction", "out",
				"err_code", code,
				"err", err,
			)
		}
		return err
	}
	if c.logger != nil {
		c.logger.WarnContext(writeCtx, "send json-rpc error response",
			"event", "acp.rpc.send_error_response",
			"rpc_id", rpcID,
			"kind", "response",
			"direction", "out",
			"err_code", code,
			"err_message", message,
		)
	}
	return nil
}

func (c *acpClient) writeMessage(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if bytes.Contains(data, []byte{'\n'}) {
		return errors.New("message contains newline, violates ACP stdio transport")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.writer.Write(data); err != nil {
		return err
	}
	_, err = c.writer.Write([]byte{'\n'})
	return err
}

func (c *acpClient) readMessage(ctx context.Context) (rpcEnvelope, int, error) {
	for {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			if c.logger != nil {
				c.logger.ErrorContext(ctx, "read agent stdout failed",
					"event", "acp.rpc.recv_error",
					"err", err,
				)
			}
			return rpcEnvelope{}, 0, err
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		payloadBytes := len(line)

		var env rpcEnvelope
		if err := json.Unmarshal(line, &env); err != nil {
			if c.logger != nil {
				c.logger.ErrorContext(ctx, "invalid json-rpc message",
					"event", "acp.rpc.decode_error",
					"payload_bytes", payloadBytes,
					"raw_preview", truncateForLog(string(line), 240),
					"err", err,
				)
			}
			return rpcEnvelope{}, payloadBytes, fmt.Errorf("invalid json-rpc message: %w (raw: %s)", err, string(line))
		}
		c.logInbound(ctx, env, payloadBytes)
		return env, payloadBytes, nil
	}
}

type runState struct {
	assistantText strings.Builder
	logger        *slog.Logger
	updateCounter uint64
	updateSample  uint64
}

func waitForResponse(ctx context.Context, c *acpClient, id int64, state *runState) (json.RawMessage, error) {
	wantID := fmt.Sprintf("%d", id)
	for {
		msg, _, err := c.readMessage(ctx)
		if err != nil {
			return nil, err
		}

		if msg.Method != "" {
			if err := handleInbound(ctx, c, msg, state); err != nil {
				return nil, err
			}
			continue
		}

		gotID, err := normalizeID(msg.ID)
		if err != nil {
			return nil, err
		}
		if gotID != wantID {
			if c.logger != nil {
				c.logger.DebugContext(ctx, "ignore unmatched response",
					"event", "acp.rpc.unmatched_response",
					"expect_rpc_id", wantID,
					"got_rpc_id", gotID,
				)
			}
			continue
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("rpc error(code=%d): %s", msg.Error.Code, msg.Error.Message)
		}
		return msg.Result, nil
	}
}

func handleInbound(ctx context.Context, c *acpClient, msg rpcEnvelope, state *runState) error {
	switch msg.Method {
	case "session/update":
		return handleSessionUpdate(ctx, msg.Params, state)
	case "session/request_permission":
		return handlePermissionRequest(ctx, c, msg)
	default:
		if len(msg.ID) == 0 {
			if c.logger != nil {
				c.logger.DebugContext(ctx, "ignore unsupported notification",
					"event", "acp.rpc.unsupported_notification",
					"method", msg.Method,
				)
			}
			return nil
		}
		sendErr := c.sendError(ctx, msg.ID, -32601, "method not supported by acp-smoke client")
		if c.logger != nil {
			rpcID, _ := normalizeID(msg.ID)
			c.logger.WarnContext(ctx, "unsupported inbound request",
				"event", "acp.rpc.unsupported_request",
				"method", msg.Method,
				"rpc_id", rpcID,
				"err", sendErr,
			)
		}
		return sendErr
	}
}

func handlePermissionRequest(ctx context.Context, c *acpClient, msg rpcEnvelope) error {
	type permissionOption struct {
		OptionID string `json:"optionId"`
		Kind     string `json:"kind"`
		Name     string `json:"name"`
	}
	type permissionParams struct {
		Options []permissionOption `json:"options"`
	}
	var params permissionParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		if c.logger != nil {
			c.logger.ErrorContext(ctx, "invalid permission request params",
				"event", "acp.permission.parse_error",
				"err", err,
			)
		}
		return c.sendError(ctx, msg.ID, -32602, "invalid session/request_permission params")
	}

	if len(params.Options) == 0 {
		if c.logger != nil {
			c.logger.WarnContext(ctx, "permission options empty, fallback cancelled",
				"event", "acp.permission.auto_cancelled",
			)
		}
		return c.sendResult(ctx, msg.ID, map[string]any{
			"outcome": map[string]any{
				"outcome": "cancelled",
			},
		})
	}

	selected := params.Options[0].OptionID
	for _, opt := range params.Options {
		if opt.Kind == "allow_once" {
			selected = opt.OptionID
			break
		}
	}

	if c.logger != nil {
		c.logger.InfoContext(ctx, "auto-select permission option",
			"event", "acp.permission.auto_selected",
			"selected_option_id", selected,
			"options_count", len(params.Options),
		)
	}
	return c.sendResult(ctx, msg.ID, map[string]any{
		"outcome": map[string]any{
			"outcome":  "selected",
			"optionId": selected,
		},
	})
}

func handleSessionUpdate(ctx context.Context, raw json.RawMessage, state *runState) error {
	type contentText struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type updatePayload struct {
		SessionID string `json:"sessionId"`
		Update    struct {
			SessionUpdate string          `json:"sessionUpdate"`
			Content       json.RawMessage `json:"content"`
			Status        string          `json:"status"`
			ToolCallID    string          `json:"toolCallId"`
			Title         string          `json:"title"`
		} `json:"update"`
	}

	var payload updatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("parse session/update failed: %w", err)
	}

	state.updateCounter++
	logThisUpdate := shouldLogUpdate(payload.Update.SessionUpdate, state.updateCounter, state.updateSample)

	switch payload.Update.SessionUpdate {
	case "agent_message_chunk":
		var chunk contentText
		if err := json.Unmarshal(payload.Update.Content, &chunk); err == nil && chunk.Type == "text" {
			state.assistantText.WriteString(chunk.Text)
			if state.logger != nil && logThisUpdate {
				state.logger.DebugContext(ctx, "session update chunk",
					"event", "acp.session.update",
					"update", payload.Update.SessionUpdate,
					"chunk_len", len(chunk.Text),
					"chunk_preview", truncateForLog(chunk.Text, 120),
				)
			}
		}
	case "tool_call":
		if state.logger != nil {
			state.logger.InfoContext(ctx, "tool call announced",
				"event", "acp.session.update",
				"update", payload.Update.SessionUpdate,
				"tool_call_id", payload.Update.ToolCallID,
				"status", payload.Update.Status,
				"title", payload.Update.Title,
			)
		}
	case "tool_call_update":
		if state.logger != nil {
			state.logger.InfoContext(ctx, "tool call status updated",
				"event", "acp.session.update",
				"update", payload.Update.SessionUpdate,
				"tool_call_id", payload.Update.ToolCallID,
				"status", payload.Update.Status,
			)
		}
	case "plan":
		if state.logger != nil {
			state.logger.InfoContext(ctx, "execution plan updated",
				"event", "acp.session.update",
				"update", payload.Update.SessionUpdate,
			)
		}
	default:
		if state.logger != nil && logThisUpdate {
			state.logger.DebugContext(ctx, "session update received",
				"event", "acp.session.update",
				"update", payload.Update.SessionUpdate,
			)
		}
	}
	return nil
}

func shouldLogUpdate(update string, seq uint64, every uint64) bool {
	if every <= 1 {
		return true
	}
	switch update {
	case "agent_message_chunk", "usage_update":
		return seq%every == 1
	default:
		return true
	}
}

func decodeID(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, errors.New("missing id field")
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("decode id failed: %w", err)
	}
	return v, nil
}

func normalizeID(raw json.RawMessage) (string, error) {
	v, err := decodeID(raw)
	if err != nil {
		return "", err
	}
	switch id := v.(type) {
	case float64:
		return fmt.Sprintf("%.0f", id), nil
	case string:
		return id, nil
	default:
		return fmt.Sprintf("%v", id), nil
	}
}

type initializeResult struct {
	ProtocolVersion int `json:"protocolVersion"`
	AgentInfo       struct {
		Name    string `json:"name"`
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"agentInfo"`
}

type sessionNewResult struct {
	SessionID string `json:"sessionId"`
}

type promptResult struct {
	StopReason string `json:"stopReason"`
}

func callRPC[T any](ctx context.Context, c *acpClient, state *runState, method string, params any) (T, error) {
	var zero T

	start := time.Now()
	id, err := c.sendRequest(ctx, method, params)
	if err != nil {
		return zero, err
	}

	raw, err := waitForResponse(ctx, c, id, state)
	latencyMS := time.Since(start).Milliseconds()
	if err != nil {
		if c.logger != nil {
			rpcID := fmt.Sprintf("%d", id)
			c.logger.ErrorContext(ctx, "rpc call failed",
				"event", "acp.rpc.call_error",
				"method", method,
				"rpc_id", rpcID,
				"latency_ms", latencyMS,
				"err", err,
			)
		}
		return zero, err
	}

	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		if c.logger != nil {
			rpcID := fmt.Sprintf("%d", id)
			c.logger.ErrorContext(ctx, "rpc result decode failed",
				"event", "acp.rpc.decode_result_error",
				"method", method,
				"rpc_id", rpcID,
				"latency_ms", latencyMS,
				"err", err,
			)
		}
		return zero, err
	}

	if c.logger != nil {
		rpcID := fmt.Sprintf("%d", id)
		c.logger.InfoContext(ctx, "rpc call completed",
			"event", "acp.rpc.call_complete",
			"method", method,
			"rpc_id", rpcID,
			"latency_ms", latencyMS,
		)
	}
	return out, nil
}

func main() {
	var (
		agentCmd    string
		cwd         string
		promptText  string
		timeout     time.Duration
		cancelAfter time.Duration
		agentArgs   stringSliceFlag
		logFormat   string
		logLevelRaw string
		logSource   bool
		updateEvery int
	)

	flag.StringVar(&agentCmd, "agent-cmd", "npx", "用于启动 codex-acp 的命令")
	flag.Var(&agentArgs, "agent-arg", "用于启动 codex-acp 的参数（可重复指定）")
	flag.StringVar(&cwd, "cwd", ".", "session/new 的工作目录（会自动转绝对路径）")
	flag.StringVar(&promptText, "prompt", "请回复：ACP_GO_OK", "session/prompt 文本")
	flag.DurationVar(&timeout, "timeout", 3*time.Minute, "整次调用超时时间")
	flag.DurationVar(&cancelAfter, "cancel-after", 0, "发送 session/prompt 后多久触发 session/cancel（0=不取消）")
	flag.StringVar(&logFormat, "log-format", "text", "日志格式：text|json")
	flag.StringVar(&logLevelRaw, "log-level", "info", "日志级别：debug|info|warn|error")
	flag.BoolVar(&logSource, "log-source", false, "日志是否包含源码位置")
	flag.IntVar(&updateEvery, "log-update-sample", 5, "session/update 降噪采样间隔（<=1 表示不采样）")
	flag.Parse()

	if len(agentArgs) == 0 {
		agentArgs = stringSliceFlag{"-y", "@zed-industries/codex-acp@latest"}
	}
	if updateEvery <= 0 {
		updateEvery = 1
	}

	absCWD, err := filepath.Abs(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: 解析 cwd 失败: %v\n", err)
		os.Exit(1)
	}

	logLevel, err := logctx.ParseLevel(logLevelRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: 解析日志级别失败: %v\n", err)
		os.Exit(1)
	}
	logger := logctx.New(logctx.Options{
		Writer:    os.Stderr,
		Format:    logFormat,
		Level:     logLevel,
		AddSource: logSource,
		BaseAttrs: []slog.Attr{
			slog.String("component", "acp-smoke"),
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ctx, _ = observability.EnsureTraceID(ctx, "")
	connID := "conn-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 36)
	ctx = logctx.WithField(ctx, "conn_id", connID)

	cmd := exec.CommandContext(ctx, agentCmd, []string(agentArgs)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: 获取 stdin pipe 失败: %v\n", err)
		os.Exit(1)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: 获取 stdout pipe 失败: %v\n", err)
		os.Exit(1)
	}
	cmd.Stderr = os.Stderr

	logger.InfoContext(ctx, "start agent process",
		"event", "acp.agent.start",
		"agent_cmd", agentCmd,
		"agent_args", strings.Join(agentArgs, " "),
		"cwd", absCWD,
		"prompt_len", len(promptText),
	)
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: 启动 agent 失败: %v\n", err)
		os.Exit(1)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	client := newACPClient(stdout, stdin, logger)
	state := &runState{
		logger:       logger,
		updateSample: uint64(updateEvery),
	}

	initRes, err := callRPC[initializeResult](ctx, client, state, "initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs": map[string]any{
				"readTextFile":  false,
				"writeTextFile": false,
			},
			"terminal": false,
		},
		"clientInfo": map[string]any{
			"name":    "acp-go-smoke",
			"title":   "ACP Go Smoke",
			"version": "0.1.0",
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: initialize 失败: %v\n", err)
		os.Exit(1)
	}
	logger.InfoContext(ctx, "initialize completed",
		"event", "acp.initialize.ok",
		"protocol_version", initRes.ProtocolVersion,
		"agent_name", initRes.AgentInfo.Name,
		"agent_version", initRes.AgentInfo.Version,
	)

	sessionRes, err := callRPC[sessionNewResult](ctx, client, state, "session/new", map[string]any{
		"cwd":        absCWD,
		"mcpServers": []any{},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: session/new 失败: %v\n", err)
		os.Exit(1)
	}
	if sessionRes.SessionID == "" {
		fmt.Fprintln(os.Stderr, "error: session/new 未返回 sessionId")
		os.Exit(1)
	}
	ctx = logctx.WithField(ctx, "session_id", sessionRes.SessionID)
	logger.InfoContext(ctx, "session created",
		"event", "acp.session.new.ok",
	)

	if cancelAfter > 0 {
		go func() {
			timer := time.NewTimer(cancelAfter)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				if err := client.sendNotification(ctx, "session/cancel", map[string]any{
					"sessionId": sessionRes.SessionID,
				}); err != nil {
					logger.ErrorContext(ctx, "send cancel notification failed",
						"event", "acp.cancel.send_error",
						"method", "session/cancel",
						"err", err,
					)
					return
				}
				logger.WarnContext(ctx, "cancel notification sent",
					"event", "acp.cancel.sent",
					"method", "session/cancel",
				)
			}
		}()
	}

	promptRes, err := callRPC[promptResult](ctx, client, state, "session/prompt", map[string]any{
		"sessionId": sessionRes.SessionID,
		"prompt": []map[string]any{
			{
				"type": "text",
				"text": promptText,
			},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: session/prompt 失败: %v\n", err)
		os.Exit(1)
	}
	logger.InfoContext(ctx, "prompt turn completed",
		"event", "acp.turn.completed",
		"stop_reason", promptRes.StopReason,
	)

	answer := strings.TrimSpace(state.assistantText.String())
	if answer == "" {
		fmt.Println("assistant_text: <empty>")
	} else {
		fmt.Printf("assistant_text:\n%s\n", answer)
	}

	_ = stdin.Close()
	select {
	case err := <-waitDone:
		if err != nil {
			logger.WarnContext(ctx, "agent exited with error",
				"event", "acp.agent.exit_error",
				"err", err,
			)
		}
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		logger.WarnContext(ctx, "force kill agent process after grace timeout",
			"event", "acp.agent.killed",
		)
	}
}

func (c *acpClient) logInbound(ctx context.Context, env rpcEnvelope, payloadBytes int) {
	if c.logger == nil {
		return
	}

	args := []any{
		"event", "acp.rpc.recv",
		"direction", "in",
		"payload_bytes", payloadBytes,
	}
	if env.Method != "" {
		args = append(args, "method", env.Method)
		if len(env.ID) == 0 {
			args = append(args, "kind", "notification")
		} else {
			args = append(args, "kind", "request")
		}
	} else {
		args = append(args, "kind", "response")
	}
	if len(env.ID) > 0 {
		if rpcID, err := normalizeID(env.ID); err == nil && rpcID != "" {
			args = append(args, "rpc_id", rpcID)
		}
	}

	c.logger.DebugContext(ctx, "recv json-rpc message", args...)
}

func truncateForLog(in string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(in) <= max {
		return in
	}
	if max <= 3 {
		return in[:max]
	}
	return in[:max-3] + "..."
}
