package plugin_test

import (
	"testing"

	"github.com/immortal-engine/immortal/internal/plugin"
)

type testPlugin struct {
	name    string
	started bool
	stopped bool
	inited  bool
}

func (p *testPlugin) Name() string    { return p.name }
func (p *testPlugin) Version() string { return "1.0" }
func (p *testPlugin) Init() error     { p.inited = true; return nil }
func (p *testPlugin) Start() error    { p.started = true; return nil }
func (p *testPlugin) Stop() error     { p.stopped = true; return nil }

func TestRegisterAndGet(t *testing.T) {
	r := plugin.NewRegistry()
	p := &testPlugin{name: "test"}
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	got, ok := r.Get("test")
	if !ok {
		t.Fatal("expected plugin")
	}
	if got.Name() != "test" {
		t.Error("wrong name")
	}
}

func TestDuplicateRegister(t *testing.T) {
	r := plugin.NewRegistry()
	r.Register(&testPlugin{name: "dup"})
	err := r.Register(&testPlugin{name: "dup"})
	if err == nil {
		t.Error("duplicate should fail")
	}
}

func TestInitStartStopAll(t *testing.T) {
	r := plugin.NewRegistry()
	p1 := &testPlugin{name: "p1"}
	p2 := &testPlugin{name: "p2"}
	r.Register(p1)
	r.Register(p2)

	r.InitAll()
	if !p1.inited || !p2.inited {
		t.Error("all should be inited")
	}

	r.StartAll()
	if !p1.started || !p2.started {
		t.Error("all should be started")
	}

	r.StopAll()
	if !p1.stopped || !p2.stopped {
		t.Error("all should be stopped")
	}
}

func TestCount(t *testing.T) {
	r := plugin.NewRegistry()
	if r.Count() != 0 {
		t.Error("expected 0")
	}
	r.Register(&testPlugin{name: "a"})
	r.Register(&testPlugin{name: "b"})
	if r.Count() != 2 {
		t.Error("expected 2")
	}
}

func TestAll(t *testing.T) {
	r := plugin.NewRegistry()
	r.Register(&testPlugin{name: "first"})
	r.Register(&testPlugin{name: "second"})
	all := r.All()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
	if all[0].Name() != "first" {
		t.Error("order should be preserved")
	}
}
