package bootstrap

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/yoke233/ai-workflow/internal/adapters/sandbox"
	runtimeapp "github.com/yoke233/ai-workflow/internal/application/runtime"
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/platform/config"
	agentruntime "github.com/yoke233/ai-workflow/internal/runtime/agent"
)

func buildSessionManager(
	bootstrapCfg *config.Config,
	store core.Store,
	dataDir string,
	acpPool *agentruntime.ACPSessionPool,
	sb sandbox.Sandbox,
) (runtimeapp.SessionManager, string) {
	smMode := ""
	if bootstrapCfg != nil {
		smMode = strings.TrimSpace(strings.ToLower(bootstrapCfg.Runtime.SessionManager.Mode))
	}

	local := func() runtimeapp.SessionManager {
		return agentruntime.NewLocalSessionManager(acpPool, store, sb)
	}

	if smMode == "nats" {
		natsMgr, err := buildNATSSessionManager(bootstrapCfg, store, dataDir)
		if err != nil {
			slog.Error("bootstrap: NATS session manager failed, falling back to local", "error", err)
			slog.Info("bootstrap: using local session manager")
			return local(), smMode
		}
		slog.Info("bootstrap: using NATS session manager")
		return natsMgr, smMode
	}

	slog.Info("bootstrap: using local session manager")
	return local(), smMode
}

func buildNATSSessionManager(cfg *config.Config, store core.Store, _ string) (*agentruntime.NATSSessionManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	natsCfg := cfg.Runtime.SessionManager.NATS

	natsURL := strings.TrimSpace(natsCfg.URL)
	if natsURL == "" && !natsCfg.Embedded {
		return nil, fmt.Errorf("nats.url is required when mode=nats and embedded=false")
	}
	if natsCfg.Embedded && natsURL == "" {
		return nil, fmt.Errorf("embedded NATS not yet implemented; provide nats.url")
	}

	nc, err := natsConnect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}

	prefix := strings.TrimSpace(natsCfg.StreamPrefix)
	if prefix == "" {
		prefix = "aiworkflow"
	}

	return agentruntime.NewNATSSessionManager(agentruntime.NATSSessionManagerConfig{
		NATSConn:     nc,
		StreamPrefix: prefix,
		ServerID:     strings.TrimSpace(cfg.Runtime.SessionManager.ServerID),
		Store:        store,
	})
}

func natsConnect(url string) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(2 * time.Second),
	}
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("empty nats url")
	}
	return nats.Connect(url, opts...)
}
