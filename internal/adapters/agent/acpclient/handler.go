package acpclient

import (
	"context"

	acpproto "github.com/coder/acp-go-sdk"
)

type EventHandler interface {
	HandleSessionUpdate(ctx context.Context, update SessionUpdate) error
}

type NopHandler struct{}

func (h *NopHandler) ReadTextFile(context.Context, acpproto.ReadTextFileRequest) (acpproto.ReadTextFileResponse, error) {
	return acpproto.ReadTextFileResponse{}, nil
}

func (h *NopHandler) WriteTextFile(context.Context, acpproto.WriteTextFileRequest) (acpproto.WriteTextFileResponse, error) {
	return acpproto.WriteTextFileResponse{}, nil
}

func (h *NopHandler) RequestPermission(context.Context, acpproto.RequestPermissionRequest) (acpproto.RequestPermissionResponse, error) {
	return acpproto.RequestPermissionResponse{
		Outcome: acpproto.RequestPermissionOutcome{
			Cancelled: &acpproto.RequestPermissionOutcomeCancelled{Outcome: "cancelled"},
		},
	}, nil
}

func (h *NopHandler) CreateTerminal(context.Context, acpproto.CreateTerminalRequest) (acpproto.CreateTerminalResponse, error) {
	return acpproto.CreateTerminalResponse{}, nil
}

func (h *NopHandler) KillTerminalCommand(context.Context, acpproto.KillTerminalCommandRequest) (acpproto.KillTerminalCommandResponse, error) {
	return acpproto.KillTerminalCommandResponse{}, nil
}

func (h *NopHandler) TerminalOutput(context.Context, acpproto.TerminalOutputRequest) (acpproto.TerminalOutputResponse, error) {
	return acpproto.TerminalOutputResponse{}, nil
}

func (h *NopHandler) ReleaseTerminal(context.Context, acpproto.ReleaseTerminalRequest) (acpproto.ReleaseTerminalResponse, error) {
	return acpproto.ReleaseTerminalResponse{}, nil
}

func (h *NopHandler) WaitForTerminalExit(context.Context, acpproto.WaitForTerminalExitRequest) (acpproto.WaitForTerminalExitResponse, error) {
	return acpproto.WaitForTerminalExitResponse{}, nil
}

func (h *NopHandler) SessionUpdate(ctx context.Context, params acpproto.SessionNotification) error {
	update, ok := decodeACPNotificationFromStruct(params)
	if !ok {
		return nil
	}
	return h.HandleSessionUpdate(ctx, update)
}

func (h *NopHandler) HandleSessionUpdate(context.Context, SessionUpdate) error {
	return nil
}
