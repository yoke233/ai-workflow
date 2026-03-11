package web

import (
	"context"
	"errors"
	"strings"

	acpproto "github.com/coder/acp-go-sdk"
)

// ChatAssistantRequest contains one user turn for model completion.
type ChatAssistantRequest struct {
	Message        string
	Role           string
	WorkDir        string
	AgentSessionID string
	ProjectID      string
	ChatSessionID  string
	AgentOverride  string
}

// ChatAssistantResponse contains assistant content and provider session identity.
type ChatAssistantResponse struct {
	Reply          string
	AgentSessionID string
}

// ChatAssistant provides multi-turn chat completion for /chat APIs.
type ChatAssistant interface {
	Reply(ctx context.Context, req ChatAssistantRequest) (ChatAssistantResponse, error)
	GetSessionCommands(chatSessionID string) ([]acpproto.AvailableCommand, error)
	GetSessionConfigOptions(chatSessionID string) ([]acpproto.SessionConfigOptionSelect, error)
	SetSessionConfigOption(ctx context.Context, chatSessionID string, configID string, value string) ([]acpproto.SessionConfigOptionSelect, error)
}

var errChatSessionNotFound = errors.New("chat session not found")

type ChatAssistantCanceler interface {
	CancelChat(chatSessionID string) error
}

// ChatSessionStatusProvider reports liveness of pooled ACP chat sessions.
type ChatSessionStatusProvider interface {
	IsChatSessionAlive(chatSessionID string) bool
	IsChatSessionRunning(chatSessionID string) bool
}

// ClaudeChatAssistant starts ACP sessions through role-driven resolver and returns one reply turn.
type ClaudeChatAssistant struct {
	assistant *ACPChatAssistant
}

// NewClaudeChatAssistant creates a ChatAssistant backed by ACP client launch.
func NewClaudeChatAssistant(binary string) ChatAssistant {
	trimmedBinary := strings.TrimSpace(binary)
	if trimmedBinary == "" {
		trimmedBinary = "claude"
	}
	return newClaudeChatAssistantForTest(trimmedBinary, ACPChatAssistantDeps{})
}

func newClaudeChatAssistantForTest(binary string, deps ACPChatAssistantDeps) *ClaudeChatAssistant {
	trimmedBinary := strings.TrimSpace(binary)
	if trimmedBinary == "" {
		trimmedBinary = "claude"
	}
	if deps.RoleResolver == nil {
		deps.RoleResolver = newLegacyProviderRoleResolver("claude", trimmedBinary, nil, nil)
	}
	return &ClaudeChatAssistant{
		assistant: newACPChatAssistant(deps),
	}
}

func (a *ClaudeChatAssistant) Reply(ctx context.Context, req ChatAssistantRequest) (ChatAssistantResponse, error) {
	if a == nil || a.assistant == nil {
		return ChatAssistantResponse{}, errors.New("chat assistant is nil")
	}
	return a.assistant.Reply(ctx, req)
}

func (a *ClaudeChatAssistant) GetSessionCommands(chatSessionID string) ([]acpproto.AvailableCommand, error) {
	if a == nil || a.assistant == nil {
		return nil, errors.New("chat assistant is nil")
	}
	return a.assistant.GetSessionCommands(chatSessionID)
}

func (a *ClaudeChatAssistant) GetSessionConfigOptions(chatSessionID string) ([]acpproto.SessionConfigOptionSelect, error) {
	if a == nil || a.assistant == nil {
		return nil, errors.New("chat assistant is nil")
	}
	return a.assistant.GetSessionConfigOptions(chatSessionID)
}

func (a *ClaudeChatAssistant) SetSessionConfigOption(ctx context.Context, chatSessionID string, configID string, value string) ([]acpproto.SessionConfigOptionSelect, error) {
	if a == nil || a.assistant == nil {
		return nil, errors.New("chat assistant is nil")
	}
	return a.assistant.SetSessionConfigOption(ctx, chatSessionID, configID, value)
}
