package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	requirementapp "github.com/yoke233/zhanggui/internal/application/requirementapp"
	"github.com/yoke233/zhanggui/internal/core"
)

type requirementCreateThreadResponse struct {
	Thread       *core.Thread             `json:"thread"`
	ContextRefs  []*core.ThreadContextRef `json:"context_refs,omitempty"`
	Agents       []string                 `json:"agents,omitempty"`
	Message      *core.ThreadMessage      `json:"message,omitempty"`
	InviteErrors map[string]string        `json:"invite_errors,omitempty"`
}

func registerRequirementRoutes(r chi.Router, h *Handler) {
	r.Post("/requirements/analyze", h.analyzeRequirement)
	r.Post("/requirements/create-thread", h.createThreadFromRequirement)
}

func (h *Handler) analyzeRequirement(w http.ResponseWriter, r *http.Request) {
	var req requirementapp.AnalyzeInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	result, err := h.requirementService().Analyze(r.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "description is required") {
			writeError(w, http.StatusBadRequest, err.Error(), "MISSING_DESCRIPTION")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "ANALYZE_REQUIREMENT_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createThreadFromRequirement(w http.ResponseWriter, r *http.Request) {
	var req requirementapp.CreateThreadInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	if strings.TrimSpace(req.OwnerID) == "" {
		req.OwnerID = "human"
	}
	result, err := h.requirementService().CreateThread(r.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "description is required") {
			writeError(w, http.StatusBadRequest, err.Error(), "MISSING_DESCRIPTION")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "CREATE_REQUIREMENT_THREAD_FAILED")
		return
	}
	kickoff, err := h.routeRequirementThreadKickoff(r.Context(), result.Thread.ID, req.OwnerID, req, result.AgentIDs, "requirements.create_thread")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "CREATE_REQUIREMENT_MESSAGE_FAILED")
		return
	}

	writeJSON(w, http.StatusCreated, requirementCreateThreadResponse{
		Thread:       result.Thread,
		ContextRefs:  result.ContextRefs,
		Agents:       kickoff.Agents,
		Message:      kickoff.Message,
		InviteErrors: kickoff.InviteErrors,
	})
}

func buildRequirementInitialMessage(input requirementapp.CreateThreadInput) string {
	var b strings.Builder
	b.WriteString("以下是本次新需求，请开始协作分析并收敛：\n\n")
	b.WriteString("需求描述：\n")
	b.WriteString(strings.TrimSpace(input.Description))
	if ctx := strings.TrimSpace(input.Context); ctx != "" {
		b.WriteString("\n\n补充上下文：\n")
		b.WriteString(ctx)
	}
	if input.Analysis != nil {
		if summary := strings.TrimSpace(input.Analysis.Summary); summary != "" {
			b.WriteString("\n\n分析摘要：\n")
			b.WriteString(summary)
		}
		if len(input.Analysis.Risks) > 0 {
			b.WriteString("\n\n已识别风险：\n")
			for _, risk := range input.Analysis.Risks {
				if strings.TrimSpace(risk) == "" {
					continue
				}
				fmt.Fprintf(&b, "- %s\n", strings.TrimSpace(risk))
			}
		}
	}
	b.WriteString("\n\n请各自从最适合的角度接力分析，必要时明确点名下一位参与者；如果讨论已经形成稳定方案，请创建 proposal 供人审阅。")
	return b.String()
}
