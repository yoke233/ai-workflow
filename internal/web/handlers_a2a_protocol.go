package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/yoke233/ai-workflow/internal/teamleader"
)

const (
	a2aJSONRPCVersion      = "2.0"
	a2aRPCMethodNotFound   = -32601
	a2aRPCInvalidRequest   = -32600
	a2aRPCInvalidParams    = -32602
	a2aRPCInternalError    = -32603
	a2aRPCTaskNotFound     = -32004
	a2aRPCProjectScopeCode = -39001
	a2aMethodMessageSend   = "message/send"
	a2aMethodMessageStream = "message/stream"
	a2aMethodTasksGet      = "tasks/get"
	a2aMethodTasksCancel   = "tasks/cancel"
)

type a2aRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type a2aRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type a2aRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *a2aRPCError    `json:"error,omitempty"`
}

func decodeA2ARPCRequest(r *http.Request) (a2aRPCRequest, error) {
	var req a2aRPCRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		return a2aRPCRequest{}, fmt.Errorf("decode a2a rpc request: %w", err)
	}
	return req, nil
}

func writeA2ARPCError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	resp := a2aRPCResponse{
		JSONRPC: a2aJSONRPCVersion,
		ID:      id,
		Error: &a2aRPCError{
			Code:    code,
			Message: message,
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeA2ARPCResult(w http.ResponseWriter, id json.RawMessage, result any) {
	resp := a2aRPCResponse{
		JSONRPC: a2aJSONRPCVersion,
		ID:      id,
		Result:  result,
	}
	writeJSON(w, http.StatusOK, resp)
}

func decodeA2AMessageSendParams(raw json.RawMessage) (a2a.MessageSendParams, error) {
	var params a2a.MessageSendParams
	if err := decodeA2ARPCParams(raw, &params); err != nil {
		return a2a.MessageSendParams{}, err
	}
	if params.Message == nil {
		return a2a.MessageSendParams{}, errors.New("message is required")
	}
	if strings.TrimSpace(a2aMessageText(params.Message)) == "" {
		return a2a.MessageSendParams{}, errors.New("message text is required")
	}
	return params, nil
}

func decodeA2ATaskQueryParams(raw json.RawMessage) (a2a.TaskQueryParams, error) {
	var params a2a.TaskQueryParams
	if err := decodeA2ARPCParams(raw, &params); err != nil {
		return a2a.TaskQueryParams{}, err
	}
	if strings.TrimSpace(string(params.ID)) == "" {
		return a2a.TaskQueryParams{}, errors.New("task id is required")
	}
	return params, nil
}

func decodeA2ATaskIDParams(raw json.RawMessage) (a2a.TaskIDParams, error) {
	var params a2a.TaskIDParams
	if err := decodeA2ARPCParams(raw, &params); err != nil {
		return a2a.TaskIDParams{}, err
	}
	if strings.TrimSpace(string(params.ID)) == "" {
		return a2a.TaskIDParams{}, errors.New("task id is required")
	}
	return params, nil
}

func decodeA2ARPCParams(raw json.RawMessage, out any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return errors.New("params are required")
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode params: %w", err)
	}
	return nil
}

func a2aTaskFromSnapshot(snapshot *teamleader.A2ATaskSnapshot) *a2a.Task {
	task := &a2a.Task{
		Status: a2a.TaskStatus{
			State: a2a.TaskStateUnknown,
		},
	}
	if snapshot == nil {
		return task
	}

	status := a2a.TaskStatus{
		State: a2aTaskStateFromSnapshot(snapshot.State),
	}
	if !snapshot.UpdatedAt.IsZero() {
		updatedAt := snapshot.UpdatedAt
		status.Timestamp = &updatedAt
	}

	task.ID = a2a.TaskID(strings.TrimSpace(snapshot.TaskID))
	task.ContextID = strings.TrimSpace(snapshot.SessionID)
	task.Status = status
	if projectID := strings.TrimSpace(snapshot.ProjectID); projectID != "" {
		task.Metadata = map[string]any{
			"project_id": projectID,
		}
	}
	return task
}

func a2aTaskStateFromSnapshot(state teamleader.A2ATaskState) a2a.TaskState {
	switch state {
	case teamleader.A2ATaskStateSubmitted:
		return a2a.TaskStateSubmitted
	case teamleader.A2ATaskStateWorking:
		return a2a.TaskStateWorking
	case teamleader.A2ATaskStateInputRequired:
		return a2a.TaskStateInputRequired
	case teamleader.A2ATaskStateCompleted:
		return a2a.TaskStateCompleted
	case teamleader.A2ATaskStateFailed:
		return a2a.TaskStateFailed
	case teamleader.A2ATaskStateCanceled:
		return a2a.TaskStateCanceled
	default:
		return a2a.TaskStateUnknown
	}
}

func mapA2ABridgeError(err error) (int, string) {
	switch {
	case err == nil:
		return 0, ""
	case errors.Is(err, teamleader.ErrA2AInvalidInput):
		return a2aRPCInvalidParams, "invalid params"
	case errors.Is(err, teamleader.ErrA2ATaskNotFound):
		return a2aRPCTaskNotFound, "task not found"
	case errors.Is(err, teamleader.ErrA2AProjectScope):
		return a2aRPCProjectScopeCode, "project scope mismatch"
	default:
		return a2aRPCInternalError, "internal error"
	}
}

func a2aProjectID(metadata map[string]any) string {
	if len(metadata) == 0 {
		return ""
	}
	raw, ok := metadata["project_id"]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func a2aMessageText(msg *a2a.Message) string {
	if msg == nil || len(msg.Parts) == 0 {
		return ""
	}

	textParts := make([]string, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		switch typed := part.(type) {
		case a2a.TextPart:
			text := strings.TrimSpace(typed.Text)
			if text != "" {
				textParts = append(textParts, text)
			}
		case *a2a.TextPart:
			if typed == nil {
				continue
			}
			text := strings.TrimSpace(typed.Text)
			if text != "" {
				textParts = append(textParts, text)
			}
		}
	}
	return strings.TrimSpace(strings.Join(textParts, "\n"))
}
