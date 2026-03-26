package rasp

import (
	"strings"
	"sync"
	"sync/atomic"
)

type ViolationType string

const (
	ViolationDangerousExec ViolationType = "dangerous_exec"
	ViolationFileAccess    ViolationType = "sensitive_file_access"
	ViolationNetworkExfil  ViolationType = "network_exfiltration"
	ViolationNone          ViolationType = "none"
)

type Violation struct {
	Type    ViolationType `json:"type"`
	Detail  string        `json:"detail"`
	Blocked bool          `json:"blocked"`
}

type Config struct {
	BlockDangerousCommands bool
	BlockSensitiveFiles    bool
	BlockExfiltration      bool
	SensitiveFilePaths     []string
	DangerousCommands      []string
	ExfilDomains           []string
}

type Monitor struct {
	mu          sync.RWMutex
	config      Config
	violations  []Violation
	totalChecks int64
	blocked     int64
}

func New(config Config) *Monitor {
	if len(config.DangerousCommands) == 0 {
		config.DangerousCommands = []string{
			"rm -rf", "mkfs", "dd if=", "chmod 777",
			"| sh", "| bash", "curl | sh", "wget | sh", "eval(", "exec(",
			"> /dev/sda", "shutdown", "reboot", "kill -9",
		}
	}
	if len(config.SensitiveFilePaths) == 0 {
		config.SensitiveFilePaths = []string{
			"/etc/passwd", "/etc/shadow", ".env",
			"credentials", "id_rsa", ".pem",
			"secret", "token", ".key",
		}
	}
	if len(config.ExfilDomains) == 0 {
		config.ExfilDomains = []string{
			"pastebin.com", "requestbin.com", "ngrok.io",
			"burpcollaborator.net", "webhook.site",
		}
	}
	return &Monitor{config: config}
}

func NewDefault() *Monitor {
	return New(Config{
		BlockDangerousCommands: true,
		BlockSensitiveFiles:    true,
		BlockExfiltration:      true,
	})
}

func (m *Monitor) CheckCommand(cmd string) Violation {
	atomic.AddInt64(&m.totalChecks, 1)
	lower := strings.ToLower(cmd)

	for _, dangerous := range m.config.DangerousCommands {
		if strings.Contains(lower, strings.ToLower(dangerous)) {
			v := Violation{
				Type:    ViolationDangerousExec,
				Detail:  "dangerous command detected: " + cmd,
				Blocked: m.config.BlockDangerousCommands,
			}
			if m.config.BlockDangerousCommands {
				atomic.AddInt64(&m.blocked, 1)
			}
			m.mu.Lock()
			m.violations = append(m.violations, v)
			m.mu.Unlock()
			return v
		}
	}
	return Violation{Type: ViolationNone}
}

func (m *Monitor) CheckFileAccess(path string) Violation {
	atomic.AddInt64(&m.totalChecks, 1)
	lower := strings.ToLower(path)

	for _, sensitive := range m.config.SensitiveFilePaths {
		if strings.Contains(lower, strings.ToLower(sensitive)) {
			v := Violation{
				Type:    ViolationFileAccess,
				Detail:  "sensitive file access: " + path,
				Blocked: m.config.BlockSensitiveFiles,
			}
			if m.config.BlockSensitiveFiles {
				atomic.AddInt64(&m.blocked, 1)
			}
			m.mu.Lock()
			m.violations = append(m.violations, v)
			m.mu.Unlock()
			return v
		}
	}
	return Violation{Type: ViolationNone}
}

func (m *Monitor) CheckOutbound(url string) Violation {
	atomic.AddInt64(&m.totalChecks, 1)
	lower := strings.ToLower(url)

	for _, domain := range m.config.ExfilDomains {
		if strings.Contains(lower, strings.ToLower(domain)) {
			v := Violation{
				Type:    ViolationNetworkExfil,
				Detail:  "data exfiltration attempt: " + url,
				Blocked: m.config.BlockExfiltration,
			}
			if m.config.BlockExfiltration {
				atomic.AddInt64(&m.blocked, 1)
			}
			m.mu.Lock()
			m.violations = append(m.violations, v)
			m.mu.Unlock()
			return v
		}
	}
	return Violation{Type: ViolationNone}
}

func (m *Monitor) Violations() []Violation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Violation, len(m.violations))
	copy(out, m.violations)
	return out
}

func (m *Monitor) Stats() (totalChecks, blocked int64) {
	return atomic.LoadInt64(&m.totalChecks), atomic.LoadInt64(&m.blocked)
}
