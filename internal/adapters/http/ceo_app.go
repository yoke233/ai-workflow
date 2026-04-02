package api

import (
	"github.com/yoke233/zhanggui/internal/application/ceoapp"
	"github.com/yoke233/zhanggui/internal/application/orchestrateapp"
)

func (h *Handler) ceoService() *ceoapp.Service {
	if h == nil {
		return nil
	}
	var planner orchestrateapp.Planner
	if h.dagGen != nil {
		planner = h.dagGen
	}
	workItems := h.workItemService()
	return ceoapp.New(ceoapp.Config{
		Requirements: h.requirementService(),
		Tasks: orchestrateapp.New(orchestrateapp.Config{
			Store:           h.store,
			WorkItemCreator: workItems,
			Deliverables:    workItems,
			Planner:         planner,
			Threads:         h.threadService(),
			Registry:        h.registry,
		}),
	})
}
