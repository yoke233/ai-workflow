package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	acpproto "github.com/coder/acp-go-sdk"
	"github.com/nats-io/nats.go"
	"github.com/yoke233/ai-workflow/internal/config"
	"github.com/yoke233/ai-workflow/internal/teamleader"
	v2engine "github.com/yoke233/ai-workflow/internal/v2/engine"
	v2sqlite "github.com/yoke233/ai-workflow/internal/v2/store/sqlite"
)

// cmdExecutor runs a remote executor worker that connects to NATS and processes
// ACP execution messages. This is the `ai-flow executor` subcommand.
//
// Usage:
//
//	ai-flow executor --nats-url nats://localhost:4222 [--agents claude,codex] [--max-concurrent 2]
func cmdExecutor(args []string) error {
	opts, err := parseExecutorArgs(args)
	if err != nil {
		return err
	}
	natsURL := opts.natsURL

	if natsURL == "" {
		natsURL = os.Getenv("AI_WORKFLOW_NATS_URL")
	}
	if natsURL == "" {
		return fmt.Errorf("--nats-url is required (or set AI_WORKFLOW_NATS_URL)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load config for agent registry.
	cfg, err := loadBootstrapConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Open the v2 store for agent profile/driver resolution.
	dbPath := expandStorePath(cfg.Store.Path)
	v2DBPath := strings.TrimSuffix(dbPath, ".db") + "_v2.db"
	store, err := v2sqlite.New(v2DBPath)
	if err != nil {
		return fmt.Errorf("open v2 store: %w", err)
	}
	defer store.Close()

	// Seed registry.
	seedV2Registry(context.Background(), store, cfg, nil)
	runtimeManager := buildV2RuntimeManager(store, buildExecutorMCPEnv(cfg))
	if runtimeManager != nil {
		defer func() {
			_ = runtimeManager.Close()
		}()
	}

	// Connect to NATS.
	nc, err := nats.Connect(natsURL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return fmt.Errorf("connect to NATS at %s: %w", natsURL, err)
	}
	defer nc.Drain()

	slog.Info("executor: connected to NATS", "url", natsURL)

	streamPrefix := "aiworkflow"
	if cfg.V2.SessionManager.NATS.StreamPrefix != "" {
		streamPrefix = cfg.V2.SessionManager.NATS.StreamPrefix
	}

	worker, err := v2engine.NewExecutorWorker(v2engine.ExecutorWorkerConfig{
		NATSConn:       nc,
		StreamPrefix:   streamPrefix,
		WorkerID:       cfg.V2.SessionManager.ServerID,
		AgentTypes:     opts.agentTypes,
		Store:          store,
		Registry:       store,
		DefaultWorkDir: resolveDefaultWorkDir(cfg),
		MaxConcurrent:  opts.maxConcurrent,
		MCPEnv:         buildExecutorMCPEnv(cfg),
		MCPResolver: func(profileID string, agentSupportsSSE bool) []acpproto.McpServer {
			if runtimeManager == nil {
				return nil
			}
			return runtimeManager.ResolveMCPServers(profileID, agentSupportsSSE)
		},
	})
	if err != nil {
		return fmt.Errorf("create executor worker: %w", err)
	}

	slog.Info("executor: starting worker", "agents", opts.agentTypes, "max_concurrent", opts.maxConcurrent)

	err = worker.Start(ctx)
	worker.Stop()

	if ctx.Err() != nil {
		slog.Info("executor: shutting down")
		return nil
	}
	return err
}

func resolveDefaultWorkDir(cfg *config.Config) string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

type executorCLIOptions struct {
	natsURL       string
	agentTypes    []string
	maxConcurrent int
}

func parseExecutorArgs(args []string) (executorCLIOptions, error) {
	opts := executorCLIOptions{maxConcurrent: 2}
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		switch {
		case arg == "--nats-url":
			i++
			if i >= len(args) {
				return executorCLIOptions{}, fmt.Errorf("missing value for --nats-url")
			}
			opts.natsURL = strings.TrimSpace(args[i])
		case strings.HasPrefix(arg, "--nats-url="):
			opts.natsURL = strings.TrimSpace(strings.TrimPrefix(arg, "--nats-url="))
		case arg == "--agents":
			i++
			if i >= len(args) {
				return executorCLIOptions{}, fmt.Errorf("missing value for --agents")
			}
			opts.agentTypes = parseAgentTypes(args[i])
		case strings.HasPrefix(arg, "--agents="):
			opts.agentTypes = parseAgentTypes(strings.TrimPrefix(arg, "--agents="))
		case arg == "--max-concurrent":
			i++
			if i >= len(args) {
				return executorCLIOptions{}, fmt.Errorf("missing value for --max-concurrent")
			}
			n, err := parsePositiveInt(args[i], "--max-concurrent")
			if err != nil {
				return executorCLIOptions{}, err
			}
			opts.maxConcurrent = n
		case strings.HasPrefix(arg, "--max-concurrent="):
			n, err := parsePositiveInt(strings.TrimPrefix(arg, "--max-concurrent="), "--max-concurrent")
			if err != nil {
				return executorCLIOptions{}, err
			}
			opts.maxConcurrent = n
		default:
			return executorCLIOptions{}, fmt.Errorf("unknown flag: %s\nusage: ai-flow executor --nats-url <url> [--agents claude,codex] [--max-concurrent 2]", arg)
		}
	}
	return opts, nil
}

func parseAgentTypes(raw string) []string {
	var agents []string
	for _, a := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(a); t != "" {
			agents = append(agents, t)
		}
	}
	return agents
}

func parsePositiveInt(raw string, flagName string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid value for %s: %s", flagName, raw)
	}
	return n, nil
}

func buildExecutorMCPEnv(cfg *config.Config) teamleader.MCPEnvConfig {
	return teamleader.MCPEnvConfig{
		DBPath: expandStorePath(cfg.Store.Path),
	}
}
