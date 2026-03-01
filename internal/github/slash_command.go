package github

import (
	"fmt"
	"strings"

	"github.com/user/ai-workflow/internal/core"
)

type SlashCommandType string

const (
	SlashCommandRun     SlashCommandType = "run"
	SlashCommandApprove SlashCommandType = "approve"
	SlashCommandReject  SlashCommandType = "reject"
	SlashCommandStatus  SlashCommandType = "status"
	SlashCommandAbort   SlashCommandType = "abort"
)

type SlashCommand struct {
	Type     SlashCommandType
	Stage    core.StageID
	Reason   string
	Template string
	Raw      string
}

type SlashACLConfig struct {
	AssociationPermissions map[string][]SlashCommandType
	AuthorizedUsernames    []string
}

var defaultAssociationPermissions = map[string][]SlashCommandType{
	"OWNER":        {SlashCommandRun, SlashCommandApprove, SlashCommandReject, SlashCommandStatus, SlashCommandAbort},
	"MEMBER":       {SlashCommandRun, SlashCommandApprove, SlashCommandReject, SlashCommandStatus, SlashCommandAbort},
	"COLLABORATOR": {SlashCommandRun, SlashCommandApprove, SlashCommandReject, SlashCommandStatus, SlashCommandAbort},
	"CONTRIBUTOR":  {SlashCommandRun, SlashCommandStatus},
	"NONE":         {SlashCommandStatus},
}

func ParseSlashCommand(message string) (SlashCommand, bool, error) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return SlashCommand{}, false, nil
	}

	tokens := strings.Fields(trimmed)
	if len(tokens) == 0 {
		return SlashCommand{}, false, nil
	}
	cmdToken := strings.TrimPrefix(strings.ToLower(tokens[0]), "/")

	command := SlashCommand{Raw: trimmed}
	switch SlashCommandType(cmdToken) {
	case SlashCommandApprove:
		command.Type = SlashCommandApprove
		return command, true, nil
	case SlashCommandStatus:
		command.Type = SlashCommandStatus
		return command, true, nil
	case SlashCommandAbort:
		command.Type = SlashCommandAbort
		return command, true, nil
	case SlashCommandRun:
		command.Type = SlashCommandRun
		if len(tokens) >= 2 {
			command.Template = strings.TrimSpace(tokens[1])
		}
		return command, true, nil
	case SlashCommandReject:
		command.Type = SlashCommandReject
		if len(tokens) < 2 {
			return SlashCommand{}, true, fmt.Errorf("/reject requires stage id")
		}
		stage := core.StageID(strings.TrimSpace(tokens[1]))
		if !isValidRejectStage(stage) {
			return SlashCommand{}, true, fmt.Errorf("invalid reject stage: %s", tokens[1])
		}
		command.Stage = stage
		if len(tokens) >= 3 {
			command.Reason = strings.TrimSpace(strings.Join(tokens[2:], " "))
		}
		return command, true, nil
	default:
		return SlashCommand{}, false, nil
	}
}

func IsSlashCommandAllowed(actor string, association string, command SlashCommandType, cfg SlashACLConfig) bool {
	if command == "" {
		return false
	}
	actorNormalized := strings.ToLower(strings.TrimSpace(actor))
	for _, username := range cfg.AuthorizedUsernames {
		if strings.ToLower(strings.TrimSpace(username)) == actorNormalized && actorNormalized != "" {
			return true
		}
	}

	permissions := defaultAssociationPermissions
	if len(cfg.AssociationPermissions) > 0 {
		permissions = cfg.AssociationPermissions
	}
	associationKey := strings.ToUpper(strings.TrimSpace(association))
	allowedCommands, ok := permissions[associationKey]
	if !ok {
		allowedCommands = permissions["NONE"]
	}
	for _, allowed := range allowedCommands {
		if allowed == command {
			return true
		}
	}
	return false
}

func isValidRejectStage(stage core.StageID) bool {
	return core.IsKnownStage(stage)
}
