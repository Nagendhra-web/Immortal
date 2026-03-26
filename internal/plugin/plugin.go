package plugin

import (
	"fmt"
	"sync"
)

type Plugin interface {
	Name() string
	Version() string
	Init() error
	Start() error
	Stop() error
}

type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	order   []string
}

func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]Plugin)}
}

func (r *Registry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.plugins[p.Name()]; exists {
		return fmt.Errorf("plugin '%s' already registered", p.Name())
	}
	r.plugins[p.Name()] = p
	r.order = append(r.order, p.Name())
	return nil
}

func (r *Registry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

func (r *Registry) All() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Plugin
	for _, name := range r.order {
		result = append(result, r.plugins[name])
	}
	return result
}

func (r *Registry) InitAll() error {
	for _, name := range r.order {
		if err := r.plugins[name].Init(); err != nil {
			return fmt.Errorf("plugin '%s' init failed: %w", name, err)
		}
	}
	return nil
}

func (r *Registry) StartAll() error {
	for _, name := range r.order {
		if err := r.plugins[name].Start(); err != nil {
			return fmt.Errorf("plugin '%s' start failed: %w", name, err)
		}
	}
	return nil
}

func (r *Registry) StopAll() error {
	for i := len(r.order) - 1; i >= 0; i-- {
		name := r.order[i]
		if err := r.plugins[name].Stop(); err != nil {
			return fmt.Errorf("plugin '%s' stop failed: %w", name, err)
		}
	}
	return nil
}

func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.plugins)
}
