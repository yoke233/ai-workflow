package bootstrap

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/yoke233/ai-workflow/internal/adapters/sandbox"
	"github.com/yoke233/ai-workflow/internal/platform/config"
)

func buildSandbox(cfg *config.Config, dataDir string) sandbox.Sandbox {
	if cfg == nil || !cfg.Runtime.Sandbox.Enabled {
		return sandbox.NoopSandbox{}
	}

	requireAuth := false
	if raw := strings.ToLower(strings.TrimSpace(os.Getenv("AI_WORKFLOW_CODEX_REQUIRE_AUTH"))); raw != "" {
		switch raw {
		case "1", "true", "yes", "on":
			requireAuth = true
		}
	}

	homeSandbox := sandbox.HomeDirSandbox{
		DataDir:          dataDir,
		SkillsRoot:       filepath.Join(dataDir, "skills"),
		RequireCodexAuth: requireAuth,
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Runtime.Sandbox.Provider)) {
	case "", "home_dir":
		return homeSandbox
	case "litebox":
		return sandbox.LiteBoxSandbox{
			Base:          homeSandbox,
			BridgeCommand: strings.TrimSpace(cfg.Runtime.Sandbox.LiteBox.BridgeCommand),
			BridgeArgs:    append([]string(nil), cfg.Runtime.Sandbox.LiteBox.BridgeArgs...),
			RunnerPath:    strings.TrimSpace(cfg.Runtime.Sandbox.LiteBox.RunnerPath),
			RunnerArgs:    append([]string(nil), cfg.Runtime.Sandbox.LiteBox.RunnerArgs...),
		}
	default:
		slog.Warn("sandbox: unknown provider, fallback to home_dir", "provider", cfg.Runtime.Sandbox.Provider)
		return homeSandbox
	}
}
