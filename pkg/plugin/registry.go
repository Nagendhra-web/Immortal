package plugin

import "sync"

// Registry collects all plugin contributions. It is safe for concurrent use.
// Create one with NewRegistry, then pass it to Plugin.Register or call the
// individual Register* methods directly.
type Registry struct {
	mu             sync.RWMutex
	plugins        []Meta
	effectModels   []EffectModel
	healingActions []HealingAction
	tools          []Tool
	invariants     []Invariant

	// name-dedup sets
	effectModelNames   map[string]struct{}
	healingActionNames map[string]struct{}
	toolNames          map[string]struct{}
	invariantNames     map[string]struct{}
}

// NewRegistry allocates an empty, ready-to-use Registry.
func NewRegistry() *Registry {
	return &Registry{
		effectModelNames:   make(map[string]struct{}),
		healingActionNames: make(map[string]struct{}),
		toolNames:          make(map[string]struct{}),
		invariantNames:     make(map[string]struct{}),
	}
}

// RegisterPlugin calls p.Register(r) and stores p.PluginMeta() in the registry.
// It returns the first error produced by any contribution registered inside
// Plugin.Register, propagated via the individual Register* methods.
func (r *Registry) RegisterPlugin(p Plugin) error {
	if p == nil {
		return ErrNilContribution
	}
	// Use a sentinel sub-registry to catch errors from Register.
	sub := NewRegistry()
	p.Register(sub)

	// Merge sub into r, checking for duplicates against r's existing sets.
	for _, e := range sub.effectModels {
		if err := r.RegisterEffectModel(e); err != nil {
			return err
		}
	}
	for _, a := range sub.healingActions {
		if err := r.RegisterHealingAction(a); err != nil {
			return err
		}
	}
	for _, t := range sub.tools {
		if err := r.RegisterTool(t); err != nil {
			return err
		}
	}
	for _, i := range sub.invariants {
		if err := r.RegisterInvariant(i); err != nil {
			return err
		}
	}

	r.mu.Lock()
	r.plugins = append(r.plugins, p.PluginMeta())
	r.mu.Unlock()
	return nil
}

// RegisterEffectModel adds an EffectModel to the registry.
// Returns ErrNilContribution, ErrEmptyName, or ErrDuplicateName on error.
func (r *Registry) RegisterEffectModel(e EffectModel) error {
	if e == nil {
		return ErrNilContribution
	}
	name := e.Name()
	if name == "" {
		return ErrEmptyName
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.effectModelNames[name]; exists {
		return ErrDuplicateName
	}
	r.effectModelNames[name] = struct{}{}
	r.effectModels = append(r.effectModels, e)
	return nil
}

// RegisterHealingAction adds a HealingAction to the registry.
// Returns ErrNilContribution, ErrEmptyName, or ErrDuplicateName on error.
func (r *Registry) RegisterHealingAction(a HealingAction) error {
	if a == nil {
		return ErrNilContribution
	}
	name := a.Name()
	if name == "" {
		return ErrEmptyName
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.healingActionNames[name]; exists {
		return ErrDuplicateName
	}
	r.healingActionNames[name] = struct{}{}
	r.healingActions = append(r.healingActions, a)
	return nil
}

// RegisterTool adds a Tool to the registry.
// Returns ErrNilContribution, ErrEmptyName, or ErrDuplicateName on error.
func (r *Registry) RegisterTool(t Tool) error {
	if t == nil {
		return ErrNilContribution
	}
	name := t.Name()
	if name == "" {
		return ErrEmptyName
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.toolNames[name]; exists {
		return ErrDuplicateName
	}
	r.toolNames[name] = struct{}{}
	r.tools = append(r.tools, t)
	return nil
}

// RegisterInvariant adds an Invariant to the registry.
// Returns ErrNilContribution, ErrEmptyName, or ErrDuplicateName on error.
func (r *Registry) RegisterInvariant(i Invariant) error {
	if i == nil {
		return ErrNilContribution
	}
	name := i.Name()
	if name == "" {
		return ErrEmptyName
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.invariantNames[name]; exists {
		return ErrDuplicateName
	}
	r.invariantNames[name] = struct{}{}
	r.invariants = append(r.invariants, i)
	return nil
}

// Plugins returns the metadata for every plugin registered via RegisterPlugin.
func (r *Registry) Plugins() []Meta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Meta, len(r.plugins))
	copy(out, r.plugins)
	return out
}

// EffectModels returns all registered EffectModel contributions.
func (r *Registry) EffectModels() []EffectModel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]EffectModel, len(r.effectModels))
	copy(out, r.effectModels)
	return out
}

// HealingActions returns all registered HealingAction contributions.
func (r *Registry) HealingActions() []HealingAction {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]HealingAction, len(r.healingActions))
	copy(out, r.healingActions)
	return out
}

// Tools returns all registered Tool contributions.
func (r *Registry) Tools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, len(r.tools))
	copy(out, r.tools)
	return out
}

// Invariants returns all registered Invariant contributions.
func (r *Registry) Invariants() []Invariant {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Invariant, len(r.invariants))
	copy(out, r.invariants)
	return out
}
