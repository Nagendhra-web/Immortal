package firewall_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/immortal-engine/immortal/internal/security/firewall"
)

func TestFirewallBlocksSQLi(t *testing.T) {
	fw := firewall.New()

	attacks := []string{
		"' OR 1=1 --",
		"'; DROP TABLE users; --",
		"UNION SELECT * FROM passwords",
		"1' OR '1'='1",
	}

	for _, attack := range attacks {
		result := fw.Analyze(attack)
		if !result.Blocked {
			t.Errorf("SQLi not blocked: %s", attack)
		}
		if result.ThreatType != firewall.ThreatSQLi {
			t.Errorf("expected SQLi type, got %s for: %s", result.ThreatType, attack)
		}
	}
}

func TestFirewallBlocksXSS(t *testing.T) {
	fw := firewall.New()

	attacks := []string{
		"<script>alert('xss')</script>",
		"javascript:alert(1)",
		`<img onerror="alert(1)">`,
		"document.cookie",
		"<iframe src='evil.com'>",
	}

	for _, attack := range attacks {
		result := fw.Analyze(attack)
		if !result.Blocked {
			t.Errorf("XSS not blocked: %s", attack)
		}
		if result.ThreatType != firewall.ThreatXSS {
			t.Errorf("expected XSS type, got %s for: %s", result.ThreatType, attack)
		}
	}
}

func TestFirewallBlocksPathTraversal(t *testing.T) {
	fw := firewall.New()

	attacks := []string{
		"../../etc/passwd",
		"..\\..\\windows\\system32",
		"/etc/passwd",
	}

	for _, attack := range attacks {
		result := fw.Analyze(attack)
		if !result.Blocked {
			t.Errorf("path traversal not blocked: %s", attack)
		}
	}
}

func TestFirewallBlocksCmdInjection(t *testing.T) {
	fw := firewall.New()

	attacks := []string{
		"; rm -rf /",
		"| cat /etc/passwd",
		"`whoami`",
		"$(cat /etc/passwd)",
		"&& rm -rf /",
	}

	for _, attack := range attacks {
		result := fw.Analyze(attack)
		if !result.Blocked {
			t.Errorf("cmd injection not blocked: %s", attack)
		}
	}
}

func TestFirewallAllowsLegitimate(t *testing.T) {
	fw := firewall.New()

	legitimate := []string{
		"Hello, world!",
		"john.doe@example.com",
		"SELECT a product from our store",
		"/api/users/123",
		"The quick brown fox",
		"Price: $19.99",
		"2024-01-15T10:30:00Z",
	}

	for _, input := range legitimate {
		result := fw.Analyze(input)
		if result.Blocked {
			t.Errorf("legitimate input blocked: %s (detected as %s)", input, result.ThreatType)
		}
	}
}

func TestFirewallMiddleware(t *testing.T) {
	fw := firewall.New()

	handler := fw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))

	// Legitimate request
	req := httptest.NewRequest("GET", "/api/users?name=john", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("legitimate request should pass, got %d", rec.Code)
	}

	// Attack in query param
	req = httptest.NewRequest("GET", "/api/users?name='+OR+1=1--", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Errorf("attack should be blocked, got %d", rec.Code)
	}
}

func TestFirewallStats(t *testing.T) {
	fw := firewall.New()

	fw.Analyze("hello")
	fw.Analyze("world")
	fw.Analyze("' OR 1=1")

	stats := fw.GetStats()
	if stats.TotalRequests != 3 {
		t.Errorf("expected 3 total, got %d", stats.TotalRequests)
	}
	if stats.BlockedRequests != 1 {
		t.Errorf("expected 1 blocked, got %d", stats.BlockedRequests)
	}
}
