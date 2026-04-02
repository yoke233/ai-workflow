package api

import (
	"context"

	requirementapp "github.com/yoke233/zhanggui/internal/application/requirementapp"
	"github.com/yoke233/zhanggui/internal/core"
)

type requirementThreadRouteResult struct {
	Agents       []string
	Message      *core.ThreadMessage
	InviteErrors map[string]string
}

func (h *Handler) routeRequirementThreadKickoff(ctx context.Context, threadID int64, ownerID string, input requirementapp.CreateThreadInput, agentIDs []string, source string) (*requirementThreadRouteResult, error) {
	successfulAgents := append([]string(nil), agentIDs...)
	inviteErrors := map[string]string{}
	if h.threadPool != nil {
		successfulAgents = successfulAgents[:0]
		for _, profileID := range agentIDs {
			if _, inviteErr := h.threadPool.InviteAgent(ctx, threadID, profileID); inviteErr != nil {
				inviteErrors[profileID] = inviteErr.Error()
				continue
			}
			successfulAgents = append(successfulAgents, profileID)
		}
		if len(inviteErrors) == 0 {
			inviteErrors = nil
		}
	}

	targetAgentIDs := successfulAgents
	if h.threadPool == nil {
		targetAgentIDs = nil
	}

	_, message, err := h.createThreadMessageAndRoute(ctx, threadMessageInput{
		ThreadID:       threadID,
		SenderID:       ownerID,
		Role:           "human",
		Content:        buildRequirementInitialMessage(input),
		Metadata:       map[string]any{"source": source, "broadcast": len(successfulAgents) == 0},
		TargetAgentIDs: targetAgentIDs,
	})
	if err != nil {
		return nil, err
	}

	return &requirementThreadRouteResult{
		Agents:       successfulAgents,
		Message:      message,
		InviteErrors: inviteErrors,
	}, nil
}
