package core

import "context"

// PluginSlot identifies one of the core pluggable extension points.
type PluginSlot string

const (
	SlotWorkspace  PluginSlot = "workspace"
	SlotSpec       PluginSlot = "spec"
	SlotReviewGate PluginSlot = "review_gate"
	SlotTracker    PluginSlot = "tracker"
	SlotSCM        PluginSlot = "scm"
	SlotNotifier   PluginSlot = "notifier"
	SlotStore      PluginSlot = "store"
	SlotTerminal   PluginSlot = "terminal"
)

// Plugin is the common interface every pluggable component must satisfy.
type Plugin interface {
	Name() string
	Init(ctx context.Context) error
	Close() error
}

// SpecContextRequest describes input for fetching plan-level spec context.
type SpecContextRequest struct {
	ProjectID string `json:"project_id"`
	PlanID    string `json:"plan_id"`
	Query     string `json:"query"`
}

// SpecContext carries spec enrichment for plan/review stages.
type SpecContext struct {
	Summary    string   `json:"summary"`
	References []string `json:"references"`
}

// SpecPlugin provides plan-level spec context and lifecycle hooks.
type SpecPlugin interface {
	Plugin
	IsInitialized() bool
	GetContext(ctx context.Context, req SpecContextRequest) (SpecContext, error)
}

// PluginModule describes a registerable plugin implementation.
type PluginModule struct {
	Name    string
	Slot    PluginSlot
	Factory func(cfg map[string]any) (Plugin, error)
}
