package web

import (
	"net/http"

	"github.com/yoke233/ai-workflow/internal/acpclient"
)

type agentHandlers struct {
	resolver *acpclient.RoleResolver
}

type agentInfo struct {
	Name string `json:"name"`
}

type listAgentsResponse struct {
	Agents []agentInfo `json:"agents"`
}

func (h *agentHandlers) list(w http.ResponseWriter, r *http.Request) {
	agents := h.resolver.ListAgents()
	items := make([]agentInfo, 0, len(agents))
	for _, a := range agents {
		items = append(items, agentInfo{Name: a.ID})
	}
	writeJSON(w, http.StatusOK, listAgentsResponse{Agents: items})
}
