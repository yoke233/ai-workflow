package core

import (
	"fmt"
	"sync"
)

// Registry stores plugin modules by slot/name.
type Registry struct {
	mu      sync.RWMutex
	modules map[PluginSlot]map[string]PluginModule
}

func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[PluginSlot]map[string]PluginModule),
	}
}

func (r *Registry) Register(module PluginModule) error {
	if module.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if module.Slot == "" {
		return fmt.Errorf("plugin slot is required")
	}
	if module.Factory == nil {
		return fmt.Errorf("plugin factory is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	slotModules, ok := r.modules[module.Slot]
	if !ok {
		slotModules = make(map[string]PluginModule)
		r.modules[module.Slot] = slotModules
	}
	if _, exists := slotModules[module.Name]; exists {
		return fmt.Errorf("plugin already registered: slot=%s name=%s", module.Slot, module.Name)
	}
	slotModules[module.Name] = module
	return nil
}

func (r *Registry) Get(slot PluginSlot, name string) (PluginModule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	slotModules, ok := r.modules[slot]
	if !ok {
		return PluginModule{}, false
	}
	module, ok := slotModules[name]
	return module, ok
}
