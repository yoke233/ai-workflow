package configruntime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	acpproto "github.com/coder/acp-go-sdk"
	"github.com/fsnotify/fsnotify"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/yoke233/ai-workflow/internal/config"
	"github.com/yoke233/ai-workflow/internal/teamleader"
	v2core "github.com/yoke233/ai-workflow/internal/v2/core"
)

var ErrInvalidConfig = errors.New("invalid config")

type Snapshot struct {
	Version              int64
	LoadedAt             time.Time
	Config               *config.Config
	Drivers              []*v2core.AgentDriver
	Profiles             []*v2core.AgentProfile
	MCPServersByID       map[string]MCPServer
	MCPBindingsByProfile map[string][]MCPProfileBinding
}

type MCPServer struct {
	ID        string
	Name      string
	Kind      string
	Transport string
	Endpoint  string
	Command   string
	Args      []string
	Env       map[string]string
	Headers   []acpproto.HttpHeader
	Enabled   bool
}

type MCPProfileBinding struct {
	ProfileID string
	ServerID  string
	Enabled   bool
	ToolMode  string
	Tools     []string
}

type ReloadStatus struct {
	ActiveVersion int64     `json:"active_version"`
	LastSuccessAt time.Time `json:"last_success_at"`
	LastError     string    `json:"last_error"`
	LastErrorAt   time.Time `json:"last_error_at"`
}

type ValidationError struct {
	Err error
}

func (e *ValidationError) Error() string {
	if e == nil || e.Err == nil {
		return ErrInvalidConfig.Error()
	}
	return e.Err.Error()
}

func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type V2RuntimeConfig struct {
	Agents  config.V2AgentsConfig  `json:"agents"`
	MCP     config.V2MCPConfig     `json:"mcp"`
	Prompts config.V2PromptsConfig `json:"prompts"`
}

type Manager struct {
	configPath  string
	secretsPath string
	mcpEnv      teamleader.MCPEnvConfig
	logger      *slog.Logger
	onReload    func(context.Context, *Snapshot) error

	nextVersion atomic.Int64
	current     atomic.Pointer[Snapshot]

	statusMu sync.RWMutex
	status   ReloadStatus

	reloadMu sync.Mutex
	watcher  *fsnotify.Watcher
}

func NewManager(configPath string, secretsPath string, mcpEnv teamleader.MCPEnvConfig, logger *slog.Logger, onReload func(context.Context, *Snapshot) error) (*Manager, error) {
	if logger == nil {
		logger = slog.Default()
	}
	m := &Manager{
		configPath:  configPath,
		secretsPath: secretsPath,
		mcpEnv:      mcpEnv,
		logger:      logger,
		onReload:    onReload,
	}
	if _, err := m.Reload(context.Background(), "startup"); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) Current() *Snapshot {
	return m.current.Load()
}

func (m *Manager) Status() ReloadStatus {
	m.statusMu.RLock()
	defer m.statusMu.RUnlock()
	return m.status
}

func (m *Manager) ReadRaw() ([]byte, error) {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("read config runtime raw: %w", err)
	}
	return data, nil
}

func (m *Manager) ReadRawString() (string, error) {
	data, err := m.ReadRaw()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (m *Manager) GetV2Runtime() V2RuntimeConfig {
	snap := m.Current()
	if snap == nil || snap.Config == nil {
		return V2RuntimeConfig{}
	}
	return V2RuntimeConfig{
		Agents:  snap.Config.V2.Agents,
		MCP:     snap.Config.V2.MCP,
		Prompts: snap.Config.V2.Prompts,
	}
}

func (m *Manager) CurrentV2Config() (config.V2AgentsConfig, config.V2MCPConfig, bool) {
	current := m.GetV2Runtime()
	snap := m.Current()
	return current.Agents, current.MCP, snap != nil && snap.Config != nil
}

func (m *Manager) Reload(ctx context.Context, reason string) (*Snapshot, error) {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	return m.reloadLocked(ctx, reason)
}

func (m *Manager) WriteRaw(ctx context.Context, raw string) (*Snapshot, error) {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()

	content := normalizeRaw(raw)
	if len(bytes.TrimSpace(content)) == 0 {
		err := &ValidationError{Err: errors.New("config.toml content is empty")}
		m.setError(err)
		return nil, err
	}
	if err := m.validateRaw(content); err != nil {
		m.setError(err)
		return nil, err
	}

	previous, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("read current config before write: %w", err)
	}
	if err := writeFileKeepingMode(m.configPath, content); err != nil {
		return nil, fmt.Errorf("write config runtime raw: %w", err)
	}

	snap, err := m.reloadLocked(ctx, "api")
	if err == nil {
		return snap, nil
	}

	if restoreErr := writeFileKeepingMode(m.configPath, previous); restoreErr != nil {
		return nil, fmt.Errorf("reload config runtime failed: %w (rollback write failed: %v)", err, restoreErr)
	}
	if _, rollbackErr := m.reloadLocked(context.Background(), "rollback"); rollbackErr != nil {
		return nil, fmt.Errorf("reload config runtime failed: %w (rollback reload failed: %v)", err, rollbackErr)
	}
	return nil, err
}

func (m *Manager) UpdateV2Runtime(ctx context.Context, next V2RuntimeConfig) (*Snapshot, error) {
	layer, err := m.readLayer()
	if err != nil {
		return nil, err
	}
	if layer.V2 == nil {
		layer.V2 = &config.V2Layer{}
	}
	layer.V2.Agents = &config.V2AgentsLayerCfg{
		Drivers:  cloneV2Drivers(next.Agents.Drivers),
		Profiles: cloneV2Profiles(next.Agents.Profiles),
	}
	layer.V2.MCP = &config.V2MCPLayer{
		Servers:         cloneV2MCPServers(next.MCP.Servers),
		ProfileBindings: cloneV2MCPBindings(next.MCP.ProfileBindings),
	}
	layer.V2.Prompts = &config.V2PromptsLayer{
		ReworkFollowup:        stringPtr(next.Prompts.ReworkFollowup),
		ContinueFollowup:      stringPtr(next.Prompts.ContinueFollowup),
		PRImplementObjective:  stringPtr(next.Prompts.PRImplementObjective),
		PRGateObjective:       stringPtr(next.Prompts.PRGateObjective),
		PRMergeReworkFeedback: stringPtr(next.Prompts.PRMergeReworkFeedback),
		PRProviders: &config.V2PRPromptProvidersLayer{
			GitHub: buildPRProviderPromptLayer(next.Prompts.PRProviders.GitHub),
			CodeUp: buildPRProviderPromptLayer(next.Prompts.PRProviders.CodeUp),
			GitLab: buildPRProviderPromptLayer(next.Prompts.PRProviders.GitLab),
		},
	}

	raw, err := toml.Marshal(layer)
	if err != nil {
		return nil, fmt.Errorf("marshal v2 runtime config: %w", err)
	}
	return m.WriteRaw(ctx, string(raw))
}

func buildPRProviderPromptLayer(in config.V2PRProviderPromptConfig) *config.V2PRProviderPromptLayer {
	return &config.V2PRProviderPromptLayer{
		ImplementObjective:  stringPtr(in.ImplementObjective),
		GateObjective:       stringPtr(in.GateObjective),
		MergeReworkFeedback: stringPtr(in.MergeReworkFeedback),
		MergeStates: &config.V2PRMergeStatePromptLayer{
			Default:  stringPtr(in.MergeStates.Default),
			Dirty:    stringPtr(in.MergeStates.Dirty),
			Blocked:  stringPtr(in.MergeStates.Blocked),
			Behind:   stringPtr(in.MergeStates.Behind),
			Unstable: stringPtr(in.MergeStates.Unstable),
			Draft:    stringPtr(in.MergeStates.Draft),
		},
	}
}

func (m *Manager) UpdateV2Config(ctx context.Context, agents config.V2AgentsConfig, mcp config.V2MCPConfig) (*Snapshot, error) {
	current := m.GetV2Runtime()
	current.Agents = agents
	current.MCP = mcp
	return m.UpdateV2Runtime(ctx, current)
}

func (m *Manager) Start(ctx context.Context) error {
	dir := filepath.Dir(m.configPath)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create config watcher: %w", err)
	}
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return fmt.Errorf("watch config dir %s: %w", dir, err)
	}
	m.watcher = watcher

	go m.watchLoop(ctx, dir)
	return nil
}

func (m *Manager) Close() error {
	if m.watcher != nil {
		return m.watcher.Close()
	}
	return nil
}

func (m *Manager) ResolveMCPServers(profileID string, agentSupportsSSE bool) []acpproto.McpServer {
	snap := m.Current()
	if snap == nil {
		return nil
	}
	bindings := snap.MCPBindingsByProfile[strings.TrimSpace(profileID)]
	if len(bindings) == 0 {
		return nil
	}

	var out []acpproto.McpServer
	for _, binding := range bindings {
		if !binding.Enabled {
			continue
		}
		server, ok := snap.MCPServersByID[binding.ServerID]
		if !ok || !server.Enabled {
			continue
		}
		if strings.EqualFold(server.Kind, "internal") {
			out = append(out, buildInternalServer(server, m.mcpEnv, agentSupportsSSE)...)
			continue
		}
		switch strings.ToLower(strings.TrimSpace(server.Transport)) {
		case "sse":
			out = append(out, acpproto.McpServer{
				Sse: &acpproto.McpServerSseInline{
					Name:    server.Name,
					Type:    "sse",
					Url:     server.Endpoint,
					Headers: server.Headers,
				},
			})
		case "stdio":
			env := make([]acpproto.EnvVariable, 0, len(server.Env))
			for k, v := range server.Env {
				env = append(env, acpproto.EnvVariable{Name: k, Value: v})
			}
			out = append(out, acpproto.McpServer{
				Stdio: &acpproto.McpServerStdio{
					Name:    server.Name,
					Command: server.Command,
					Args:    append([]string(nil), server.Args...),
					Env:     env,
				},
			})
		}
	}
	return out
}

func (m *Manager) reloadLocked(ctx context.Context, reason string) (*Snapshot, error) {
	snap, err := m.buildSnapshotFromPaths(m.configPath, m.secretsPath)
	if err != nil {
		m.setError(err)
		return nil, err
	}
	if m.onReload != nil {
		if err := m.onReload(ctx, snap); err != nil {
			m.setError(err)
			return nil, err
		}
	}
	m.current.Store(snap)
	m.setSuccess(snap)
	if m.logger != nil {
		m.logger.Info("config runtime reloaded", "version", snap.Version, "reason", reason)
	}
	return snap, nil
}

func (m *Manager) buildSnapshotFromPaths(configPath string, secretsPath string) (*Snapshot, error) {
	cfg, err := config.LoadGlobal(configPath, secretsPath)
	if err != nil {
		return nil, fmt.Errorf("load config runtime: %w", err)
	}
	secrets, err := config.LoadSecrets(secretsPath)
	if err != nil {
		return nil, fmt.Errorf("load secrets runtime: %w", err)
	}
	drivers, profiles := BuildV2Agents(cfg)
	servers, err := buildMCPServers(cfg, secrets, profiles)
	if err != nil {
		return nil, err
	}
	bindings := buildBindings(cfg, profiles)

	return &Snapshot{
		Version:              m.nextVersion.Add(1),
		LoadedAt:             time.Now().UTC(),
		Config:               cfg,
		Drivers:              drivers,
		Profiles:             profiles,
		MCPServersByID:       servers,
		MCPBindingsByProfile: bindings,
	}, nil
}

func (m *Manager) validateRaw(raw []byte) error {
	layer := &config.ConfigLayer{}
	decoder := toml.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(layer); err != nil {
		return &ValidationError{Err: fmt.Errorf("parse config.toml: %w", err)}
	}

	tmp, err := os.CreateTemp(filepath.Dir(m.configPath), "config-runtime-*.toml")
	if err != nil {
		return fmt.Errorf("create config runtime validation file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write config runtime validation file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close config runtime validation file: %w", err)
	}
	if _, err := config.LoadGlobal(tmpPath, m.secretsPath); err != nil {
		return &ValidationError{Err: fmt.Errorf("%w: %v", ErrInvalidConfig, err)}
	}
	return nil
}

func (m *Manager) setSuccess(snap *Snapshot) {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	m.status.ActiveVersion = snap.Version
	m.status.LastSuccessAt = snap.LoadedAt
	m.status.LastError = ""
}

func (m *Manager) setError(err error) {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	m.status.LastError = err.Error()
	m.status.LastErrorAt = time.Now().UTC()
	if m.logger != nil {
		m.logger.Warn("config runtime reload failed", "error", err)
	}
}

func (m *Manager) readLayer() (*config.ConfigLayer, error) {
	raw, err := m.ReadRaw()
	if err != nil {
		return nil, err
	}
	layer := &config.ConfigLayer{}
	decoder := toml.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(layer); err != nil {
		return nil, fmt.Errorf("decode config layer: %w", err)
	}
	return layer, nil
}

func cloneV2Drivers(items []config.V2DriverConfig) *[]config.V2DriverConfig {
	if items == nil {
		return nil
	}
	out := append([]config.V2DriverConfig(nil), items...)
	return &out
}

func cloneV2Profiles(items []config.V2ProfileConfig) *[]config.V2ProfileConfig {
	if items == nil {
		return nil
	}
	out := append([]config.V2ProfileConfig(nil), items...)
	return &out
}

func cloneV2MCPServers(items []config.V2MCPServerConfig) *[]config.V2MCPServerConfig {
	if items == nil {
		return nil
	}
	out := append([]config.V2MCPServerConfig(nil), items...)
	return &out
}

func cloneV2MCPBindings(items []config.V2MCPProfileBindingConfig) *[]config.V2MCPProfileBindingConfig {
	if items == nil {
		return nil
	}
	out := append([]config.V2MCPProfileBindingConfig(nil), items...)
	return &out
}

func stringPtr(v string) *string {
	return &v
}

func StructToTomlMap(v any) (map[string]any, error) {
	data, err := toml.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal toml value: %w", err)
	}
	out := map[string]any{}
	if err := toml.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("unmarshal toml map: %w", err)
	}
	return out, nil
}

func TomlMapToStruct(raw map[string]any, out any) error {
	data, err := toml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal toml map: %w", err)
	}
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode toml map: %w", err)
	}
	return nil
}

func normalizeRaw(raw string) []byte {
	content := []byte(strings.ReplaceAll(raw, "\r\n", "\n"))
	if len(content) > 0 && content[len(content)-1] != '\n' {
		content = append(content, '\n')
	}
	return content
}

func writeFileKeepingMode(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(path, data, mode)
}

func (m *Manager) watchLoop(ctx context.Context, dir string) {
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}

	schedule := func() {
		timer.Reset(500 * time.Millisecond)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-m.watcher.Events:
			if !ok {
				return
			}
			base := filepath.Base(evt.Name)
			if base != filepath.Base(m.configPath) && base != filepath.Base(m.secretsPath) {
				continue
			}
			if evt.Has(fsnotify.Remove) || evt.Has(fsnotify.Rename) {
				_ = m.watcher.Remove(dir)
				_ = m.watcher.Add(dir)
			}
			schedule()
		case err, ok := <-m.watcher.Errors:
			if ok && m.logger != nil {
				m.logger.Warn("config watcher error", "error", err)
			}
		case <-timer.C:
			if _, err := m.Reload(context.Background(), "fsnotify"); err != nil && m.logger != nil {
				m.logger.Warn("config watcher reload skipped", "error", err)
			}
		}
	}
}

const defaultInternalMCPServerID = "ai-workflow-query"

func buildMCPServers(cfg *config.Config, secrets *config.Secrets, profiles []*v2core.AgentProfile) (map[string]MCPServer, error) {
	out := make(map[string]MCPServer, len(cfg.V2.MCP.Servers))
	for _, server := range cfg.V2.MCP.Servers {
		headers := []acpproto.HttpHeader{}
		if ref := strings.TrimSpace(server.AuthSecretRef); ref != "" {
			token, err := resolveSecretRef(secrets, ref)
			if err != nil {
				return nil, fmt.Errorf("resolve auth_secret_ref for server %q: %w", server.ID, err)
			}
			if token != "" {
				headers = append(headers, acpproto.HttpHeader{Name: "Authorization", Value: "Bearer " + token})
			}
		}
		name := strings.TrimSpace(server.Name)
		if name == "" {
			name = strings.TrimSpace(server.ID)
		}
		out[strings.TrimSpace(server.ID)] = MCPServer{
			ID:        strings.TrimSpace(server.ID),
			Name:      name,
			Kind:      strings.TrimSpace(server.Kind),
			Transport: strings.TrimSpace(server.Transport),
			Endpoint:  strings.TrimSpace(server.Endpoint),
			Command:   strings.TrimSpace(server.Command),
			Args:      append([]string(nil), server.Args...),
			Env:       cloneStringMap(server.Env),
			Headers:   headers,
			Enabled:   server.Enabled,
		}
	}
	if len(out) == 0 && hasLegacyProfileMCP(profiles) {
		out[defaultInternalMCPServerID] = MCPServer{
			ID:        defaultInternalMCPServerID,
			Name:      defaultInternalMCPServerID,
			Kind:      "internal",
			Transport: "sse",
			Enabled:   true,
		}
	}
	return out, nil
}

func buildBindings(cfg *config.Config, profiles []*v2core.AgentProfile) map[string][]MCPProfileBinding {
	out := make(map[string][]MCPProfileBinding, len(cfg.V2.MCP.ProfileBindings))
	if len(cfg.V2.MCP.ProfileBindings) == 0 {
		for _, profile := range profiles {
			if profile == nil || !profile.MCP.Enabled {
				continue
			}
			mode := "all"
			if len(profile.MCP.Tools) > 0 {
				mode = "allow_list"
			}
			out[profile.ID] = append(out[profile.ID], MCPProfileBinding{
				ProfileID: profile.ID,
				ServerID:  defaultInternalMCPServerID,
				Enabled:   true,
				ToolMode:  mode,
				Tools:     append([]string(nil), profile.MCP.Tools...),
			})
		}
		return out
	}
	for _, binding := range cfg.V2.MCP.ProfileBindings {
		item := MCPProfileBinding{
			ProfileID: strings.TrimSpace(binding.Profile),
			ServerID:  strings.TrimSpace(binding.Server),
			Enabled:   binding.Enabled,
			ToolMode:  strings.TrimSpace(binding.ToolMode),
			Tools:     append([]string(nil), binding.Tools...),
		}
		out[item.ProfileID] = append(out[item.ProfileID], item)
	}
	return out
}

func hasLegacyProfileMCP(profiles []*v2core.AgentProfile) bool {
	for _, profile := range profiles {
		if profile != nil && profile.MCP.Enabled {
			return true
		}
	}
	return false
}

func resolveSecretRef(secrets *config.Secrets, ref string) (string, error) {
	if secrets == nil {
		return "", fmt.Errorf("secrets unavailable")
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", nil
	}
	if !strings.HasPrefix(ref, "tokens.") {
		return "", fmt.Errorf("unsupported secret ref %q", ref)
	}
	name := strings.TrimPrefix(ref, "tokens.")
	entry, ok := secrets.Tokens[name]
	if !ok {
		return "", fmt.Errorf("unknown token %q", name)
	}
	return strings.TrimSpace(entry.Token), nil
}

func buildInternalServer(server MCPServer, env teamleader.MCPEnvConfig, agentSupportsSSE bool) []acpproto.McpServer {
	name := strings.TrimSpace(server.Name)
	if name == "" {
		name = strings.TrimSpace(server.ID)
	}
	if addr := strings.TrimSpace(env.ServerAddr); addr != "" && agentSupportsSSE {
		url := strings.TrimRight(addr, "/") + "/api/v1/mcp"
		headers := []acpproto.HttpHeader{}
		if tok := strings.TrimSpace(env.AuthToken); tok != "" {
			headers = append(headers, acpproto.HttpHeader{Name: "Authorization", Value: "Bearer " + tok})
		}
		return []acpproto.McpServer{{
			Sse: &acpproto.McpServerSseInline{
				Name:    name,
				Type:    "sse",
				Url:     url,
				Headers: headers,
			},
		}}
	}
	if strings.TrimSpace(env.DBPath) == "" {
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return nil
	}
	stdioEnv := []acpproto.EnvVariable{{Name: "AI_WORKFLOW_DB_PATH", Value: env.DBPath}}
	if env.DevMode {
		stdioEnv = append(stdioEnv,
			acpproto.EnvVariable{Name: "AI_WORKFLOW_DEV_MODE", Value: "true"},
			acpproto.EnvVariable{Name: "AI_WORKFLOW_SOURCE_ROOT", Value: env.SourceRoot},
			acpproto.EnvVariable{Name: "AI_WORKFLOW_SERVER_ADDR", Value: env.ServerAddr},
		)
	}
	return []acpproto.McpServer{{
		Stdio: &acpproto.McpServerStdio{
			Name:    name,
			Command: self,
			Args:    []string{"mcp-serve"},
			Env:     stdioEnv,
		},
	}}
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
