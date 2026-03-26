package rollback

import (
	"fmt"
	"sync"
	"time"
)

type Action struct {
	ID        string
	Name      string
	Timestamp time.Time
	Undo      func() error
}

type Manager struct {
	mu      sync.Mutex
	actions []Action
	maxSize int
}

func New(maxSize int) *Manager {
	return &Manager{maxSize: maxSize}
}

func (m *Manager) Record(name string, undo func() error) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fmt.Sprintf("rb_%d", time.Now().UnixNano())
	a := Action{ID: id, Name: name, Timestamp: time.Now(), Undo: undo}
	m.actions = append(m.actions, a)
	if len(m.actions) > m.maxSize {
		m.actions = m.actions[1:]
	}
	return id
}

func (m *Manager) Rollback(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.actions) - 1; i >= 0; i-- {
		if m.actions[i].ID == id {
			err := m.actions[i].Undo()
			m.actions = append(m.actions[:i], m.actions[i+1:]...)
			return err
		}
	}
	return fmt.Errorf("action '%s' not found", id)
}

func (m *Manager) RollbackLast() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.actions) == 0 {
		return fmt.Errorf("no actions to rollback")
	}
	last := m.actions[len(m.actions)-1]
	err := last.Undo()
	m.actions = m.actions[:len(m.actions)-1]
	return err
}

func (m *Manager) History() []Action {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Action, len(m.actions))
	copy(out, m.actions)
	return out
}

func (m *Manager) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.actions)
}
