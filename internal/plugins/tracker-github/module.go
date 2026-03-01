package trackergithub

import (
	"github.com/user/ai-workflow/internal/config"
	"github.com/user/ai-workflow/internal/core"
)

func Module() core.PluginModule {
	return core.PluginModule{
		Name: "tracker-github",
		Slot: core.SlotTracker,
		Factory: func(cfg map[string]any) (core.Plugin, error) {
			if cfg != nil {
				if githubCfg, ok := cfg["github"].(config.GitHubConfig); ok {
					return NewWithGitHubConfig(githubCfg), nil
				}
			}
			return New(), nil
		},
	}
}
