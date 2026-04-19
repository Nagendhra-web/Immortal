package secrets_test

import (
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/security/secrets"
)

func TestScannerDetectsAWSKeys(t *testing.T) {
	s := secrets.New()
	findings := s.Scan("my key is AKIAIOSFODNN7EXAMPLE")

	if len(findings) == 0 {
		t.Error("expected to find AWS key")
	}
	found := false
	for _, f := range findings {
		if f.Type == secrets.SecretAWSKey {
			found = true
		}
	}
	if !found {
		t.Error("expected AWS key type finding")
	}
}

func TestScannerDetectsGitHubTokens(t *testing.T) {
	s := secrets.New()
	findings := s.Scan("token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij")

	if len(findings) == 0 {
		t.Error("expected to find GitHub token")
	}
}

func TestScannerDetectsPrivateKeys(t *testing.T) {
	s := secrets.New()
	findings := s.Scan("-----BEGIN RSA PRIVATE KEY-----\nMIIBog...")

	if len(findings) == 0 {
		t.Error("expected to find private key")
	}
}

func TestScannerDetectsPasswords(t *testing.T) {
	s := secrets.New()
	findings := s.Scan(`password = "supersecret123"`)

	if len(findings) == 0 {
		t.Error("expected to find password")
	}
}

func TestScannerDetectsJWT(t *testing.T) {
	s := secrets.New()
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	findings := s.Scan(jwt)

	if len(findings) == 0 {
		t.Error("expected to find JWT token")
	}
}

func TestScannerNoFalsePositives(t *testing.T) {
	s := secrets.New()

	safe := []string{
		"Hello, world!",
		"user@example.com",
		"The API was designed for developers",
		"password requirements: 8 characters",
		"https://api.example.com/v1/users",
	}

	for _, input := range safe {
		findings := s.Scan(input)
		if len(findings) > 0 {
			t.Errorf("false positive for: %s (found %s)", input, findings[0].Type)
		}
	}
}

func TestScannerMasksSecrets(t *testing.T) {
	s := secrets.New()
	findings := s.Scan("AKIAIOSFODNN7EXAMPLE")

	if len(findings) == 0 {
		t.Fatal("expected finding")
	}
	if findings[0].Match == "AKIAIOSFODNN7EXAMPLE" {
		t.Error("secret should be masked, not shown in full")
	}
}

func TestScannerHasSecrets(t *testing.T) {
	s := secrets.New()

	if s.HasSecrets("normal text") {
		t.Error("should not detect secrets in normal text")
	}
	if !s.HasSecrets("my AKIAIOSFODNN7EXAMPLE key") {
		t.Error("should detect AWS key")
	}
}

func TestScannerWithLocation(t *testing.T) {
	s := secrets.New()
	findings := s.ScanWithLocation("AKIAIOSFODNN7EXAMPLE", "config.yaml:15")

	if len(findings) == 0 {
		t.Fatal("expected finding")
	}
	if findings[0].Location != "config.yaml:15" {
		t.Errorf("expected location 'config.yaml:15', got '%s'", findings[0].Location)
	}
}
