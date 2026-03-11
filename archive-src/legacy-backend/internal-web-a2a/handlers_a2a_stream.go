package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/yoke233/ai-workflow/internal/teamleader"
)

type a2aStreamEvent struct {
	Name string
	Data any
}

func handleA2AMessageStream(w http.ResponseWriter, r *http.Request, cfg Config, req a2aRPCRequest) {
	if cfg.A2ABridge == nil {
		writeA2ARPCError(w, req.ID, a2aRPCInternalError, "internal error")
		return
	}

	params, err := decodeA2AMessageSendParams(req.Params)
	if err != nil {
		writeA2ARPCError(w, req.ID, a2aRPCInvalidParams, "invalid params")
		return
	}

	snapshot, err := cfg.A2ABridge.SendMessage(r.Context(), teamleader.A2ASendMessageInput{
		ProjectID:    a2aProjectID(params.Metadata),
		SessionID:    strings.TrimSpace(params.Message.ContextID),
		Conversation: a2aMessageText(params.Message),
	})
	if err != nil {
		code, message := mapA2ABridgeError(err)
		writeA2ARPCError(w, req.ID, code, message)
		return
	}

	events := buildA2AStreamEvents(a2aMessageText(params.Message), snapshot)
	if err := writeA2AStreamEvents(r.Context(), w, events); err != nil && !errors.Is(err, context.Canceled) {
		// Best effort: if streaming is unavailable before headers are written, fallback to JSON-RPC error.
		if !headerWritten(w) {
			writeA2ARPCError(w, req.ID, a2aRPCInternalError, "internal error")
		}
	}
}

func buildA2AStreamEvents(messageText string, snapshot *teamleader.A2ATaskSnapshot) []a2aStreamEvent {
	fragments := splitA2AStreamFragments(messageText)
	events := make([]a2aStreamEvent, 0, len(fragments)+2)
	for _, fragment := range fragments {
		events = append(events, a2aStreamEvent{
			Name: "delta",
			Data: map[string]any{
				"text": fragment,
			},
		})
	}

	events = append(events, a2aStreamEvent{
		Name: "task",
		Data: a2aTaskFromSnapshot(snapshot),
	})

	done := map[string]any{"done": true}
	if snapshot != nil {
		if taskID := strings.TrimSpace(snapshot.TaskID); taskID != "" {
			done["id"] = taskID
		}
	}
	events = append(events, a2aStreamEvent{
		Name: "done",
		Data: done,
	})
	return events
}

func splitA2AStreamFragments(text string) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}

	words := strings.Fields(trimmed)
	if len(words) >= 2 {
		return words
	}

	runes := []rune(trimmed)
	if len(runes) <= 1 {
		return []string{trimmed}
	}

	mid := len(runes) / 2
	left := strings.TrimSpace(string(runes[:mid]))
	right := strings.TrimSpace(string(runes[mid:]))
	out := make([]string, 0, 2)
	if left != "" {
		out = append(out, left)
	}
	if right != "" {
		out = append(out, right)
	}
	if len(out) == 0 {
		return []string{trimmed}
	}
	return out
}

func writeA2AStreamEvents(ctx context.Context, w http.ResponseWriter, events []a2aStreamEvent) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("response writer does not support streaming")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return err
		}

		name := strings.TrimSpace(event.Name)
		if name == "" {
			name = "message"
		}
		payload, err := json.Marshal(event.Data)
		if err != nil {
			return fmt.Errorf("marshal stream event %q: %w", name, err)
		}
		if _, err := fmt.Fprintf(w, "event: %s\n", name); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return err
		}
		flusher.Flush()
	}
	return nil
}

func headerWritten(w http.ResponseWriter) bool {
	recorder, ok := w.(*statusRecorder)
	if !ok {
		return false
	}
	return recorder.wroteHeader
}
