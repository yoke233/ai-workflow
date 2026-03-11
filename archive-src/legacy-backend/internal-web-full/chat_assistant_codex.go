package web

import (
	"context"
	"errors"
	"strings"

	acpproto "github.com/coder/acp-go-sdk"
)

// CodexChatAssistant starts ACP sessions through role-driven resolver and returns one reply turn.
type CodexChatAssistant struct {
	assistant *ACPChatAssistant
}

// NewCodexChatAssistant creates a ChatAssistant backed by ACP client launch.
func NewCodexChatAssistant(binary, model, reasoning string) ChatAssistant {
	trimmedBinary := strings.TrimSpace(binary)
	if trimmedBinary == "" {
		trimmedBinary = "codex"
	}
	return newCodexChatAssistantForTest(trimmedBinary, model, reasoning, ACPChatAssistantDeps{})
}

func newCodexChatAssistantForTest(binary, model, reasoning string, deps ACPChatAssistantDeps) *CodexChatAssistant {
	trimmedBinary := strings.TrimSpace(binary)
	if trimmedBinary == "" {
		trimmedBinary = "codex"
	}
	launchEnv := map[string]string{}
	if trimmedModel := strings.TrimSpace(model); trimmedModel != "" {
		launchEnv["AI_WORKFLOW_CODEX_MODEL"] = trimmedModel
	}
	if trimmedReasoning := strings.TrimSpace(reasoning); trimmedReasoning != "" {
		launchEnv["AI_WORKFLOW_CODEX_REASONING"] = trimmedReasoning
	}
	if len(launchEnv) == 0 {
		launchEnv = nil
	}
	if deps.RoleResolver == nil {
		deps.RoleResolver = newLegacyProviderRoleResolver("codex", trimmedBinary, nil, launchEnv)
	}
	return &CodexChatAssistant{
		assistant: newACPChatAssistant(deps),
	}
}

func (a *CodexChatAssistant) Reply(ctx context.Context, req ChatAssistantRequest) (ChatAssistantResponse, error) {
	if a == nil || a.assistant == nil {
		return ChatAssistantResponse{}, errors.New("chat assistant is nil")
	}
	return a.assistant.Reply(ctx, req)
}

func (a *CodexChatAssistant) GetSessionCommands(chatSessionID string) ([]acpproto.AvailableCommand, error) {
	if a == nil || a.assistant == nil {
		return nil, errors.New("chat assistant is nil")
	}
	return a.assistant.GetSessionCommands(chatSessionID)
}

func (a *CodexChatAssistant) GetSessionConfigOptions(chatSessionID string) ([]acpproto.SessionConfigOptionSelect, error) {
	if a == nil || a.assistant == nil {
		return nil, errors.New("chat assistant is nil")
	}
	return a.assistant.GetSessionConfigOptions(chatSessionID)
}

func (a *CodexChatAssistant) SetSessionConfigOption(ctx context.Context, chatSessionID string, configID string, value string) ([]acpproto.SessionConfigOptionSelect, error) {
	if a == nil || a.assistant == nil {
		return nil, errors.New("chat assistant is nil")
	}
	return a.assistant.SetSessionConfigOption(ctx, chatSessionID, configID, value)
}
