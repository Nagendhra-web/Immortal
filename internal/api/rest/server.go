package rest

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/healing"
	"github.com/immortal-engine/immortal/internal/health"
	"github.com/immortal-engine/immortal/internal/storage"
	"github.com/immortal-engine/immortal/internal/stream"
)

type Server struct {
	store      *storage.Store
	registry   *health.Registry
	healer     *healing.Healer
	liveStream *stream.Stream
	mux        *http.ServeMux
}

func New(store *storage.Store, registry *health.Registry, healer *healing.Healer) *Server {
	s := &Server{
		store:    store,
		registry: registry,
		healer:   healer,
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s
}

func NewWithStream(store *storage.Store, registry *health.Registry, healer *healing.Healer, ls *stream.Stream) *Server {
	s := &Server{
		store:      store,
		registry:   registry,
		healer:     healer,
		liveStream: ls,
		mux:        http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/events", s.handleEvents)
	s.mux.HandleFunc("/api/healing/history", s.handleHealingHistory)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/logs/stream", s.handleLogStream)
	s.mux.HandleFunc("/api/logs/history", s.handleLogHistory)
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	services := s.registry.All()
	overall := s.registry.OverallStatus()

	resp := map[string]interface{}{
		"status":   overall,
		"services": services,
	}
	writeJSON(w, resp)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := storage.Query{Limit: 100}
	if typ := r.URL.Query().Get("type"); typ != "" {
		q.Type = event.Type(typ)
	}
	if src := r.URL.Query().Get("source"); src != "" {
		q.Source = src
	}

	events, err := s.store.Query(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, events)
}

func (s *Server) handleHealingHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.healer.History())
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{
		"engine":  "running",
		"version": "0.1.0",
	})
}

// handleLogStream serves Server-Sent Events (SSE) for live log streaming.
func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request) {
	if s.liveStream == nil {
		http.Error(w, "live stream not available", http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sub, cleanup := s.liveStream.Subscribe(fmt.Sprintf("sse-%s", r.RemoteAddr), 50, nil)
	defer cleanup()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-sub.Ch:
			if !ok {
				return
			}
			data := stream.FormatJSON(entry)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// handleLogHistory returns recent log entries.
func (s *Server) handleLogHistory(w http.ResponseWriter, r *http.Request) {
	if s.liveStream == nil {
		writeJSON(w, []stream.LogEntry{})
		return
	}
	entries := s.liveStream.History(100)
	writeJSON(w, entries)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
