package antiscrape_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/security/antiscrape"
)

func TestShieldDetectsBotUserAgent(t *testing.T) {
	s := antiscrape.NewDefault()

	botUAs := []string{
		"Mozilla/5.0 (compatible; Googlebot/2.1)",
		"python-requests/2.28.0",
		"Go-http-client/1.1",
		"curl/7.88.1",
		"Wget/1.21",
		"Selenium/4.0",
	}

	for _, ua := range botUAs {
		if !s.IsBot("1.2.3.4", ua, "/page") {
			t.Errorf("should detect bot UA: %s", ua)
		}
	}
}

func TestShieldAllowsLegitBrowsers(t *testing.T) {
	s := antiscrape.NewDefault()

	legitimate := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605.1.15",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Mobile/15E148",
	}

	for _, ua := range legitimate {
		if s.IsBot("5.6.7.8", ua, "/page") {
			t.Errorf("should allow legitimate browser: %s", ua)
		}
	}
}

func TestShieldDetectsScraping(t *testing.T) {
	s := antiscrape.New(antiscrape.Config{MaxUniquePaths: 5})

	// Normal browser user agent
	ua := "Mozilla/5.0 Chrome/120.0"

	// But hits many unique paths rapidly
	for i := 0; i < 10; i++ {
		s.Analyze("1.2.3.4", ua, fmt.Sprintf("/page/%d", i))
	}

	profile := s.GetProfile("1.2.3.4")
	if profile == nil {
		t.Fatal("expected profile")
	}
	if !profile.IsBot {
		t.Error("should detect scraper hitting too many unique paths")
	}
}

func TestShieldMiddleware(t *testing.T) {
	s := antiscrape.NewDefault()

	handler := s.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))

	// Legitimate request
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("legitimate request should pass, got %d", rec.Code)
	}

	// Bot request
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "python-requests/2.28")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Errorf("bot should be blocked, got %d", rec.Code)
	}
}

func TestShieldBotCount(t *testing.T) {
	s := antiscrape.NewDefault()

	s.Analyze("1.1.1.1", "curl/7.88", "/")
	s.Analyze("2.2.2.2", "Mozilla/5.0 Chrome", "/")
	s.Analyze("3.3.3.3", "python-requests/2.28", "/")

	if s.BotCount() != 2 {
		t.Errorf("expected 2 bots, got %d", s.BotCount())
	}
}

func TestShieldPerIPTracking(t *testing.T) {
	s := antiscrape.NewDefault()

	s.Analyze("1.1.1.1", "Mozilla/5.0", "/page1")
	s.Analyze("1.1.1.1", "Mozilla/5.0", "/page2")
	s.Analyze("2.2.2.2", "Mozilla/5.0", "/page1")

	p1 := s.GetProfile("1.1.1.1")
	p2 := s.GetProfile("2.2.2.2")

	if p1.RequestCount != 2 {
		t.Errorf("IP1 should have 2 requests, got %d", p1.RequestCount)
	}
	if p2.RequestCount != 1 {
		t.Errorf("IP2 should have 1 request, got %d", p2.RequestCount)
	}
}
