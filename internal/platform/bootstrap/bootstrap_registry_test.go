package bootstrap

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/yoke233/zhanggui/internal/adapters/store/sqlite"
	"github.com/yoke233/zhanggui/internal/core"
	"github.com/yoke233/zhanggui/internal/platform/config"
)

func TestSeedRegistryMaterializesMinimumOrgOnEmptyStore(t *testing.T) {
	t.Parallel()

	store := newBootstrapRegistryTestStore(t)
	cfg := bootstrapRegistrySeedConfig()
	seedRegistry(context.Background(), store, cfg, nil)

	profiles, err := store.ListProfiles(context.Background())
	if err != nil {
		t.Fatalf("ListProfiles() error = %v", err)
	}
	if len(profiles) != 4 {
		t.Fatalf("profiles len = %d, want 4", len(profiles))
	}

	ids := make([]string, 0, len(profiles))
	managers := make(map[string]string, len(profiles))
	for _, profile := range profiles {
		ids = append(ids, profile.ID)
		managers[profile.ID] = profile.ManagerProfileID
	}
	slices.Sort(ids)
	if got, want := ids, []string{"ceo", "lead", "reviewer", "worker"}; !slices.Equal(got, want) {
		t.Fatalf("profile ids = %#v, want %#v", got, want)
	}
	if managers["ceo"] != "" {
		t.Fatalf("ceo manager = %q, want empty", managers["ceo"])
	}
	if managers["lead"] != "ceo" {
		t.Fatalf("lead manager = %q, want ceo", managers["lead"])
	}
	if managers["worker"] != "lead" {
		t.Fatalf("worker manager = %q, want lead", managers["worker"])
	}
	if managers["reviewer"] != "lead" {
		t.Fatalf("reviewer manager = %q, want lead", managers["reviewer"])
	}
}

func TestSeedRegistryDoesNotOverwriteExistingProfiles(t *testing.T) {
	t.Parallel()

	store := newBootstrapRegistryTestStore(t)
	if err := store.UpsertProfile(context.Background(), &core.AgentProfile{
		ID:          "custom",
		Name:        "Custom",
		DriverID:    "claude-acp",
		LLMConfigID: "system",
		Role:        core.RoleLead,
		Driver: core.DriverConfig{
			CapabilitiesMax: core.DriverCapabilities{FSRead: true, FSWrite: true, Terminal: true},
		},
	}); err != nil {
		t.Fatalf("UpsertProfile(custom) error = %v", err)
	}

	cfg := bootstrapRegistrySeedConfig()
	seedRegistry(context.Background(), store, cfg, nil)

	profiles, err := store.ListProfiles(context.Background())
	if err != nil {
		t.Fatalf("ListProfiles() error = %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("profiles len = %d, want 1", len(profiles))
	}
	if profiles[0].ID != "custom" {
		t.Fatalf("profiles[0].ID = %q, want custom", profiles[0].ID)
	}
}

func TestBootstrapProfilesFollowsConfiguredManagerChain(t *testing.T) {
	t.Parallel()

	profiles := bootstrapProfiles([]*core.AgentProfile{
		{ID: "ceo", Role: core.RoleLead},
		{ID: "director", Role: core.RoleLead, ManagerProfileID: "ceo"},
		{ID: "builder-a", Role: core.RoleWorker, ManagerProfileID: "director"},
		{ID: "gate-a", Role: core.RoleGate, ManagerProfileID: "director"},
		{ID: "support-a", Role: core.RoleSupport, ManagerProfileID: "director"},
	})

	if len(profiles) != 4 {
		t.Fatalf("profiles len = %d, want 4", len(profiles))
	}
	got := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		got = append(got, profile.ID)
	}
	want := []string{"ceo", "director", "builder-a", "gate-a"}
	if !slices.Equal(got, want) {
		t.Fatalf("profile ids = %#v, want %#v", got, want)
	}
}

func newBootstrapRegistryTestStore(t *testing.T) *sqlite.Store {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "bootstrap-registry-test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func bootstrapRegistrySeedConfig() *config.Config {
	return &config.Config{
		Runtime: config.RuntimeConfig{
			Agents: config.RuntimeAgentsConfig{
				Drivers: []config.RuntimeDriverConfig{{
					ID:            "codex-cli",
					LaunchCommand: "codex",
					CapabilitiesMax: config.CapabilitiesConfig{
						FSRead:   true,
						FSWrite:  true,
						Terminal: true,
					},
				}},
				Profiles: []config.RuntimeProfileConfig{{
					ID:          "ceo",
					Name:        "CEO Orchestrator",
					Driver:      "codex-cli",
					LLMConfigID: "system",
					Role:        string(core.RoleLead),
					Session: config.RuntimeSessionConfig{
						Reuse:    true,
						MaxTurns: 16,
						IdleTTL:  config.Duration{Duration: 30 * time.Minute},
					},
				}, {
					ID:               "lead",
					Name:             "Lead Agent",
					Driver:           "codex-cli",
					LLMConfigID:      "system",
					Role:             string(core.RoleLead),
					ManagerProfileID: "ceo",
				}, {
					ID:               "worker",
					Name:             "Worker Agent",
					Driver:           "codex-cli",
					LLMConfigID:      "system",
					Role:             string(core.RoleWorker),
					ManagerProfileID: "lead",
				}, {
					ID:               "reviewer",
					Name:             "Reviewer Agent",
					Driver:           "codex-cli",
					LLMConfigID:      "system",
					Role:             string(core.RoleGate),
					ManagerProfileID: "lead",
				}},
			},
		},
	}
}
