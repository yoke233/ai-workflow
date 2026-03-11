package bootstrap

import (
	"github.com/yoke233/ai-workflow/internal/adapters/sandbox"
	"github.com/yoke233/ai-workflow/internal/platform/config"
	"github.com/yoke233/ai-workflow/internal/platform/configruntime"
)

func buildSandbox(cfg *config.Config, runtimeManager *configruntime.Manager, dataDir string) sandbox.Sandbox {
	fallback := config.RuntimeSandboxConfig{}
	if cfg != nil {
		fallback = cfg.Runtime.Sandbox
	}
	if runtimeManager != nil {
		return sandbox.NewRuntimeSandbox(runtimeManager, fallback, dataDir)
	}
	return sandbox.FromRuntimeConfig(fallback, dataDir)
}
