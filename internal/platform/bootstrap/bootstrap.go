package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nats.go"
	"github.com/yoke233/ai-workflow/internal/adapters/agent/acpclient"
	chatacp "github.com/yoke233/ai-workflow/internal/adapters/chat/acp"
	llmcollector "github.com/yoke233/ai-workflow/internal/adapters/collector/llm"
	membus "github.com/yoke233/ai-workflow/internal/adapters/events/memory"
	"github.com/yoke233/ai-workflow/internal/adapters/http"
	"github.com/yoke233/ai-workflow/internal/adapters/llm"
	llmplanning "github.com/yoke233/ai-workflow/internal/adapters/planning/llm"
	scmadapter "github.com/yoke233/ai-workflow/internal/adapters/scm"
	"github.com/yoke233/ai-workflow/internal/adapters/store/sqlite"
	workspaceprovider "github.com/yoke233/ai-workflow/internal/adapters/workspace/provider"
	flowapp "github.com/yoke233/ai-workflow/internal/application/flow"
	probeapp "github.com/yoke233/ai-workflow/internal/application/probe"
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/engine"
	"github.com/yoke233/ai-workflow/internal/platform/appdata"
	"github.com/yoke233/ai-workflow/internal/platform/config"
	"github.com/yoke233/ai-workflow/internal/platform/configruntime"
)

// seedRegistry seeds agent drivers and profiles into the SQLite store from TOML config.
// Uses upsert so TOML always acts as the source of truth for configured agents,
// while runtime additions via API are also persisted.
func seedRegistry(ctx context.Context, store *sqlite.Store, cfg *config.Config, _ *acpclient.RoleResolver) {
	if cfg == nil {
		return
	}

	drivers, profiles := configruntime.BuildAgents(cfg)
	if len(drivers) == 0 {
		slog.Warn("registry: no agent config to seed")
		return
	}

	for _, d := range drivers {
		if err := store.UpsertDriver(ctx, d); err != nil {
			slog.Warn("registry: seed driver failed", "id", d.ID, "error", err)
		}
	}
	for _, p := range profiles {
		if err := store.UpsertProfile(ctx, p); err != nil {
			slog.Warn("registry: seed profile failed", "id", p.ID, "error", err)
		}
	}
	slog.Info("registry: seeded from config", "drivers", len(drivers), "profiles", len(profiles))
}

type gitHubTokens struct {
	CommitPAT string
	MergePAT  string
}

// bootstrap creates the runtime store, event bus, engine, event persister, and API handler.
// Returns the store (for lifecycle), the agent registry, runtime manager, cleanup func, and route registrar.
func bootstrap(storePath string, roleResolver *acpclient.RoleResolver, bootstrapCfg *config.Config, ghTokens gitHubTokens, upgradeFn engine.UpgradeFunc) (*sqlite.Store, core.AgentRegistry, *configruntime.Manager, func(), func(chi.Router)) {
	runtimeDBPath := strings.TrimSuffix(storePath, filepath.Ext(storePath)) + "_runtime.db"
	store, err := sqlite.New(runtimeDBPath)
	if err != nil {
		slog.Error("bootstrap: failed to open store", "path", runtimeDBPath, "error", err)
		return nil, nil, nil, nil, nil
	}

	bus := membus.NewBus()
	acpPool := engine.NewACPSessionPool(store, bus)

	persister := flowapp.NewEventPersister(store, bus)
	if err := persister.Start(context.Background()); err != nil {
		slog.Error("bootstrap: failed to start event persister", "error", err)
		store.Close()
		return nil, nil, nil, nil, nil
	}

	seedRegistry(context.Background(), store, bootstrapCfg, roleResolver)
	runtimeManager := buildRuntimeManager(store)

	var registry core.AgentRegistry = store

	dataDir := ""
	if dd, err := appdata.ResolveDataDir(); err == nil {
		dataDir = dd
	}
	sb := buildSandbox(bootstrapCfg, dataDir)

	// Build SessionManager based on config mode.
	var sessionMgr engine.SessionManager
	smMode := ""
	if bootstrapCfg != nil {
		smMode = strings.TrimSpace(strings.ToLower(bootstrapCfg.Runtime.SessionManager.Mode))
	}
	switch smMode {
	case "nats":
		natsMgr, natsErr := buildNATSSessionManager(bootstrapCfg, store, dataDir)
		if natsErr != nil {
			slog.Error("bootstrap: NATS session manager failed, falling back to local", "error", natsErr)
			sessionMgr = engine.NewLocalSessionManager(acpPool, store, sb)
		} else {
			sessionMgr = natsMgr
			slog.Info("bootstrap: using NATS session manager")
		}
	default:
		sessionMgr = engine.NewLocalSessionManager(acpPool, store, sb)
		slog.Info("bootstrap: using local session manager")
	}

	mockEnabled := bootstrapCfg != nil && bootstrapCfg.Runtime.MockExecutor
	if !mockEnabled {
		if raw := strings.TrimSpace(os.Getenv("AI_WORKFLOW_MOCK_EXECUTOR")); raw != "" {
			switch strings.ToLower(raw) {
			case "1", "true", "yes", "on":
				mockEnabled = true
			}
		}
	}

	var executor flowapp.StepExecutor
	if mockEnabled {
		slog.Warn("bootstrap: using mock step executor (no ACP processes will be spawned)")
		executor = engine.NewMockStepExecutor(store, bus)
	} else {
		executor = engine.NewACPStepExecutor(engine.ACPExecutorConfig{
			Registry:       registry,
			Store:          store,
			Bus:            bus,
			SessionManager: sessionMgr,
			ReworkFollowupTemplate: func() string {
				if bootstrapCfg == nil {
					return ""
				}
				return bootstrapCfg.Runtime.Prompts.ReworkFollowup
			}(),
			ContinueFollowupTemplate: func() string {
				if bootstrapCfg == nil {
					return ""
				}
				return bootstrapCfg.Runtime.Prompts.ContinueFollowup
			}(),
		})
	}

	wsProvider := workspaceprovider.NewCompositeProvider()
	var llmClient *llm.Client
	engOpts := []flowapp.Option{
		flowapp.WithWorkspaceProvider(wsProvider),
		flowapp.WithGitHubTokens(flowapp.GitHubTokens{
			CommitPAT: strings.TrimSpace(ghTokens.CommitPAT),
			MergePAT:  strings.TrimSpace(ghTokens.MergePAT),
		}),
		flowapp.WithPRFlowPromptsProvider(func() flowapp.PRFlowPrompts {
			return currentPRFlowPrompts(runtimeManager, bootstrapCfg)
		}),
		flowapp.WithChangeRequestProviders(scmadapter.NewChangeRequestProviders),
	}

	if bootstrapCfg != nil {
		openaiCfg := bootstrapCfg.Runtime.Collector.OpenAI
		if strings.TrimSpace(openaiCfg.APIKey) != "" && strings.TrimSpace(openaiCfg.Model) != "" {
			c, err := llm.New(llm.Config{
				BaseURL:    openaiCfg.BaseURL,
				APIKey:     openaiCfg.APIKey,
				Model:      openaiCfg.Model,
				MaxRetries: bootstrapCfg.Runtime.Collector.MaxRetries,
			})
			if err != nil {
				slog.Warn("bootstrap: LLM client disabled (invalid openai config)", "error", err)
			} else {
				llmClient = c
				engOpts = append(engOpts, flowapp.WithCollector(llmcollector.NewLLMCollector(llmClient.Complete)))
				slog.Info("bootstrap: LLM client enabled (collector + DAG generator)")
			}
		}
	}

	executor = engine.NewCompositeStepExecutor(engine.CompositeStepExecutorConfig{
		Store: store,
		Bus:   bus,
		GitHubTokens: flowapp.GitHubTokens{
			CommitPAT: strings.TrimSpace(ghTokens.CommitPAT),
			MergePAT:  strings.TrimSpace(ghTokens.MergePAT),
		},
		UpgradeFunc: upgradeFn,
		ACPExecutor: executor,
	})

	engOpts = append(engOpts, flowapp.WithBriefingBuilder(flowapp.NewBriefingBuilder(store)))
	eng := flowapp.New(store, bus, executor, engOpts...)

	scheduler := flowapp.NewFlowScheduler(eng, store, bus, flowapp.FlowSchedulerConfig{MaxConcurrentFlows: 2})
	schedCtx, schedCancel := context.WithCancel(context.Background())
	go scheduler.Start(schedCtx)

	recoverFlows := flowapp.RecoverInterruptedFlows
	recoveryLogLabel := "interrupted flows"
	if smMode == "nats" {
		recoverFlows = flowapp.RecoverQueuedFlows
		recoveryLogLabel = "queued flows"
		slog.Warn("bootstrap: skipping running-flow recovery in NATS mode until execution recovery is implemented")
	}
	if n, err := recoverFlows(context.Background(), store, scheduler); err != nil {
		slog.Warn("bootstrap: flow recovery error", "error", err)
	} else if n > 0 {
		slog.Info("bootstrap: recovered flows", "kind", recoveryLogLabel, "count", n)
	}

	leadAgent := chatacp.NewLeadAgent(chatacp.LeadAgentConfig{
		Registry: registry,
		Bus:      bus,
		Sandbox:  sb,
	})
	var leadChatService api.LeadChatService = leadAgent

	var dagGen api.DAGGenerator
	if llmClient != nil {
		dagGen = llmplanning.NewDAGGenerator(llmClient, registry)
	}

	probeSvc := probeapp.NewExecutionProbeService(probeapp.ExecutionProbeServiceConfig{
		Store:          store,
		Bus:            bus,
		SessionManager: sessionMgr,
	})

	apiOpts := buildAPIOptions(bootstrapCfg, runtimeManager, leadChatService, scheduler, registry, dagGen)
	apiOpts = append(apiOpts, api.WithExecutionProbeService(probeSvc))
	handler := api.NewHandler(store, bus, eng, apiOpts...)
	registrar := func(r chi.Router) { handler.Register(r) }

	var runtimeWatchCancel context.CancelFunc
	var probeWatchCancel context.CancelFunc
	cleanup := func() {
		if runtimeWatchCancel != nil {
			runtimeWatchCancel()
		}
		if probeWatchCancel != nil {
			probeWatchCancel()
		}
		if runtimeManager != nil {
			_ = runtimeManager.Close()
		}
		if sessionMgr != nil {
			sessionMgr.Close()
		}
		if leadAgent != nil {
			leadAgent.Shutdown()
		}
		schedCancel()
		scheduler.Shutdown()
		persister.Stop()
		store.Close()
	}

	if runtimeManager != nil {
		watchCtx, cancel := context.WithCancel(context.Background())
		runtimeWatchCancel = cancel
		if err := runtimeManager.Start(watchCtx); err != nil {
			slog.Warn("bootstrap: config runtime watcher disabled", "error", err)
		}
	}

	if bootstrapCfg != nil && bootstrapCfg.Runtime.ExecutionProbe.Enabled {
		probeWatchdog := probeapp.NewExecutionProbeWatchdog(store, probeSvc, probeapp.ExecutionProbeWatchdogConfig{
			Enabled:      bootstrapCfg.Runtime.ExecutionProbe.Enabled,
			Interval:     bootstrapCfg.Runtime.ExecutionProbe.Interval.Duration,
			ProbeAfter:   bootstrapCfg.Runtime.ExecutionProbe.After.Duration,
			IdleAfter:    bootstrapCfg.Runtime.ExecutionProbe.IdleAfter.Duration,
			ProbeTimeout: bootstrapCfg.Runtime.ExecutionProbe.Timeout.Duration,
			MaxAttempts:  bootstrapCfg.Runtime.ExecutionProbe.MaxAttempts,
		})
		watchCtx, cancel := context.WithCancel(context.Background())
		probeWatchCancel = cancel
		go probeWatchdog.Start(watchCtx)
	}

	slog.Info("engine bootstrapped", "db", runtimeDBPath)
	return store, registry, runtimeManager, cleanup, registrar
}

// buildNATSSessionManager creates a NATS-backed session manager from config.
func buildNATSSessionManager(cfg *config.Config, store core.Store, dataDir string) (*engine.NATSSessionManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	natsCfg := cfg.Runtime.SessionManager.NATS

	natsURL := strings.TrimSpace(natsCfg.URL)
	if natsURL == "" && !natsCfg.Embedded {
		return nil, fmt.Errorf("nats.url is required when mode=nats and embedded=false")
	}

	if natsCfg.Embedded {
		// TODO: start embedded NATS server when github.com/nats-io/nats-server is available here.
		// For now, require an external NATS server.
		if natsURL == "" {
			return nil, fmt.Errorf("embedded NATS not yet implemented; provide nats.url")
		}
	}

	nc, err := natsConnect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}

	prefix := strings.TrimSpace(natsCfg.StreamPrefix)
	if prefix == "" {
		prefix = "aiworkflow"
	}

	serverID := strings.TrimSpace(cfg.Runtime.SessionManager.ServerID)

	return engine.NewNATSSessionManager(engine.NATSSessionManagerConfig{
		NATSConn:     nc,
		StreamPrefix: prefix,
		ServerID:     serverID,
		Store:        store,
	})
}

// natsConnect connects to a NATS server with retry.
func natsConnect(url string) (*nats.Conn, error) {
	nc, err := nats.Connect(url,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, err
	}
	return nc, nil
}
