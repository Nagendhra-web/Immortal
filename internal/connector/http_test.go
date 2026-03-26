package connector_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/connector"
	"github.com/immortal-engine/immortal/internal/event"
)

func TestHTTPConnectorHealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	var received []*event.Event
	var mu sync.Mutex

	hc := connector.NewHTTPConnector(connector.HTTPConfig{
		URL:      server.URL,
		Interval: 100 * time.Millisecond,
		Callback: func(e *event.Event) {
			mu.Lock()
			received = append(received, e)
			mu.Unlock()
		},
	})

	hc.Start()
	defer hc.Stop()
	time.Sleep(250 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Fatal("expected health check events")
	}
	if received[0].Severity != event.SeverityInfo {
		t.Errorf("expected info severity for healthy endpoint, got %s", received[0].Severity)
	}
}

func TestHTTPConnectorUnhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	var received []*event.Event
	var mu sync.Mutex

	hc := connector.NewHTTPConnector(connector.HTTPConfig{
		URL:      server.URL,
		Interval: 100 * time.Millisecond,
		Callback: func(e *event.Event) {
			mu.Lock()
			received = append(received, e)
			mu.Unlock()
		},
	})

	hc.Start()
	defer hc.Stop()
	time.Sleep(250 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Fatal("expected health check events")
	}
	if received[0].Severity != event.SeverityCritical {
		t.Errorf("expected critical severity for 500, got %s", received[0].Severity)
	}
}
