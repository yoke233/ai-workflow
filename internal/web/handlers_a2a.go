package web

import (
	"net/http"
	"strings"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/go-chi/chi/v5"
	"github.com/yoke233/ai-workflow/internal/teamleader"
)

func registerA2ARoutes(r chi.Router, cfg Config) {
	if !cfg.A2AEnabled {
		r.Handle("/api/v1/a2a", http.HandlerFunc(handleA2ADisabled))
		r.Handle("/.well-known/agent-card.json", http.HandlerFunc(handleA2ADisabled))
		return
	}

	r.Get("/.well-known/agent-card.json", handleA2AAgentCard(cfg))
	r.With(BearerAuthMiddleware(cfg.A2AToken)).Post("/api/v1/a2a", handleA2AJSONRPC(cfg))
}

func handleA2ADisabled(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func handleA2AAgentCard(cfg Config) http.HandlerFunc {
	version := strings.TrimSpace(cfg.A2AVersion)
	if version == "" {
		version = "0.3"
	}

	return func(w http.ResponseWriter, r *http.Request) {
		card := &a2a.AgentCard{
			Name:               "ai-workflow",
			Description:        "ai-workflow a2a endpoint",
			URL:                requestAbsoluteURL(r, "/api/v1/a2a"),
			PreferredTransport: a2a.TransportProtocolJSONRPC,
			ProtocolVersion:    version,
			Capabilities:       a2a.AgentCapabilities{Streaming: true},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
			Skills:             []a2a.AgentSkill{},
			Version:            "0.1.0",
		}
		writeJSON(w, http.StatusOK, card)
	}
}

func handleA2AJSONRPC(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := decodeA2ARPCRequest(r)
		if err != nil {
			writeA2ARPCError(w, nil, a2aRPCInvalidRequest, "invalid request")
			return
		}
		if strings.TrimSpace(req.JSONRPC) != a2aJSONRPCVersion {
			writeA2ARPCError(w, req.ID, a2aRPCInvalidRequest, "invalid request")
			return
		}

		method := strings.TrimSpace(req.Method)
		if method == "" {
			writeA2ARPCError(w, req.ID, a2aRPCInvalidRequest, "invalid request")
			return
		}
		switch method {
		case a2aMethodMessageSend:
			handleA2AMessageSend(w, r, cfg, req)
		case a2aMethodTasksGet:
			handleA2ATasksGet(w, r, cfg, req)
		case a2aMethodTasksCancel:
			handleA2ATasksCancel(w, r, cfg, req)
		case a2aMethodMessageStream:
			handleA2AMessageStream(w, r, cfg, req)
		default:
			writeA2ARPCError(w, req.ID, a2aRPCMethodNotFound, "method not found")
		}
	}
}

func handleA2AMessageSend(w http.ResponseWriter, r *http.Request, cfg Config, req a2aRPCRequest) {
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
	writeA2ARPCResult(w, req.ID, a2aTaskFromSnapshot(snapshot))
}

func handleA2ATasksGet(w http.ResponseWriter, r *http.Request, cfg Config, req a2aRPCRequest) {
	if cfg.A2ABridge == nil {
		writeA2ARPCError(w, req.ID, a2aRPCInternalError, "internal error")
		return
	}

	params, err := decodeA2ATaskQueryParams(req.Params)
	if err != nil {
		writeA2ARPCError(w, req.ID, a2aRPCInvalidParams, "invalid params")
		return
	}

	snapshot, err := cfg.A2ABridge.GetTask(r.Context(), teamleader.A2AGetTaskInput{
		ProjectID: a2aProjectID(params.Metadata),
		TaskID:    strings.TrimSpace(string(params.ID)),
	})
	if err != nil {
		code, message := mapA2ABridgeError(err)
		writeA2ARPCError(w, req.ID, code, message)
		return
	}
	writeA2ARPCResult(w, req.ID, a2aTaskFromSnapshot(snapshot))
}

func handleA2ATasksCancel(w http.ResponseWriter, r *http.Request, cfg Config, req a2aRPCRequest) {
	if cfg.A2ABridge == nil {
		writeA2ARPCError(w, req.ID, a2aRPCInternalError, "internal error")
		return
	}

	params, err := decodeA2ATaskIDParams(req.Params)
	if err != nil {
		writeA2ARPCError(w, req.ID, a2aRPCInvalidParams, "invalid params")
		return
	}

	snapshot, err := cfg.A2ABridge.CancelTask(r.Context(), teamleader.A2ACancelTaskInput{
		ProjectID: a2aProjectID(params.Metadata),
		TaskID:    strings.TrimSpace(string(params.ID)),
	})
	if err != nil {
		code, message := mapA2ABridgeError(err)
		writeA2ARPCError(w, req.ID, code, message)
		return
	}
	writeA2ARPCResult(w, req.ID, a2aTaskFromSnapshot(snapshot))
}

func requestAbsoluteURL(r *http.Request, path string) string {
	if r == nil {
		return path
	}

	host := strings.TrimSpace(firstForwardedValue(r.Header.Get("X-Forwarded-Host")))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		return path
	}

	scheme := strings.ToLower(strings.TrimSpace(firstForwardedValue(r.Header.Get("X-Forwarded-Proto"))))
	if scheme != "http" && scheme != "https" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	return scheme + "://" + host + path
}

func firstForwardedValue(raw string) string {
	for _, part := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
