package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/yoke233/ai-workflow/internal/adapters/store/sqlite"
	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/platform/appdata"
	"github.com/yoke233/ai-workflow/internal/platform/configruntime"
)

func buildRuntimeManager(store *sqlite.Store, runtimeDBPath string, bus core.EventBus) *configruntime.Manager {
	dataDir, err := appdata.ResolveDataDir()
	if err != nil {
		return nil
	}

	cfgPath := filepath.Join(dataDir, "config.toml")
	if resolved := resolveRuntimeConfigFilePath(dataDir); resolved != "" {
		cfgPath = resolved
	}
	secretsPath := resolveSecretsFilePath(dataDir)
	mcpEnv := configruntime.MCPEnvConfig{
		DBPath: runtimeDBPath,
	}
	runtimeManager, err := configruntime.NewManager(cfgPath, secretsPath, mcpEnv, slog.Default(), func(ctx context.Context, snap *configruntime.Snapshot) error {
		if err := configruntime.SyncRegistry(ctx, store, snap); err != nil {
			return err
		}
		if bus != nil && snap != nil && snap.Config != nil && snap.Version > 1 {
			bus.Publish(context.Background(), core.Event{
				Type: core.EventRuntimeConfigReloaded,
				Data: map[string]any{
					"version":       snap.Version,
					"loaded_at":     snap.LoadedAt.Format(time.RFC3339),
					"driver_count":  len(snap.Config.Runtime.Agents.Drivers),
					"profile_count": len(snap.Config.Runtime.Agents.Profiles),
				},
				Timestamp: time.Now().UTC(),
			})
		}
		return nil
	})
	if err != nil {
		slog.Warn("bootstrap: config runtime disabled", "error", err)
		return nil
	}
	return runtimeManager
}

func resolveRuntimeConfigFilePath(dataDir string) string {
	for _, name := range []string{"config.toml", "config.yaml", "config.yml"} {
		path := filepath.Join(dataDir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join(dataDir, "config.toml")
}
