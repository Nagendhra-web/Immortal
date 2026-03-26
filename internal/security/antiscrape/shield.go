package antiscrape

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

type ClientProfile struct {
	IP           string
	RequestCount int
	FirstSeen    time.Time
	LastSeen     time.Time
	Paths        map[string]int
	IsBot        bool
	BotReason    string
}

type Shield struct {
	mu      sync.RWMutex
	clients map[string]*ClientProfile
	config  Config
}

type Config struct {
	MaxRequestsPerMinute int
	MaxUniquePaths       int
	BotUserAgents        []string
}

func New(config Config) *Shield {
	if config.MaxRequestsPerMinute == 0 {
		config.MaxRequestsPerMinute = 60
	}
	if config.MaxUniquePaths == 0 {
		config.MaxUniquePaths = 50
	}
	if len(config.BotUserAgents) == 0 {
		config.BotUserAgents = []string{
			"bot", "crawler", "spider", "scraper", "curl", "wget",
			"python-requests", "go-http-client", "java/", "httpclient",
			"headlesschrome", "phantomjs", "selenium",
		}
	}
	return &Shield{
		clients: make(map[string]*ClientProfile),
		config:  config,
	}
}

func NewDefault() *Shield {
	return New(Config{})
}

func (s *Shield) Analyze(ip string, userAgent string, path string) *ClientProfile {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	client, exists := s.clients[ip]
	if !exists {
		client = &ClientProfile{
			IP:        ip,
			FirstSeen: now,
			Paths:     make(map[string]int),
		}
		s.clients[ip] = client
	}

	client.LastSeen = now
	client.RequestCount++
	client.Paths[path]++

	// Check bot user agent
	lowerUA := strings.ToLower(userAgent)
	for _, bot := range s.config.BotUserAgents {
		if strings.Contains(lowerUA, bot) {
			client.IsBot = true
			client.BotReason = "bot user agent: " + bot
			return client
		}
	}

	// Check request rate
	elapsed := now.Sub(client.FirstSeen)
	if elapsed > 0 && elapsed < time.Minute {
		rate := float64(client.RequestCount) / elapsed.Minutes()
		if rate > float64(s.config.MaxRequestsPerMinute) {
			client.IsBot = true
			client.BotReason = "excessive request rate"
			return client
		}
	}

	// Check unique paths (scrapers hit many different pages)
	if len(client.Paths) > s.config.MaxUniquePaths {
		client.IsBot = true
		client.BotReason = "too many unique paths"
		return client
	}

	return client
}

func (s *Shield) IsBot(ip string, userAgent string, path string) bool {
	profile := s.Analyze(ip, userAgent, path)
	return profile.IsBot
}

func (s *Shield) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		ua := r.UserAgent()
		path := r.URL.Path

		if s.IsBot(ip, ua, path) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Shield) GetProfile(ip string) *ClientProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.clients[ip]; ok {
		cp := *p
		cp.Paths = make(map[string]int, len(p.Paths))
		for k, v := range p.Paths {
			cp.Paths[k] = v
		}
		return &cp
	}
	return nil
}

func (s *Shield) BotCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, c := range s.clients {
		if c.IsBot {
			count++
		}
	}
	return count
}
