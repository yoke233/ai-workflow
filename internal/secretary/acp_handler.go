package secretary

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	acpproto "github.com/coder/acp-go-sdk"
	"github.com/yoke233/ai-workflow/internal/acpclient"
	"github.com/yoke233/ai-workflow/internal/core"
)

type acpEventPublisher interface {
	Publish(evt core.Event)
}

type ChatRunEventRecorder interface {
	AppendChatRunEvent(event core.ChatRunEvent) error
}

type ACPHandlerSessionContext struct {
	SessionID    string
	ChangedFiles []string
}

type ACPHandler struct {
	acpclient.NopHandler

	cwd           string
	sessionID     string
	chatSessionID string
	projectID     string
	publisher     acpEventPublisher
	recorder      ChatRunEventRecorder

	mu          sync.Mutex
	changedSet  map[string]struct{}
	changedList []string

	terminalSeq atomic.Uint64
	terminalMu  sync.Mutex
	terminals   map[string]*acpTerminalState
}

type acpTerminalState struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	outbuf   *lockedBuffer
	done     chan struct{}
	waitErr  error
	exitCode *int
	signal   *string
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) Snapshot(maxBytes int) (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data := b.buf.Bytes()
	if maxBytes > 0 && len(data) > maxBytes {
		return string(data[len(data)-maxBytes:]), true
	}
	return string(data), false
}

var _ acpproto.Client = (*ACPHandler)(nil)
var _ acpclient.EventHandler = (*ACPHandler)(nil)

func NewACPHandler(cwd string, sessionID string, publisher acpEventPublisher) *ACPHandler {
	return &ACPHandler{
		cwd:        strings.TrimSpace(cwd),
		sessionID:  strings.TrimSpace(sessionID),
		publisher:  publisher,
		changedSet: make(map[string]struct{}),
		terminals:  make(map[string]*acpTerminalState),
	}
}

func (h *ACPHandler) SetRunEventRecorder(recorder ChatRunEventRecorder) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.recorder = recorder
}

func (h *ACPHandler) SetSessionID(sessionID string) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessionID = strings.TrimSpace(sessionID)
}

func (h *ACPHandler) SetProjectID(projectID string) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.projectID = strings.TrimSpace(projectID)
}

func (h *ACPHandler) SetChatSessionID(chatSessionID string) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.chatSessionID = strings.TrimSpace(chatSessionID)
}

func (h *ACPHandler) ReadTextFile(_ context.Context, req acpproto.ReadTextFileRequest) (acpproto.ReadTextFileResponse, error) {
	if h == nil {
		return acpproto.ReadTextFileResponse{}, errors.New("acp handler is nil")
	}

	targetPath, _, err := h.normalizePathInScope(req.Path)
	if err != nil {
		return acpproto.ReadTextFileResponse{}, err
	}
	raw, err := os.ReadFile(targetPath)
	if err != nil {
		return acpproto.ReadTextFileResponse{}, fmt.Errorf("read file: %w", err)
	}

	content := string(raw)
	content = applyReadLineWindow(content, req.Line, req.Limit)
	return acpproto.ReadTextFileResponse{Content: content}, nil
}

func (h *ACPHandler) RequestPermission(_ context.Context, req acpproto.RequestPermissionRequest) (acpproto.RequestPermissionResponse, error) {
	if h == nil {
		return acpproto.RequestPermissionResponse{}, errors.New("acp handler is nil")
	}
	if selected := selectPermissionOptionID(req.Options); selected != "" {
		return acpproto.RequestPermissionResponse{
			Outcome: acpproto.RequestPermissionOutcome{
				Selected: &acpproto.RequestPermissionOutcomeSelected{
					Outcome:  "selected",
					OptionId: acpproto.PermissionOptionId(selected),
				},
			},
		}, nil
	}
	return acpproto.RequestPermissionResponse{
		Outcome: acpproto.RequestPermissionOutcome{
			Cancelled: &acpproto.RequestPermissionOutcomeCancelled{Outcome: "cancelled"},
		},
	}, nil
}

func (h *ACPHandler) CreateTerminal(_ context.Context, req acpproto.CreateTerminalRequest) (acpproto.CreateTerminalResponse, error) {
	if h == nil {
		return acpproto.CreateTerminalResponse{}, errors.New("acp handler is nil")
	}
	command := strings.TrimSpace(req.Command)
	if command == "" {
		return acpproto.CreateTerminalResponse{}, errors.New("terminal command is required")
	}

	cwd, err := h.normalizeDirInScope(stringPtrValue(req.Cwd))
	if err != nil {
		return acpproto.CreateTerminalResponse{}, err
	}

	commandParts := append([]string{command}, req.Args...)
	cmd := exec.Command(commandParts[0], commandParts[1:]...)
	cmd.Dir = cwd
	cmd.Env = mergeTerminalEnv(req.Env)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return acpproto.CreateTerminalResponse{}, fmt.Errorf("create terminal stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return acpproto.CreateTerminalResponse{}, fmt.Errorf("create terminal stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return acpproto.CreateTerminalResponse{}, fmt.Errorf("create terminal stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return acpproto.CreateTerminalResponse{}, fmt.Errorf("start terminal command: %w", err)
	}

	terminalID := fmt.Sprintf("term-%d", h.terminalSeq.Add(1))
	state := &acpTerminalState{
		cmd:    cmd,
		stdin:  stdin,
		outbuf: &lockedBuffer{},
		done:   make(chan struct{}),
	}

	go func() {
		_, _ = io.Copy(state.outbuf, stdout)
	}()
	go func() {
		_, _ = io.Copy(state.outbuf, stderr)
	}()
	go func() {
		waitErr := cmd.Wait()
		if cmd.ProcessState != nil {
			code := cmd.ProcessState.ExitCode()
			state.exitCode = &code
		}
		state.waitErr = waitErr
		close(state.done)
	}()

	h.terminalMu.Lock()
	h.terminals[terminalID] = state
	h.terminalMu.Unlock()
	return acpproto.CreateTerminalResponse{TerminalId: terminalID}, nil
}

func (h *ACPHandler) KillTerminalCommand(_ context.Context, req acpproto.KillTerminalCommandRequest) (acpproto.KillTerminalCommandResponse, error) {
	state, ok := h.getTerminal(req.TerminalId)
	if !ok {
		return acpproto.KillTerminalCommandResponse{}, nil
	}
	if state.cmd == nil || state.cmd.Process == nil {
		return acpproto.KillTerminalCommandResponse{}, nil
	}
	if err := state.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return acpproto.KillTerminalCommandResponse{}, fmt.Errorf("kill terminal process: %w", err)
	}
	return acpproto.KillTerminalCommandResponse{}, nil
}

func (h *ACPHandler) TerminalOutput(_ context.Context, req acpproto.TerminalOutputRequest) (acpproto.TerminalOutputResponse, error) {
	state, ok := h.getTerminal(req.TerminalId)
	if !ok {
		return acpproto.TerminalOutputResponse{}, fmt.Errorf("terminal %q not found", req.TerminalId)
	}
	output, truncated := state.outbuf.Snapshot(0)
	return acpproto.TerminalOutputResponse{
		Output:    output,
		Truncated: truncated,
	}, nil
}

func (h *ACPHandler) ReleaseTerminal(_ context.Context, req acpproto.ReleaseTerminalRequest) (acpproto.ReleaseTerminalResponse, error) {
	state, ok := h.removeTerminal(req.TerminalId)
	if !ok {
		return acpproto.ReleaseTerminalResponse{}, nil
	}
	if state.stdin != nil {
		_ = state.stdin.Close()
	}
	return acpproto.ReleaseTerminalResponse{}, nil
}

func (h *ACPHandler) WaitForTerminalExit(ctx context.Context, req acpproto.WaitForTerminalExitRequest) (acpproto.WaitForTerminalExitResponse, error) {
	state, ok := h.getTerminal(req.TerminalId)
	if !ok {
		return acpproto.WaitForTerminalExitResponse{}, fmt.Errorf("terminal %q not found", req.TerminalId)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-state.done:
		return acpproto.WaitForTerminalExitResponse{
			ExitCode: state.exitCode,
			Signal:   state.signal,
		}, nil
	case <-ctx.Done():
		return acpproto.WaitForTerminalExitResponse{}, ctx.Err()
	}
}

func (h *ACPHandler) WriteTextFile(_ context.Context, req acpproto.WriteTextFileRequest) (acpproto.WriteTextFileResponse, error) {
	if h == nil {
		return acpproto.WriteTextFileResponse{}, errors.New("acp handler is nil")
	}

	targetPath, relPath, err := h.normalizePathInScope(req.Path)
	if err != nil {
		return acpproto.WriteTextFileResponse{}, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return acpproto.WriteTextFileResponse{}, fmt.Errorf("ensure parent dir: %w", err)
	}

	content := []byte(req.Content)
	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		return acpproto.WriteTextFileResponse{}, fmt.Errorf("write file %q: %w", relPath, err)
	}

	filePaths := h.recordChangedFile(relPath)
	h.publishFilesChanged(filePaths)

	return acpproto.WriteTextFileResponse{}, nil
}

func (h *ACPHandler) SessionUpdate(context.Context, acpproto.SessionNotification) error {
	return nil
}

func (h *ACPHandler) SessionContext() ACPHandlerSessionContext {
	if h == nil {
		return ACPHandlerSessionContext{}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	changed := make([]string, len(h.changedList))
	copy(changed, h.changedList)
	sessionID := h.sessionID
	if trimmedChatSessionID := strings.TrimSpace(h.chatSessionID); trimmedChatSessionID != "" {
		sessionID = trimmedChatSessionID
	}
	return ACPHandlerSessionContext{
		SessionID:    sessionID,
		ChangedFiles: changed,
	}
}

func (h *ACPHandler) HandleSessionUpdate(_ context.Context, update acpclient.SessionUpdate) error {
	if h == nil {
		return nil
	}

	h.mu.Lock()
	projectID := strings.TrimSpace(h.projectID)
	chatSessionID := strings.TrimSpace(h.chatSessionID)
	agentSessionID := strings.TrimSpace(h.sessionID)
	recorder := h.recorder
	h.mu.Unlock()
	if chatSessionID == "" {
		chatSessionID = agentSessionID
	}
	if agentSessionID == "" {
		agentSessionID = strings.TrimSpace(update.SessionID)
	}

	data := map[string]string{
		"session_id":       chatSessionID,
		"agent_session_id": agentSessionID,
	}
	if rawUpdate := strings.TrimSpace(update.RawUpdateJSON); rawUpdate != "" {
		data["acp_update_json"] = rawUpdate
	}

	updateType := strings.TrimSpace(update.Type)
	if recorder != nil && !isACPChunkUpdateType(updateType) && chatSessionID != "" && projectID != "" {
		payload := map[string]any{
			"session_id":       chatSessionID,
			"agent_session_id": agentSessionID,
		}
		if text := strings.TrimSpace(update.Text); text != "" {
			payload["text"] = text
		}
		if status := strings.TrimSpace(update.Status); status != "" {
			payload["status"] = status
		}
		if rawUpdate := strings.TrimSpace(update.RawUpdateJSON); rawUpdate != "" {
			var acpPayload any
			if err := json.Unmarshal([]byte(rawUpdate), &acpPayload); err == nil {
				payload["acp"] = acpPayload
			} else {
				payload["acp_raw"] = rawUpdate
			}
		}
		if err := recorder.AppendChatRunEvent(core.ChatRunEvent{
			SessionID:  chatSessionID,
			ProjectID:  projectID,
			EventType:  string(core.EventChatRunUpdate),
			UpdateType: updateType,
			Payload:    payload,
			CreatedAt:  time.Now().UTC(),
		}); err != nil {
			log.Printf("[acp] persist chat run event failed project_id=%s session_id=%s update_type=%s err=%v", projectID, chatSessionID, updateType, err)
		}
	}

	if h.publisher != nil {
		h.publisher.Publish(core.Event{
			Type:      core.EventChatRunUpdate,
			ProjectID: projectID,
			Data:      data,
			Timestamp: time.Now(),
		})
	}
	return nil
}

func isACPChunkUpdateType(updateType string) bool {
	switch strings.TrimSpace(updateType) {
	case "agent_message_chunk", "assistant_message_chunk", "message_chunk":
		return true
	default:
		return false
	}
}

func (h *ACPHandler) normalizePathInScope(rawPath string) (string, string, error) {
	cwd := strings.TrimSpace(h.cwd)
	if cwd == "" {
		return "", "", errors.New("handler cwd is required")
	}
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return "", "", fmt.Errorf("resolve cwd: %w", err)
	}

	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", "", errors.New("write file path is required")
	}

	target := trimmed
	if !filepath.IsAbs(target) {
		target = filepath.Join(cwdAbs, target)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", "", fmt.Errorf("resolve path: %w", err)
	}

	rel, err := filepath.Rel(cwdAbs, targetAbs)
	if err != nil {
		return "", "", fmt.Errorf("check path scope: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", "", fmt.Errorf("path %q is outside cwd scope", trimmed)
	}

	rel = filepath.ToSlash(filepath.Clean(rel))
	return targetAbs, rel, nil
}

func (h *ACPHandler) recordChangedFile(path string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.changedSet[path]; !ok {
		h.changedSet[path] = struct{}{}
		h.changedList = append(h.changedList, path)
	}

	out := make([]string, len(h.changedList))
	copy(out, h.changedList)
	return out
}

func (h *ACPHandler) publishFilesChanged(filePaths []string) {
	if h.publisher == nil {
		return
	}

	h.mu.Lock()
	projectID := strings.TrimSpace(h.projectID)
	sessionID := strings.TrimSpace(h.chatSessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(h.sessionID)
	}
	h.mu.Unlock()

	h.publisher.Publish(core.Event{
		Type:      core.EventSecretaryFilesChanged,
		ProjectID: projectID,
		Data: map[string]string{
			"session_id": sessionID,
			"file_paths": strings.Join(filePaths, ","),
		},
		Timestamp: time.Now(),
	})
}

func (h *ACPHandler) normalizeDirInScope(rawDir string) (string, error) {
	cwd := strings.TrimSpace(h.cwd)
	if cwd == "" {
		return "", errors.New("handler cwd is required")
	}
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve cwd: %w", err)
	}

	trimmedDir := strings.TrimSpace(rawDir)
	if trimmedDir == "" {
		return cwdAbs, nil
	}

	target := trimmedDir
	if !filepath.IsAbs(target) {
		target = filepath.Join(cwdAbs, target)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve terminal cwd: %w", err)
	}

	rel, err := filepath.Rel(cwdAbs, targetAbs)
	if err != nil {
		return "", fmt.Errorf("check terminal cwd scope: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("terminal cwd %q is outside handler cwd scope", trimmedDir)
	}
	return targetAbs, nil
}

func applyReadLineWindow(content string, line *int, limit *int) string {
	if line == nil && limit == nil {
		return content
	}

	start := 1
	if line != nil && *line > 0 {
		start = *line
	}

	lines := strings.Split(content, "\n")
	if start > len(lines) {
		return ""
	}

	from := start - 1
	to := len(lines)
	if limit != nil && *limit > 0 {
		max := from + *limit
		if max < to {
			to = max
		}
	}
	return strings.Join(lines[from:to], "\n")
}

func selectPermissionOptionID(options []acpproto.PermissionOption) string {
	if len(options) == 0 {
		return ""
	}

	preferred := []string{"allow_once", "allow_always"}
	for _, want := range preferred {
		for _, option := range options {
			if strings.EqualFold(strings.TrimSpace(string(option.OptionId)), want) {
				return strings.TrimSpace(string(option.OptionId))
			}
		}
	}
	for _, option := range options {
		if id := strings.TrimSpace(string(option.OptionId)); id != "" {
			return id
		}
	}
	return ""
}

func (h *ACPHandler) getTerminal(terminalID string) (*acpTerminalState, bool) {
	trimmed := strings.TrimSpace(terminalID)
	if h == nil || trimmed == "" {
		return nil, false
	}
	h.terminalMu.Lock()
	defer h.terminalMu.Unlock()
	state, ok := h.terminals[trimmed]
	return state, ok
}

func (h *ACPHandler) removeTerminal(terminalID string) (*acpTerminalState, bool) {
	trimmed := strings.TrimSpace(terminalID)
	if h == nil || trimmed == "" {
		return nil, false
	}
	h.terminalMu.Lock()
	defer h.terminalMu.Unlock()
	state, ok := h.terminals[trimmed]
	if ok {
		delete(h.terminals, trimmed)
	}
	return state, ok
}

func isTerminalDone(state *acpTerminalState) bool {
	if state == nil {
		return true
	}
	select {
	case <-state.done:
		return true
	default:
		return false
	}
}

func mergeTerminalEnv(extra []acpproto.EnvVariable) []string {
	env := os.Environ()
	for _, item := range extra {
		key := strings.TrimSpace(item.Name)
		if key == "" {
			continue
		}
		env = append(env, key+"="+item.Value)
	}
	return env
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
