package api

import requirementapp "github.com/yoke233/zhanggui/internal/application/requirementapp"

func (h *Handler) requirementService() *requirementapp.Service {
	if h == nil {
		return nil
	}
	return requirementapp.New(requirementapp.Config{
		Store:         h.store,
		Registry:      h.registry,
		ThreadService: h.threadService(),
		LLM:           h.requirementLLM,
	})
}
