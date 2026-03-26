package secrets

import (
	"regexp"
	"sync"
)

type SecretType string

const (
	SecretAWSKey      SecretType = "aws_access_key"
	SecretAWSSecret   SecretType = "aws_secret_key"
	SecretGitHubToken SecretType = "github_token"
	SecretGenericAPI  SecretType = "generic_api_key"
	SecretPrivateKey  SecretType = "private_key"
	SecretPassword    SecretType = "password_in_code"
	SecretJWT         SecretType = "jwt_token"
)

type Finding struct {
	Type     SecretType `json:"type"`
	Match    string     `json:"match"`
	Location string     `json:"location"`
	Line     int        `json:"line"`
}

type Scanner struct {
	mu       sync.RWMutex
	patterns map[SecretType]*regexp.Regexp
	findings []Finding
}

func New() *Scanner {
	s := &Scanner{
		patterns: make(map[SecretType]*regexp.Regexp),
	}
	s.loadDefaultPatterns()
	return s
}

func (s *Scanner) loadDefaultPatterns() {
	s.patterns[SecretAWSKey] = regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`)
	s.patterns[SecretAWSSecret] = regexp.MustCompile(`(?i)aws_secret_access_key\s*[=:]\s*["']?[A-Za-z0-9/+=]{40}`)
	s.patterns[SecretGitHubToken] = regexp.MustCompile(`(?i)(ghp_[A-Za-z0-9]{36}|github_pat_[A-Za-z0-9_]{82})`)
	s.patterns[SecretGenericAPI] = regexp.MustCompile(`(?i)(api[_-]?key|apikey|api[_-]?secret)\s*[=:]\s*["']?[A-Za-z0-9]{20,}`)
	s.patterns[SecretPrivateKey] = regexp.MustCompile(`-----BEGIN (RSA |EC |DSA )?PRIVATE KEY-----`)
	s.patterns[SecretPassword] = regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[=:]\s*["'][^"']{8,}["']`)
	s.patterns[SecretJWT] = regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+`)
}

func (s *Scanner) Scan(input string) []Finding {
	var findings []Finding

	for secretType, pattern := range s.patterns {
		matches := pattern.FindAllString(input, -1)
		for _, match := range matches {
			f := Finding{
				Type:  secretType,
				Match: maskSecret(match),
			}
			findings = append(findings, f)
		}
	}

	s.mu.Lock()
	s.findings = append(s.findings, findings...)
	s.mu.Unlock()

	return findings
}

func (s *Scanner) ScanWithLocation(input string, location string) []Finding {
	findings := s.Scan(input)
	for i := range findings {
		findings[i].Location = location
	}
	return findings
}

func (s *Scanner) HasSecrets(input string) bool {
	for _, pattern := range s.patterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}

func (s *Scanner) AllFindings() []Finding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Finding, len(s.findings))
	copy(out, s.findings)
	return out
}

func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}
