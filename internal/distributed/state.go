package distributed

import (
	"encoding/json"
	"sync"
	"time"
)

type EntryType string

const (
	EntryHealing  EntryType = "healing"
	EntryLock     EntryType = "lock"
	EntryConfig   EntryType = "config"
	EntryBaseline EntryType = "baseline"
)

type Entry struct {
	Key       string        `json:"key"`
	Value     []byte        `json:"value"`
	Type      EntryType     `json:"type"`
	NodeID    string        `json:"node_id"`
	Timestamp time.Time     `json:"timestamp"`
	Version   int64         `json:"version"`
	TTL       time.Duration `json:"ttl,omitempty"`
}

type StateStore struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	version int64
}

func NewStateStore() *StateStore {
	return &StateStore{entries: make(map[string]*Entry)}
}

func (s *StateStore) Put(key string, value interface{}, entryType EntryType, nodeID string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++
	s.entries[key] = &Entry{
		Key:       key,
		Value:     data,
		Type:      entryType,
		NodeID:    nodeID,
		Timestamp: time.Now(),
		Version:   s.version,
	}
	return nil
}

func (s *StateStore) Get(key string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[key]
	if !ok {
		return nil, false
	}
	copy := *e
	return &copy, true
}

func (s *StateStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, key)
}

func (s *StateStore) All() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Entry
	for _, e := range s.entries {
		result = append(result, *e)
	}
	return result
}

func (s *StateStore) ByType(entryType EntryType) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Entry
	for _, e := range s.entries {
		if e.Type == entryType {
			result = append(result, *e)
		}
	}
	return result
}

// TryLock attempts to acquire a distributed lock. Returns true if acquired.
func (s *StateStore) TryLock(key string, nodeID string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.entries["lock:"+key]
	if ok && time.Since(existing.Timestamp) < existing.TTL {
		return existing.NodeID == nodeID // already holds lock
	}

	s.version++
	s.entries["lock:"+key] = &Entry{
		Key:       "lock:" + key,
		Value:     []byte(nodeID),
		Type:      EntryLock,
		NodeID:    nodeID,
		Timestamp: time.Now(),
		Version:   s.version,
		TTL:       ttl,
	}
	return true
}

func (s *StateStore) Unlock(key string, nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.entries["lock:"+key]
	if ok && existing.NodeID == nodeID {
		delete(s.entries, "lock:"+key)
	}
}

func (s *StateStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

func (s *StateStore) CurrentVersion() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}
