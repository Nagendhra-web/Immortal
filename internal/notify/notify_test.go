package notify_test

import (
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/notify"
)

func TestDispatcherSendsToAll(t *testing.T) {
	d := notify.NewDispatcher()
	var received []string
	d.AddChannel(&notify.CallbackChannel{Fn: func(title, msg, lvl string) { received = append(received, "ch1:"+title) }})
	d.AddChannel(&notify.CallbackChannel{Fn: func(title, msg, lvl string) { received = append(received, "ch2:"+title) }})
	d.Send("Alert", "Server down", "critical")
	if len(received) != 2 {
		t.Errorf("expected 2 notifications, got %d", len(received))
	}
}

func TestDispatcherHistory(t *testing.T) {
	d := notify.NewDispatcher()
	d.AddChannel(&notify.ConsoleChannel{})
	d.Send("Test1", "msg1", "info")
	d.Send("Test2", "msg2", "error")
	if len(d.History()) != 2 {
		t.Errorf("expected 2 history, got %d", len(d.History()))
	}
}

func TestConsoleChannel(t *testing.T) {
	ch := &notify.ConsoleChannel{}
	if err := ch.Send("Test", "message", "info"); err != nil {
		t.Fatal(err)
	}
	if ch.Name() != "console" {
		t.Error("wrong name")
	}
}

func TestCallbackChannel(t *testing.T) {
	var called bool
	ch := &notify.CallbackChannel{Fn: func(t, m, l string) { called = true }}
	ch.Send("t", "m", "l")
	if !called {
		t.Error("callback not called")
	}
}
