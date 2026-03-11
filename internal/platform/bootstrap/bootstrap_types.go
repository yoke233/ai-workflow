package bootstrap

import (
	"context"

	"github.com/go-chi/chi/v5"
	chatacp "github.com/yoke233/ai-workflow/internal/adapters/chat/acp"
	"github.com/yoke233/ai-workflow/internal/adapters/llm"
	"github.com/yoke233/ai-workflow/internal/adapters/store/sqlite"
	flowapp "github.com/yoke233/ai-workflow/internal/application/flow"
	probeapp "github.com/yoke233/ai-workflow/internal/application/probe"
	runtimeapp "github.com/yoke233/ai-workflow/internal/application/runtime"
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/platform/configruntime"
)

type bootstrapBase struct {
	runtimeDBPath  string
	store          *sqlite.Store
	bus            core.EventBus
	persister      *flowapp.EventPersister
	registry       core.AgentRegistry
	runtimeManager *configruntime.Manager
	dataDir        string
}

type flowStack struct {
	sessionMode   string
	sessionMgr    runtimeapp.SessionManager
	llmClient     *llm.Client
	engine        *flowapp.FlowEngine
	scheduler     *flowapp.FlowScheduler
	schedulerStop context.CancelFunc
}

type apiStack struct {
	leadAgent *chatacp.LeadAgent
	probeSvc  *probeapp.ExecutionProbeService
	registrar func(chi.Router)
}

type bootstrapLifecycle struct {
	runtimeWatchCancel context.CancelFunc
	probeWatchCancel   context.CancelFunc
}
