package zerotrust_test

import (
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/security/zerotrust"
)

func TestIssueAndValidateToken(t *testing.T) {
	v := zerotrust.New("my-secret-key")

	identity := v.IssueToken("api-service", time.Hour)
	if identity.Token == "" {
		t.Error("expected non-empty token")
	}
	if identity.ServiceName != "api-service" {
		t.Error("wrong service name")
	}

	validated, err := v.ValidateToken(identity.Token)
	if err != nil {
		t.Errorf("valid token should validate: %v", err)
	}
	if validated.ServiceName != "api-service" {
		t.Error("wrong service in validated identity")
	}
}

func TestInvalidToken(t *testing.T) {
	v := zerotrust.New("secret")

	_, err := v.ValidateToken("fake-token")
	if err == nil {
		t.Error("fake token should fail validation")
	}
}

func TestExpiredToken(t *testing.T) {
	v := zerotrust.New("secret")

	identity := v.IssueToken("svc", 1*time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	_, err := v.ValidateToken(identity.Token)
	if err == nil {
		t.Error("expired token should fail validation")
	}
}

func TestAccessPolicy(t *testing.T) {
	v := zerotrust.New("secret")

	v.SetPolicy("database", &zerotrust.Policy{
		AllowedServices: []string{"api-service", "auth-service"},
		AllowedPaths:    []string{"/read", "/write"},
	})

	// Allowed service + path
	err := v.CheckAccess("api-service", "database", "/read")
	if err != nil {
		t.Errorf("should be allowed: %v", err)
	}

	// Blocked service
	err = v.CheckAccess("evil-service", "database", "/read")
	if err == nil {
		t.Error("unauthorized service should be blocked")
	}

	// Blocked path
	err = v.CheckAccess("api-service", "database", "/admin")
	if err == nil {
		t.Error("unauthorized path should be blocked")
	}
}

func TestRevokeToken(t *testing.T) {
	v := zerotrust.New("secret")

	identity := v.IssueToken("svc", time.Hour)

	// Valid before revoke
	_, err := v.ValidateToken(identity.Token)
	if err != nil {
		t.Error("should be valid before revoke")
	}

	// Revoke
	v.RevokeToken(identity.Token)

	// Invalid after revoke
	_, err = v.ValidateToken(identity.Token)
	if err == nil {
		t.Error("should be invalid after revoke")
	}
}

func TestActiveTokenCount(t *testing.T) {
	v := zerotrust.New("secret")

	v.IssueToken("svc1", time.Hour)
	v.IssueToken("svc2", time.Hour)
	v.IssueToken("svc3", 1*time.Millisecond) // Will expire

	time.Sleep(10 * time.Millisecond)

	if v.ActiveTokenCount() != 2 {
		t.Errorf("expected 2 active tokens, got %d", v.ActiveTokenCount())
	}
}

func TestUniqueTokens(t *testing.T) {
	v := zerotrust.New("secret")

	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := v.IssueToken("svc", time.Hour)
		if tokens[id.Token] {
			t.Fatalf("duplicate token at iteration %d", i)
		}
		tokens[id.Token] = true
	}
}

func TestWildcardPolicy(t *testing.T) {
	v := zerotrust.New("secret")

	v.SetPolicy("public-api", &zerotrust.Policy{
		AllowedServices: []string{"*"},
		AllowedPaths:    []string{"*"},
	})

	err := v.CheckAccess("any-service", "public-api", "/any-path")
	if err != nil {
		t.Errorf("wildcard should allow everything: %v", err)
	}
}
