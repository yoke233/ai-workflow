package acp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	acpproto "github.com/coder/acp-go-sdk"
	"github.com/yoke233/ai-workflow/internal/adapters/agent/acpclient"
	eventbridge "github.com/yoke233/ai-workflow/internal/adapters/events/bridge"
	v2sandbox "github.com/yoke233/ai-workflow/internal/adapters/sandbox"
	chatapp "github.com/yoke233/ai-workflow/internal/application/chat"
	"github.com/yoke233/ai-workflow/internal/core"
)

const (
	defaultLeadProfileID  = "lead"
	defaultLeadTimeout    = 120 * time.Second
	defaultSessionIdleTTL = 30 * time.Minute
)

type LeadAgentConfig struct {
	Registry  core.AgentRegistry
	Bus       core.EventBus
	ProfileID string
	Timeout   time.Duration
	IdleTTL   time.Duration
	Sandbox   v2sandbox.Sandbox
}

type LeadAgent struct {
	cfg LeadAgentConfig

	mu       sync.Mutex
	sessions map[string]*leadSession

	activeMu   sync.Mutex
	activeRuns map[string]context.CancelFunc
}

type leadSession struct {
	client    ChatACPClient
	sessionID acpproto.SessionId
	bridge    *eventbridge.EventBridge

	mu        sync.Mutex
	idleTimer *time.Timer
	closed    bool
}

type ChatACPClient interface {
	NewSession(ctx context.Context, req acpproto.NewSessionRequest) (acpproto.SessionId, error)
	Prompt(ctx context.Context, req acpproto.PromptRequest) (*acpclient.PromptResult, error)
	Cancel(ctx context.Context, req acpproto.CancelNotification) error
	Close(ctx context.Context) error
}

func NewLeadAgent(cfg LeadAgentConfig) *LeadAgent {
	if cfg.ProfileID == "" {
		cfg.ProfileID = defaultLeadProfileID
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultLeadTimeout
	}
	if cfg.IdleTTL <= 0 {
		cfg.IdleTTL = defaultSessionIdleTTL
	}
	return &LeadAgent{
		cfg:        cfg,
		sessions:   make(map[string]*leadSession),
		activeRuns: make(map[string]context.CancelFunc),
	}
}

func (l *LeadAgent) Chat(ctx context.Context, req chatapp.Request) (*chatapp.Response, error) {
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return nil, errors.New("message is required")
	}

	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = fmt.Sprintf("chat-%d", time.Now().UnixNano())
	}

	sess, err := l.getOrCreateSession(ctx, sessionID, strings.TrimSpace(req.WorkDir))
	if err != nil {
		return nil, err
	}
	sess.stopIdleTimer()

	promptCtx, promptCancel := context.WithTimeout(ctx, l.cfg.Timeout)
	defer promptCancel()

	l.activeMu.Lock()
	l.activeRuns[sessionID] = promptCancel
	l.activeMu.Unlock()
	defer func() {
		l.activeMu.Lock()
		delete(l.activeRuns, sessionID)
		l.activeMu.Unlock()
	}()

	result, err := sess.client.Prompt(promptCtx, acpproto.PromptRequest{
		SessionId: sess.sessionID,
		Prompt: []acpproto.ContentBlock{
			{Text: &acpproto.ContentBlockText{Text: message}},
		},
	})

	sess.bridge.FlushPending(ctx)

	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			l.resetSessionIdle(sessionID, sess)
		} else {
			l.removeSession(sessionID)
		}
		return nil, fmt.Errorf("prompt failed: %w", err)
	}
	if result == nil {
		l.removeSession(sessionID)
		return nil, errors.New("empty result from agent")
	}

	reply := strings.TrimSpace(result.Text)
	if reply == "" {
		l.removeSession(sessionID)
		return nil, errors.New("empty reply from agent")
	}

	sess.bridge.PublishData(ctx, map[string]any{
		"type":    "done",
		"content": reply,
	})

	l.resetSessionIdle(sessionID, sess)

	return &chatapp.Response{
		SessionID: sessionID,
		Reply:     reply,
	}, nil
}

func (l *LeadAgent) CancelChat(sessionID string) error {
	id := strings.TrimSpace(sessionID)
	if id == "" {
		return errors.New("session_id is required")
	}

	l.activeMu.Lock()
	cancel, ok := l.activeRuns[id]
	l.activeMu.Unlock()
	if !ok {
		return errors.New("session is not running")
	}
	cancel()

	l.mu.Lock()
	sess := l.sessions[id]
	l.mu.Unlock()
	if sess != nil {
		cancelCtx, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		_ = sess.client.Cancel(cancelCtx, acpproto.CancelNotification{SessionId: sess.sessionID})
	}
	return nil
}

func (l *LeadAgent) CloseSession(sessionID string) {
	l.removeSession(strings.TrimSpace(sessionID))
}

func (l *LeadAgent) Shutdown() {
	l.mu.Lock()
	sessions := make([]*leadSession, 0, len(l.sessions))
	for id, sess := range l.sessions {
		sessions = append(sessions, sess)
		delete(l.sessions, id)
	}
	l.mu.Unlock()

	for _, sess := range sessions {
		sess.close()
	}
}

func (l *LeadAgent) IsSessionAlive(sessionID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	sess, ok := l.sessions[strings.TrimSpace(sessionID)]
	return ok && !sess.isClosed()
}

func (l *LeadAgent) IsSessionRunning(sessionID string) bool {
	l.activeMu.Lock()
	defer l.activeMu.Unlock()
	_, ok := l.activeRuns[strings.TrimSpace(sessionID)]
	return ok
}

func (l *LeadAgent) getOrCreateSession(ctx context.Context, sessionID, workDir string) (*leadSession, error) {
	l.mu.Lock()
	if sess, ok := l.sessions[sessionID]; ok && !sess.isClosed() {
		l.mu.Unlock()
		return sess, nil
	}
	l.mu.Unlock()

	if l.cfg.Registry == nil {
		return nil, errors.New("agent registry is not configured")
	}
	profile, driver, err := l.cfg.Registry.ResolveByID(ctx, l.cfg.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("resolve lead profile %q: %w", l.cfg.ProfileID, err)
	}

	launchCfg := acpclient.LaunchConfig{
		Command: driver.LaunchCommand,
		Args:    driver.LaunchArgs,
		WorkDir: workDir,
		Env:     cloneEnv(driver.Env),
	}

	bridge := eventbridge.New(l.cfg.Bus, core.EventChatOutput, eventbridge.Scope{
		SessionID: sessionID,
	})

	sb := l.cfg.Sandbox
	if sb == nil {
		sb = v2sandbox.NoopSandbox{}
	}
	sandboxedLaunch, sbErr := sb.Prepare(ctx, v2sandbox.PrepareInput{
		Profile: profile,
		Driver:  driver,
		Launch:  launchCfg,
		Scope:   "chat-" + sessionID,
	})
	if sbErr != nil {
		return nil, fmt.Errorf("prepare sandbox: %w", sbErr)
	}
	launchCfg = sandboxedLaunch

	client, err := acpclient.New(launchCfg, &acpclient.NopHandler{}, acpclient.WithEventHandler(bridge))
	if err != nil {
		return nil, fmt.Errorf("launch lead agent: %w", err)
	}

	caps := profile.EffectiveCapabilities()
	acpCaps := acpclient.ClientCapabilities{
		FSRead:   caps.FSRead,
		FSWrite:  caps.FSWrite,
		Terminal: caps.Terminal,
	}

	initCtx, initCancel := context.WithTimeout(ctx, 30*time.Second)
	defer initCancel()

	if err := client.Initialize(initCtx, acpCaps); err != nil {
		_ = client.Close(context.Background())
		return nil, fmt.Errorf("initialize lead agent: %w", err)
	}

	acpSessionID, err := client.NewSession(initCtx, acpproto.NewSessionRequest{
		Cwd:        workDir,
		McpServers: []acpproto.McpServer{},
	})
	if err != nil {
		_ = client.Close(context.Background())
		return nil, fmt.Errorf("create lead session: %w", err)
	}

	sess := &leadSession{
		client:    client,
		sessionID: acpSessionID,
		bridge:    bridge,
	}

	l.mu.Lock()
	if old, ok := l.sessions[sessionID]; ok {
		go old.close()
	}
	l.sessions[sessionID] = sess
	l.mu.Unlock()

	slog.Info("runtime lead session created", "session_id", sessionID, "profile", profile.ID, "driver", driver.ID)
	return sess, nil
}

func (l *LeadAgent) removeSession(sessionID string) {
	if sessionID == "" {
		return
	}
	l.mu.Lock()
	sess, ok := l.sessions[sessionID]
	if ok {
		delete(l.sessions, sessionID)
	}
	l.mu.Unlock()
	if sess != nil {
		sess.close()
	}
}

func (l *LeadAgent) resetSessionIdle(sessionID string, sess *leadSession) {
	sess.resetIdleTimer(l.cfg.IdleTTL, func() {
		l.removeSession(sessionID)
	})
}

func (s *leadSession) stopIdleTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.idleTimer != nil {
		s.idleTimer.Stop()
		s.idleTimer = nil
	}
}

func (s *leadSession) resetIdleTimer(d time.Duration, onExpire func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.idleTimer != nil {
		s.idleTimer.Stop()
	}
	s.idleTimer = time.AfterFunc(d, onExpire)
}

func (s *leadSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *leadSession) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	if s.idleTimer != nil {
		s.idleTimer.Stop()
		s.idleTimer = nil
	}
	client := s.client
	s.mu.Unlock()

	if client != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = client.Close(closeCtx)
	}
}

func cloneEnv(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
