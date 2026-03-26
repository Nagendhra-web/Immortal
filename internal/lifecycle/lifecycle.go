package lifecycle

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type ShutdownFunc func(ctx context.Context) error

type Manager struct {
	mu      sync.Mutex
	hooks   []namedHook
	timeout time.Duration
}

type namedHook struct {
	name string
	fn   ShutdownFunc
}

func New(timeout time.Duration) *Manager {
	return &Manager{timeout: timeout}
}

func (m *Manager) OnShutdown(name string, fn ShutdownFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, namedHook{name: name, fn: fn})
}

func (m *Manager) WaitForSignal() os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	return <-sigCh
}

func (m *Manager) Shutdown() []error {
	m.mu.Lock()
	hooks := make([]namedHook, len(m.hooks))
	copy(hooks, m.hooks)
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	var errs []error
	for i := len(hooks) - 1; i >= 0; i-- {
		if err := hooks[i].fn(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (m *Manager) HookCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.hooks)
}
