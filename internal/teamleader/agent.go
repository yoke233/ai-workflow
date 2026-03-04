package teamleader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	acpproto "github.com/coder/acp-go-sdk"
	"github.com/yoke233/ai-workflow/internal/acpclient"
	"github.com/yoke233/ai-workflow/internal/core"
)

const (
	defaultTemplatePath     = "configs/prompts/team_leader.tmpl"
	defaultMaxTurns         = 12
	defaultRoleID           = "team_leader"
	defaultTeamLeaderRoleID = "team_leader"
)

var defaultAllowedTools = []string{"Read(*)"}

// Request is the input payload for TeamLeader decomposition.
type Request struct {
	Conversation string
	ProjectName  string
	TechStack    string
	RepoPath     string
	Role         string
	SourceFiles  []string

	// FileContents is optional file-based plan input for parser extraction.
	// key: file path, value: raw file content snapshot.
	FileContents map[string]string

	// Regeneration input fields (rules 10.3).
	OriginalConversationSummary string
	PreviousTaskPlanJSON        string
	AIReviewSummaryJSON         string
	HumanFeedbackJSON           string

	WorkDir  string
	Env      map[string]string
	MaxTurns int
	Timeout  time.Duration
}

type promptVars struct {
	Conversation                string
	ProjectName                 string
	TechStack                   string
	RepoPath                    string
	OriginalConversationSummary string
	PreviousTaskPlanJSON        string
	AIReviewSummaryJSON         string
	HumanFeedbackJSON           string
}

// Agent is the TeamLeader decomposition driver based on core.AgentPlugin.
type Agent struct {
	agent        core.AgentPlugin
	runtime      core.RuntimePlugin
	promptTmpl   *template.Template
	allowedTools []string
	maxTurns     int
}

type TeamLeaderSessionClient interface {
	LoadSession(ctx context.Context, req acpproto.LoadSessionRequest) (acpproto.SessionId, error)
	NewSession(ctx context.Context, req acpproto.NewSessionRequest) (acpproto.SessionId, error)
}

func NewAgent(agent core.AgentPlugin, runtime core.RuntimePlugin) (*Agent, error) {
	return NewAgentWithTemplatePath(agent, runtime, "")
}

func NewAgentWithTemplatePath(agent core.AgentPlugin, runtime core.RuntimePlugin, templatePath string) (*Agent, error) {
	if agent == nil {
		return nil, errors.New("agent plugin is required")
	}

	content, resolvedPath, err := readTemplateContent(templatePath)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(filepath.Base(resolvedPath)).Option("missingkey=error").Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse TeamLeader template %q: %w", resolvedPath, err)
	}

	return &Agent{
		agent:        agent,
		runtime:      runtime,
		promptTmpl:   tmpl,
		allowedTools: copyStrings(defaultAllowedTools),
		maxTurns:     defaultMaxTurns,
	}, nil
}

// BuildCommand renders prompt and delegates to core.AgentPlugin.BuildCommand.
func (a *Agent) BuildCommand(req Request) ([]string, error) {
	prompt, err := a.RenderPrompt(req)
	if err != nil {
		return nil, err
	}

	opts := a.buildExecOpts(req, prompt)
	cmd, err := a.agent.BuildCommand(opts)
	if err != nil {
		return nil, fmt.Errorf("build command: %w", err)
	}
	return cmd, nil
}

// Decompose executes TeamLeader decomposition and returns the raw model output.
func (a *Agent) Decompose(ctx context.Context, req Request) (string, error) {
	if a.runtime == nil {
		return "", errors.New("runtime plugin is required for decompose")
	}

	prompt, err := a.RenderPrompt(req)
	if err != nil {
		return "", err
	}

	opts := a.buildExecOpts(req, prompt)
	cmd, err := a.agent.BuildCommand(opts)
	if err != nil {
		return "", fmt.Errorf("build command: %w", err)
	}
	log.Printf("[TeamLeader] decompose agent=%s cmd=%v", a.agent.Name(), cmd)

	sess, err := a.runtime.Create(ctx, core.RuntimeOpts{
		WorkDir: req.WorkDir,
		Env:     copyMap(req.Env),
		Command: cmd,
	})
	if err != nil {
		return "", fmt.Errorf("create runtime session: %w", err)
	}

	// Drain stderr in the background to avoid deadlocks when the child process
	// writes a lot of progress output to stderr (common for CLIs).
	//
	// Without this, the stderr pipe buffer can fill up, causing the child process
	// to block indefinitely and the HTTP request to "hang".
	var stderrBuf bytes.Buffer
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		// Keep only a bounded amount to avoid unbounded memory growth.
		_, _ = io.CopyN(&stderrBuf, sess.Stderr, 64<<10)
		// Drain the remaining data (if any) without buffering it.
		_, _ = io.Copy(io.Discard, sess.Stderr)
	}()

	parser := a.agent.NewStreamParser(sess.Stdout)
	rawOutput, parseErr := collectOutput(parser)
	waitErr := sess.Wait()
	<-stderrDone
	if parseErr != nil {
		return "", parseErr
	}
	if waitErr != nil {
		stderrText := strings.TrimSpace(stderrBuf.String())
		if stderrText != "" {
			return "", fmt.Errorf("wait session: %w (stderr: %s)", waitErr, stderrText)
		}
		return "", fmt.Errorf("wait session: %w", waitErr)
	}
	return rawOutput, nil
}

func (a *Agent) RenderPrompt(req Request) (string, error) {
	rawConversation := strings.TrimSpace(req.Conversation)
	fileContext := renderFileContentsContext(req.FileContents)

	conversation := rawConversation
	if fileContext != "" {
		if conversation == "" {
			conversation = fileContext
		} else {
			conversation = conversation + "\n\n" + fileContext
		}
	}

	originalSummary := strings.TrimSpace(req.OriginalConversationSummary)
	if originalSummary == "" {
		switch {
		case rawConversation != "":
			originalSummary = rawConversation
		case fileContext != "":
			originalSummary = summarizeFileContents(req.FileContents)
		default:
			originalSummary = conversation
		}
	}
	if originalSummary == "" {
		return "", errors.New("conversation is required")
	}
	if conversation == "" {
		conversation = originalSummary
	}

	vars := promptVars{
		Conversation:                conversation,
		ProjectName:                 strings.TrimSpace(req.ProjectName),
		TechStack:                   strings.TrimSpace(req.TechStack),
		RepoPath:                    strings.TrimSpace(req.RepoPath),
		OriginalConversationSummary: originalSummary,
		PreviousTaskPlanJSON:        defaultJSONPlaceholder(req.PreviousTaskPlanJSON),
		AIReviewSummaryJSON:         defaultJSONPlaceholder(req.AIReviewSummaryJSON),
		HumanFeedbackJSON:           defaultJSONPlaceholder(req.HumanFeedbackJSON),
	}

	if vars.ProjectName == "" {
		vars.ProjectName = "unknown-project"
	}
	if vars.TechStack == "" {
		vars.TechStack = "unknown"
	}
	if vars.RepoPath == "" {
		vars.RepoPath = "."
	}

	var b strings.Builder
	if err := a.promptTmpl.Execute(&b, vars); err != nil {
		return "", fmt.Errorf("render TeamLeader template: %w", err)
	}
	return strings.TrimSpace(b.String()), nil
}

func (a *Agent) buildExecOpts(req Request, prompt string) core.ExecOpts {
	maxTurns := a.maxTurns
	if req.MaxTurns > 0 {
		maxTurns = req.MaxTurns
	}
	roleID := resolveRoleID(req.Role)

	return core.ExecOpts{
		Prompt:        prompt,
		WorkDir:       req.WorkDir,
		AllowedTools:  copyStrings(a.allowedTools),
		MaxTurns:      maxTurns,
		Timeout:       req.Timeout,
		Env:           copyMap(req.Env),
		AppendContext: roleContextJSON(roleID),
	}
}

func collectOutput(parser core.StreamParser) (string, error) {
	var textChunks []string
	resultChunk := ""

	for {
		evt, err := parser.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", fmt.Errorf("parse stream output: %w", err)
		}
		if evt == nil {
			continue
		}

		content := strings.TrimSpace(evt.Content)
		if content == "" {
			continue
		}
		switch evt.Type {
		case "done":
			resultChunk = content
		case "text":
			textChunks = append(textChunks, content)
		}
	}

	if resultChunk != "" {
		return resultChunk, nil
	}
	if len(textChunks) > 0 {
		return strings.Join(textChunks, "\n"), nil
	}
	return "", errors.New("agent returned empty output")
}

func readTemplateContent(explicitPath string) ([]byte, string, error) {
	candidates := make([]string, 0, 3)
	if strings.TrimSpace(explicitPath) != "" {
		candidates = append(candidates, explicitPath)
	}
	candidates = append(candidates,
		defaultTemplatePath,
		filepath.Join("..", "..", defaultTemplatePath),
	)

	seen := map[string]struct{}{}
	var errs []string
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		data, err := os.ReadFile(path)
		if err == nil {
			return data, path, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", path, err))
	}

	return nil, "", fmt.Errorf("TeamLeader template not found (%s)", strings.Join(errs, "; "))
}

func defaultJSONPlaceholder(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "{}"
	}
	return trimmed
}

type fileContentEntry struct {
	Path    string
	Content string
}

func sortedFileContentEntries(fileContents map[string]string) []fileContentEntry {
	if len(fileContents) == 0 {
		return nil
	}

	paths := make([]string, 0, len(fileContents))
	for path := range fileContents {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	seen := make(map[string]struct{}, len(paths))
	entries := make([]fileContentEntry, 0, len(paths))
	for _, originalPath := range paths {
		path := strings.TrimSpace(originalPath)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		entries = append(entries, fileContentEntry{
			Path:    path,
			Content: fileContents[originalPath],
		})
	}
	return entries
}

func renderFileContentsContext(fileContents map[string]string) string {
	entries := sortedFileContentEntries(fileContents)
	if len(entries) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("输入 5：补充文件内容（按路径聚合）\n")
	for i, entry := range entries {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("<<<FILE:")
		b.WriteString(entry.Path)
		b.WriteString(">>>\n")

		if strings.TrimSpace(entry.Content) == "" {
			b.WriteString("(empty file)\n")
		} else {
			b.WriteString(entry.Content)
			if !strings.HasSuffix(entry.Content, "\n") {
				b.WriteString("\n")
			}
		}
		b.WriteString("<<<END FILE>>>\n")
	}
	return strings.TrimSpace(b.String())
}

func summarizeFileContents(fileContents map[string]string) string {
	entries := sortedFileContentEntries(fileContents)
	if len(entries) == 0 {
		return ""
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	return fmt.Sprintf("补充了 %d 个文件上下文：%s", len(paths), strings.Join(paths, ", "))
}

func resolveRoleID(role string) string {
	trimmed := strings.TrimSpace(role)
	if trimmed == "" {
		return defaultRoleID
	}
	return trimmed
}

func resolveTeamLeaderRoleID(explicitRole, boundRole string) string {
	if trimmed := strings.TrimSpace(explicitRole); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(boundRole); trimmed != "" {
		return trimmed
	}
	return defaultTeamLeaderRoleID
}

func startTeamLeaderSession(
	ctx context.Context,
	client TeamLeaderSessionClient,
	resolver *acpclient.RoleResolver,
	explicitRole string,
	boundRole string,
	persistedSessionID string,
	cwd string,
	mcpServers []acpproto.McpServer,
) (acpproto.SessionId, string, error) {
	if client == nil {
		return "", "", errors.New("TeamLeader session client is required")
	}

	roleID := resolveTeamLeaderRoleID(explicitRole, boundRole)
	resolvedRole := acpclient.RoleProfile{}
	if resolver != nil {
		_, role, err := resolver.Resolve(roleID)
		if err != nil {
			return "", "", fmt.Errorf("resolve TeamLeader role %q: %w", roleID, err)
		}
		resolvedRole = role
	}

	trimmedCWD := strings.TrimSpace(cwd)
	metadata := map[string]any{
		"role_id": roleID,
	}
	effectiveMCPServers := append([]acpproto.McpServer(nil), mcpServers...)
	if len(effectiveMCPServers) == 0 {
		effectiveMCPServers = MCPToolsFromRoleConfig(resolvedRole)
	}
	if sessionID := strings.TrimSpace(persistedSessionID); shouldLoadPersistedTeamLeaderSession(resolvedRole.SessionPolicy, sessionID) {
		loaded, err := client.LoadSession(ctx, acpproto.LoadSessionRequest{
			SessionId:  acpproto.SessionId(sessionID),
			Cwd:        trimmedCWD,
			McpServers: effectiveMCPServers,
			Meta:       metadata,
		})
		if err == nil && strings.TrimSpace(string(loaded)) != "" {
			return loaded, roleID, nil
		}
	}

	session, err := client.NewSession(ctx, acpproto.NewSessionRequest{
		Cwd:        trimmedCWD,
		McpServers: effectiveMCPServers,
		Meta:       metadata,
	})
	if err != nil {
		return "", "", err
	}
	return session, roleID, nil
}

func shouldLoadPersistedTeamLeaderSession(policy acpclient.SessionPolicy, persistedSessionID string) bool {
	if strings.TrimSpace(persistedSessionID) == "" {
		return false
	}
	if !policy.Reuse {
		return false
	}
	if !policy.PreferLoadSession {
		return false
	}
	return true
}

func roleContextJSON(roleID string) string {
	payload, err := json.Marshal(map[string]string{
		"role_id": roleID,
	})
	if err != nil {
		return fmt.Sprintf(`{"role_id":%q}`, defaultRoleID)
	}
	return string(payload)
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func copyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func copyMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
