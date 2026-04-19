package tenant_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/tenant"
)

// === DATA ISOLATION ===

func TestIsolation_SeparatePaths(t *testing.T) {
	iso := tenant.NewIsolation("/data")

	pathA := iso.DBPath("company-a")
	pathB := iso.DBPath("company-b")

	if pathA == pathB {
		t.Error("tenants should have different DB paths")
	}
	if !strings.Contains(pathA, "company-a") {
		t.Error("path should contain tenant ID")
	}
	if !strings.Contains(pathB, "company-b") {
		t.Error("path should contain tenant ID")
	}
	t.Logf("  Company A: %s", pathA)
	t.Logf("  Company B: %s", pathB)
	t.Log("  ✅ Per-tenant database isolation")
}

func TestIsolation_ConsistentPaths(t *testing.T) {
	iso := tenant.NewIsolation("/data")
	path1 := iso.DBPath("acme")
	path2 := iso.DBPath("acme")
	if path1 != path2 {
		t.Error("same tenant should get same path")
	}
}

func TestIsolation_DataDirs(t *testing.T) {
	iso := tenant.NewIsolation("/data")
	dir := iso.DataDir("acme")
	if !strings.Contains(dir, "acme") {
		t.Error("data dir should contain tenant ID")
	}
}

func TestIsolation_AllPaths(t *testing.T) {
	iso := tenant.NewIsolation("/data")
	iso.DBPath("a")
	iso.DBPath("b")
	iso.DBPath("c")
	all := iso.AllPaths()
	if len(all) != 3 {
		t.Errorf("expected 3 paths, got %d", len(all))
	}
}

// === PER-TENANT RATE LIMITING ===

func TestRateLimit_AllowsWithinLimit(t *testing.T) {
	rl := tenant.NewTenantRateLimiter(tenant.RateLimitConfig{
		RequestsPerSecond: 10,
		BurstSize:         10,
	})

	allowed := 0
	for i := 0; i < 10; i++ {
		if rl.Allow("tenant-a") {
			allowed++
		}
	}
	if allowed != 10 {
		t.Errorf("expected 10 allowed (burst), got %d", allowed)
	}
	t.Logf("  ✅ Allowed %d requests within burst limit", allowed)
}

func TestRateLimit_BlocksOverLimit(t *testing.T) {
	rl := tenant.NewTenantRateLimiter(tenant.RateLimitConfig{
		RequestsPerSecond: 5,
		BurstSize:         5,
	})

	// Exhaust burst
	for i := 0; i < 5; i++ {
		rl.Allow("tenant-a")
	}

	// Next should be blocked
	if rl.Allow("tenant-a") {
		t.Error("should block after burst exhausted")
	}
	t.Log("  ✅ Blocked after burst limit exhausted")
}

func TestRateLimit_RefillsOverTime(t *testing.T) {
	rl := tenant.NewTenantRateLimiter(tenant.RateLimitConfig{
		RequestsPerSecond: 1000,
		BurstSize:         5,
	})

	// Exhaust burst
	for i := 0; i < 5; i++ {
		rl.Allow("tenant-a")
	}

	// Wait generously for refill (1000/sec = 1 token per ms, wait 500ms = ~500 tokens)
	time.Sleep(500 * time.Millisecond)

	if !rl.Allow("tenant-a") {
		t.Error("should allow after refill")
	}
	t.Log("  ✅ Tokens refill over time")
}

func TestRateLimit_PerTenantIsolation(t *testing.T) {
	rl := tenant.NewTenantRateLimiter(tenant.RateLimitConfig{
		RequestsPerSecond: 5,
		BurstSize:         5,
	})

	// Exhaust tenant A
	for i := 0; i < 5; i++ {
		rl.Allow("tenant-a")
	}

	// Tenant B should still work
	if !rl.Allow("tenant-b") {
		t.Error("tenant B should not be affected by tenant A's usage")
	}
	t.Log("  ✅ Per-tenant isolation: A exhausted, B still works")
}

func TestRateLimit_CustomPerTenant(t *testing.T) {
	rl := tenant.NewTenantRateLimiter(tenant.RateLimitConfig{
		RequestsPerSecond: 5,
		BurstSize:         5,
	})

	// Give tenant-vip higher limits
	rl.SetLimit("tenant-vip", 1000, 100)

	allowed := 0
	for i := 0; i < 50; i++ {
		if rl.Allow("tenant-vip") {
			allowed++
		}
	}
	if allowed != 50 {
		t.Errorf("VIP should allow 50 (burst=100), got %d", allowed)
	}
	t.Logf("  ✅ Custom rate limit: VIP got %d/50 requests", allowed)
}

func TestRateLimit_Remaining(t *testing.T) {
	rl := tenant.NewTenantRateLimiter(tenant.RateLimitConfig{
		RequestsPerSecond: 10,
		BurstSize:         10,
	})

	if rl.Remaining("new-tenant") != 10 {
		t.Error("new tenant should have full burst")
	}

	rl.Allow("new-tenant")
	rl.Allow("new-tenant")

	rem := rl.Remaining("new-tenant")
	if rem != 8 {
		t.Errorf("expected 8 remaining, got %d", rem)
	}
}
