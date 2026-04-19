package connector_test

import (
	"sync"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/connector"
	"github.com/Nagendhra-web/Immortal/internal/event"
)

func TestProcessConnectorDetectsRunningProcess(t *testing.T) {
	var received []*event.Event
	var mu sync.Mutex

	pc := connector.NewProcessConnector(connector.ProcessConfig{
		Name:     "go",
		Interval: 100 * time.Millisecond,
		Callback: func(e *event.Event) {
			mu.Lock()
			received = append(received, e)
			mu.Unlock()
		},
	})

	pc.Start()
	defer pc.Stop()

	time.Sleep(350 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Error("expected health check events for running process")
	}
}
