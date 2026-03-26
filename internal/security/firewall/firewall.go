package firewall

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

type ThreatType string

const (
	ThreatSQLi          ThreatType = "sql_injection"
	ThreatXSS           ThreatType = "xss"
	ThreatPathTraversal ThreatType = "path_traversal"
	ThreatCmdInjection  ThreatType = "command_injection"
	ThreatNone          ThreatType = "none"
)

type ThreatResult struct {
	Blocked    bool       `json:"blocked"`
	ThreatType ThreatType `json:"threat_type"`
	Pattern    string     `json:"pattern"`
	Input      string     `json:"input"`
	Layer      string     `json:"layer"` // which detection layer caught it
}

type Stats struct {
	TotalRequests   int64 `json:"total_requests"`
	BlockedRequests int64 `json:"blocked_requests"`
}

type Firewall struct {
	mu       sync.RWMutex
	patterns map[ThreatType][]*regexp.Regexp
	stats    Stats
	blocked  []ThreatResult
}

func New() *Firewall {
	fw := &Firewall{
		patterns: make(map[ThreatType][]*regexp.Regexp),
	}
	fw.loadDefaultRules()
	return fw
}

func (f *Firewall) loadDefaultRules() {
	f.patterns[ThreatSQLi] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(\bunion\b.*\bselect\b|\bor\b\s+1\s*=\s*1|\bdrop\b\s+\btable\b|--\s*$|;\s*\bdrop\b|'\s*\bor\b\s*'|1\s*=\s*1)`),
		regexp.MustCompile(`(?i)(\binsert\b\s+\binto\b|\bdelete\b\s+\bfrom\b|\bupdate\b.*\bset\b.*\bwhere\b)`),
		regexp.MustCompile(`(?i)(['"\)]\s*\bor\b\s*['"\(])`),
		regexp.MustCompile(`(?i)(sleep\s*\(\s*\d+|benchmark\s*\(|waitfor\s+delay)`),
		regexp.MustCompile(`(?i)(information_schema|sys\.objects|sysobjects|xp_cmdshell)`),
		regexp.MustCompile(`(?i)('\s*;\s*\w+|'\s*\+\s*'|concat\s*\()`),
	}
	f.patterns[ThreatXSS] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(<script[^>]*>|javascript\s*:|on\w+\s*=\s*["']|<iframe|<embed|<object)`),
		regexp.MustCompile(`(?i)(document\.cookie|window\.location|eval\s*\(|alert\s*\()`),
		regexp.MustCompile(`(?i)(String\.fromCharCode|atob\s*\(|btoa\s*\()`),
		regexp.MustCompile(`(?i)(<svg[^>]*on\w+|<img[^>]*on\w+|<body[^>]*on\w+|<div[^>]*on\w+)`),
		regexp.MustCompile(`(?i)(\.innerHTML\s*=|\.outerHTML\s*=|\.write\s*\(|\.writeln\s*\()`),
	}
	f.patterns[ThreatPathTraversal] = []*regexp.Regexp{
		regexp.MustCompile(`(\.\./|\.\.\\|%2e%2e|%252e%252e)`),
		regexp.MustCompile(`(?i)(/etc/passwd|/etc/shadow|c:\\windows|/proc/self)`),
		regexp.MustCompile(`(?i)(\.\./\.\./|\.\.\\\.\.\\)`),
	}
	f.patterns[ThreatCmdInjection] = []*regexp.Regexp{
		regexp.MustCompile("(?i)(;\\s*(ls|cat|rm|wget|curl|bash|sh|python|perl|nc|ncat|nmap)\\b|\\|\\s*(ls|cat|rm|nmap|ncat)|`[^`]+`)"),
		regexp.MustCompile("(?i)(\\$\\(|\\$\\{|&&\\s*(rm|cat|ls))"),
		regexp.MustCompile(`(?i)(\bping\b\s+-[nc]|\bnslookup\b|\bdig\b\s+)`),
	}
}

// normalize applies multiple decoding layers to catch obfuscated payloads.
// This is the key defense against encoding tricks.
func normalize(input string) []string {
	variants := []string{input}

	// Layer 1: URL decode (catches %27, %3C, etc.)
	decoded, err := url.QueryUnescape(input)
	if err == nil && decoded != input {
		variants = append(variants, decoded)
	}

	// Layer 2: Double URL decode (catches %2527 → %27 → ')
	doubleDecoded, err := url.QueryUnescape(decoded)
	if err == nil && doubleDecoded != decoded {
		variants = append(variants, doubleDecoded)
	}

	// Layer 3: HTML entity decode
	htmlDecoded := decodeHTMLEntities(input)
	if htmlDecoded != input {
		variants = append(variants, htmlDecoded)
	}

	// Layer 4: Unicode normalization (catches fullwidth chars)
	unicodeNorm := normalizeUnicode(input)
	if unicodeNorm != input {
		variants = append(variants, unicodeNorm)
	}

	// Layer 5: Remove null bytes (poison null byte attack)
	noNull := strings.ReplaceAll(input, "\x00", "")
	noNull = strings.ReplaceAll(noNull, "%00", "")
	if noNull != input {
		variants = append(variants, noNull)
	}

	// Layer 6: Collapse whitespace (catches S E L E C T)
	collapsed := collapseSpaces(input)
	if collapsed != input {
		variants = append(variants, collapsed)
	}

	// Layer 7: Remove comments (catches /**/UN/**/ION)
	noComments := removeSQLComments(input)
	if noComments != input {
		variants = append(variants, noComments)
	}

	// Layer 8: Case variations are handled by (?i) in regex

	return variants
}

func decodeHTMLEntities(s string) string {
	r := strings.NewReplacer(
		"&lt;", "<", "&gt;", ">", "&amp;", "&", "&quot;", "\"",
		"&#39;", "'", "&#x27;", "'", "&#34;", "\"", "&#x22;", "\"",
		"&#60;", "<", "&#62;", ">", "&#x3c;", "<", "&#x3e;", ">",
		"&#x3C;", "<", "&#x3E;", ">",
		"&#47;", "/", "&#x2f;", "/", "&#x2F;", "/",
		"&#97;", "a", "&#108;", "l", "&#101;", "e", "&#114;", "r", "&#116;", "t",
	)
	return r.Replace(s)
}

func normalizeUnicode(s string) string {
	// Convert fullwidth characters to ASCII equivalents
	var result strings.Builder
	for _, r := range s {
		if r >= 0xFF01 && r <= 0xFF5E {
			result.WriteRune(r - 0xFEE0) // Fullwidth to ASCII
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func collapseSpaces(s string) string {
	var result strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevSpace {
				result.WriteRune(' ')
			}
			prevSpace = true
		} else {
			result.WriteRune(r)
			prevSpace = false
		}
	}
	return result.String()
}

func removeSQLComments(s string) string {
	// Remove /* ... */ comments
	re := regexp.MustCompile(`/\*.*?\*/`)
	result := re.ReplaceAllString(s, "")
	// Remove -- comments
	re2 := regexp.MustCompile(`--[^\n]*`)
	result = re2.ReplaceAllString(result, "")
	return result
}

// Analyze checks input against all threat patterns.
// Uses multi-layer normalization to defeat encoding/obfuscation.
func (f *Firewall) Analyze(input string) ThreatResult {
	atomic.AddInt64(&f.stats.TotalRequests, 1)

	// Generate all normalized variants
	variants := normalize(input)

	// Check each variant against each pattern
	for _, variant := range variants {
		for threatType, patterns := range f.patterns {
			for _, p := range patterns {
				if p.MatchString(variant) {
					atomic.AddInt64(&f.stats.BlockedRequests, 1)
					layer := "raw"
					if variant != input {
						layer = "decoded"
					}
					result := ThreatResult{
						Blocked:    true,
						ThreatType: threatType,
						Pattern:    p.String(),
						Input:      input,
						Layer:      layer,
					}
					f.mu.Lock()
					f.blocked = append(f.blocked, result)
					f.mu.Unlock()
					return result
				}
			}
		}
	}
	return ThreatResult{Blocked: false, ThreatType: ThreatNone}
}

func (f *Firewall) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check URL
		if result := f.Analyze(r.URL.String()); result.Blocked {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		// Check query params
		for _, values := range r.URL.Query() {
			for _, v := range values {
				if result := f.Analyze(v); result.Blocked {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
			}
		}
		// Check headers
		for _, values := range r.Header {
			for _, v := range values {
				if result := f.Analyze(v); result.Blocked {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (f *Firewall) GetStats() Stats {
	return Stats{
		TotalRequests:   atomic.LoadInt64(&f.stats.TotalRequests),
		BlockedRequests: atomic.LoadInt64(&f.stats.BlockedRequests),
	}
}

func (f *Firewall) BlockedRequests() []ThreatResult {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]ThreatResult, len(f.blocked))
	copy(out, f.blocked)
	return out
}
