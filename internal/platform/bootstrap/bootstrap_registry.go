package bootstrap

import (
	"context"
	"log/slog"

	"github.com/yoke233/zhanggui/internal/adapters/agent/acpclient"
	"github.com/yoke233/zhanggui/internal/adapters/store/sqlite"
	"github.com/yoke233/zhanggui/internal/core"
	"github.com/yoke233/zhanggui/internal/platform/config"
	"github.com/yoke233/zhanggui/internal/platform/configruntime"
)

// seedRegistry seeds agent profiles into the SQLite store from TOML config.
// Uses upsert so TOML always acts as the source of truth for configured agents,
// while runtime additions via API are also persisted.
func seedRegistry(ctx context.Context, store *sqlite.Store, cfg *config.Config, _ *acpclient.RoleResolver) {
	if cfg == nil || store == nil {
		return
	}

	currentProfiles, err := store.ListProfiles(ctx)
	if err != nil {
		slog.Warn("registry: list profiles before bootstrap failed", "error", err)
		return
	}
	if len(currentProfiles) > 0 {
		slog.Info("registry: bootstrap skipped because store already has profiles", "profiles", len(currentProfiles))
		return
	}

	profiles := bootstrapProfiles(configruntime.BuildAgents(cfg))
	if len(profiles) == 0 {
		slog.Warn("registry: no agent config to seed")
		return
	}

	for _, p := range profiles {
		if err := store.UpsertProfile(ctx, p); err != nil {
			slog.Warn("registry: seed profile failed", "id", p.ID, "error", err)
		}
	}
	slog.Info("registry: seeded from config", "profiles", len(profiles))
}

func bootstrapProfiles(profiles []*core.AgentProfile) []*core.AgentProfile {
	if len(profiles) == 0 {
		return nil
	}
	profilesByID := make(map[string]*core.AgentProfile, len(profiles))
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		profilesByID[profile.ID] = profile
	}
	if ceo := profilesByID["ceo"]; ceo != nil {
		selected := map[string]*core.AgentProfile{
			ceo.ID: ceo,
		}

		leadIDs := make([]string, 0, len(profilesByID))
		for _, profile := range profiles {
			if profile == nil || profile.ID == ceo.ID {
				continue
			}
			if profile.ManagerProfileID != ceo.ID {
				continue
			}
			switch profile.Role {
			case core.RoleLead, core.RoleWorker, core.RoleGate:
				selected[profile.ID] = profile
				if profile.Role == core.RoleLead {
					leadIDs = append(leadIDs, profile.ID)
				}
			}
		}

		for _, leadID := range leadIDs {
			for _, profile := range profiles {
				if profile == nil || profile.ID == ceo.ID {
					continue
				}
				if profile.ManagerProfileID != leadID {
					continue
				}
				switch profile.Role {
				case core.RoleWorker, core.RoleGate:
					selected[profile.ID] = profile
				}
			}
		}

		out := make([]*core.AgentProfile, 0, len(selected))
		for _, profile := range profiles {
			if profile == nil {
				continue
			}
			if selected[profile.ID] == nil {
				continue
			}
			out = append(out, profile)
		}
		return out
	}
	for _, profile := range profiles {
		if profile != nil {
			slog.Warn("registry: ceo profile missing from bootstrap config; seeding first available profile", "id", profile.ID)
			return []*core.AgentProfile{profile}
		}
	}
	return nil
}

func SeedRegistry(ctx context.Context, store *sqlite.Store, cfg *config.Config) {
	seedRegistry(ctx, store, cfg, nil)
}
