package plugin_test

import (
	"errors"
	"testing"

	"github.com/immortal-engine/immortal/pkg/plugin"
)

// -------------------------------------------------------------------------
// Fakes
// -------------------------------------------------------------------------

type fakeEffectModel struct{ name string }

func (f *fakeEffectModel) Name() string { return f.name }
func (f *fakeEffectModel) Apply(s plugin.State, _ map[string]string) (plugin.State, bool) {
	s.CPU = 1.0
	return s, true
}

type fakeHealingAction struct{ name string }

func (f *fakeHealingAction) Name() string              { return f.name }
func (f *fakeHealingAction) Match(_ plugin.EventCtx) bool  { return true }
func (f *fakeHealingAction) Execute(_ plugin.EventCtx) error { return nil }

type fakeTool struct{ name string }

func (f *fakeTool) Name() string                               { return f.name }
func (f *fakeTool) Description() string                        { return "desc" }
func (f *fakeTool) Schema() map[string]string                  { return map[string]string{"arg": "string"} }
func (f *fakeTool) Invoke(_ map[string]any) (string, error)    { return "ok", nil }
func (f *fakeTool) CostTier() plugin.CostTier                  { return plugin.CostRead }

type fakeInvariant struct{ name string }

func (f *fakeInvariant) Name() string        { return f.name }
func (f *fakeInvariant) Description() string { return "desc" }
func (f *fakeInvariant) Holds(_ map[string]plugin.ServiceState) bool { return true }

type fakePlugin struct {
	meta   plugin.Meta
	models []plugin.EffectModel
	invs   []plugin.Invariant
}

func (p *fakePlugin) PluginMeta() plugin.Meta { return p.meta }
func (p *fakePlugin) Register(r *plugin.Registry) {
	for _, m := range p.models {
		_ = r.RegisterEffectModel(m)
	}
	for _, i := range p.invs {
		_ = r.RegisterInvariant(i)
	}
}

// -------------------------------------------------------------------------
// EffectModel tests
// -------------------------------------------------------------------------

func TestRegistry_RegisterEffectModel_Succeeds(t *testing.T) {
	r := plugin.NewRegistry()
	if err := r.RegisterEffectModel(&fakeEffectModel{name: "restart_v2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	models := r.EffectModels()
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].Name() != "restart_v2" {
		t.Errorf("expected name restart_v2, got %s", models[0].Name())
	}
}

func TestRegistry_RegisterEffectModel_DuplicateName_Errors(t *testing.T) {
	r := plugin.NewRegistry()
	_ = r.RegisterEffectModel(&fakeEffectModel{name: "dup"})
	err := r.RegisterEffectModel(&fakeEffectModel{name: "dup"})
	if !errors.Is(err, plugin.ErrDuplicateName) {
		t.Fatalf("expected ErrDuplicateName, got %v", err)
	}
}

func TestRegistry_RegisterEffectModel_EmptyName_Errors(t *testing.T) {
	r := plugin.NewRegistry()
	err := r.RegisterEffectModel(&fakeEffectModel{name: ""})
	if !errors.Is(err, plugin.ErrEmptyName) {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}
}

func TestRegistry_RegisterEffectModel_Nil_Errors(t *testing.T) {
	r := plugin.NewRegistry()
	err := r.RegisterEffectModel(nil)
	if !errors.Is(err, plugin.ErrNilContribution) {
		t.Fatalf("expected ErrNilContribution, got %v", err)
	}
}

// -------------------------------------------------------------------------
// HealingAction tests
// -------------------------------------------------------------------------

func TestRegistry_RegisterHealingAction(t *testing.T) {
	r := plugin.NewRegistry()

	// success
	if err := r.RegisterHealingAction(&fakeHealingAction{name: "my_rule"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// duplicate
	if err := r.RegisterHealingAction(&fakeHealingAction{name: "my_rule"}); !errors.Is(err, plugin.ErrDuplicateName) {
		t.Fatalf("expected ErrDuplicateName, got %v", err)
	}
	// empty name
	if err := r.RegisterHealingAction(&fakeHealingAction{name: ""}); !errors.Is(err, plugin.ErrEmptyName) {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}
	// nil
	if err := r.RegisterHealingAction(nil); !errors.Is(err, plugin.ErrNilContribution) {
		t.Fatalf("expected ErrNilContribution, got %v", err)
	}

	if len(r.HealingActions()) != 1 {
		t.Errorf("expected 1 healing action, got %d", len(r.HealingActions()))
	}
}

// -------------------------------------------------------------------------
// Tool tests
// -------------------------------------------------------------------------

func TestRegistry_RegisterTool(t *testing.T) {
	r := plugin.NewRegistry()

	if err := r.RegisterTool(&fakeTool{name: "my_tool"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := r.RegisterTool(&fakeTool{name: "my_tool"}); !errors.Is(err, plugin.ErrDuplicateName) {
		t.Fatalf("expected ErrDuplicateName, got %v", err)
	}
	if err := r.RegisterTool(&fakeTool{name: ""}); !errors.Is(err, plugin.ErrEmptyName) {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}
	if err := r.RegisterTool(nil); !errors.Is(err, plugin.ErrNilContribution) {
		t.Fatalf("expected ErrNilContribution, got %v", err)
	}

	if len(r.Tools()) != 1 {
		t.Errorf("expected 1 tool, got %d", len(r.Tools()))
	}
}

// -------------------------------------------------------------------------
// Invariant tests
// -------------------------------------------------------------------------

func TestRegistry_RegisterInvariant(t *testing.T) {
	r := plugin.NewRegistry()

	if err := r.RegisterInvariant(&fakeInvariant{name: "no_split_brain"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := r.RegisterInvariant(&fakeInvariant{name: "no_split_brain"}); !errors.Is(err, plugin.ErrDuplicateName) {
		t.Fatalf("expected ErrDuplicateName, got %v", err)
	}
	if err := r.RegisterInvariant(&fakeInvariant{name: ""}); !errors.Is(err, plugin.ErrEmptyName) {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}
	if err := r.RegisterInvariant(nil); !errors.Is(err, plugin.ErrNilContribution) {
		t.Fatalf("expected ErrNilContribution, got %v", err)
	}

	if len(r.Invariants()) != 1 {
		t.Errorf("expected 1 invariant, got %d", len(r.Invariants()))
	}
}

// -------------------------------------------------------------------------
// Plugin registration
// -------------------------------------------------------------------------

func TestRegistry_Plugins_ReturnsMeta(t *testing.T) {
	r := plugin.NewRegistry()
	p := &fakePlugin{
		meta: plugin.Meta{
			Name:    "acme-plugin",
			Version: "1.0.0",
			Author:  "Acme Corp",
		},
		models: []plugin.EffectModel{&fakeEffectModel{name: "acme_effect"}},
	}
	if err := r.RegisterPlugin(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	metas := r.Plugins()
	if len(metas) != 1 {
		t.Fatalf("expected 1 meta, got %d", len(metas))
	}
	if metas[0].Name != "acme-plugin" {
		t.Errorf("expected acme-plugin, got %s", metas[0].Name)
	}
}

// -------------------------------------------------------------------------
// EffectModels listing
// -------------------------------------------------------------------------

func TestRegistry_EffectModels_ReturnsAll(t *testing.T) {
	r := plugin.NewRegistry()
	names := []string{"alpha", "beta", "gamma"}
	for _, n := range names {
		if err := r.RegisterEffectModel(&fakeEffectModel{name: n}); err != nil {
			t.Fatalf("register %s: %v", n, err)
		}
	}
	models := r.EffectModels()
	if len(models) != len(names) {
		t.Fatalf("expected %d models, got %d", len(names), len(models))
	}
	got := make(map[string]bool)
	for _, m := range models {
		got[m.Name()] = true
	}
	for _, n := range names {
		if !got[n] {
			t.Errorf("missing model %s", n)
		}
	}
}
