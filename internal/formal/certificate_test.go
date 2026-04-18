package formal

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"
)

// helpers ---------------------------------------------------------------

func testSigner(t *testing.T) CertSigner {
	t.Helper()
	s, err := NewEd25519Signer("test-key-1")
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}
	return s
}

func safeResult() Result {
	return Result{Safe: true, StatesVisited: 3, Depth: 2}
}

func testInvariants() []Invariant {
	return []Invariant{
		AtLeastNHealthy(1),
		NoMoreThanKUnhealthy(1),
	}
}

func safePlan() Plan {
	return Plan{
		ID: "safe-plan",
		Steps: []Action{
			{Name: "noop", Fn: func(w World) World { return w }},
		},
	}
}

func violatingPlan() Plan {
	return Plan{
		ID: "bad-plan",
		Steps: []Action{
			{Name: "kill-all", Fn: func(w World) World {
				for k, s := range w {
					s.Healthy = false
					w[k] = s
				}
				return w
			}},
		},
	}
}

// tests -----------------------------------------------------------------

func TestIssue_FromSafeResult_ReturnsCert(t *testing.T) {
	signer := testSigner(t)
	invs := testInvariants()
	cert, err := Issue(safeResult(), "plan-abc", invs, signer)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	if cert == nil {
		t.Fatal("cert is nil")
	}
	if cert.PlanID != "plan-abc" {
		t.Errorf("PlanID: got %q want %q", cert.PlanID, "plan-abc")
	}
	if cert.Algorithm != "ed25519" {
		t.Errorf("Algorithm: got %q", cert.Algorithm)
	}
	if len(cert.Signature) != ed25519.SignatureSize {
		t.Errorf("Signature length: got %d want %d", len(cert.Signature), ed25519.SignatureSize)
	}
	if len(cert.PublicKey) != ed25519.PublicKeySize {
		t.Errorf("PublicKey length: got %d want %d", len(cert.PublicKey), ed25519.PublicKeySize)
	}
	if len(cert.CanonicalPayload) == 0 {
		t.Error("CanonicalPayload must not be empty")
	}
}

func TestIssue_FromViolationResult_Errors(t *testing.T) {
	signer := testSigner(t)
	bad := Result{Safe: false, Violation: &Violation{Invariant: "some-inv"}}
	cert, err := Issue(bad, "plan-x", testInvariants(), signer)
	if err == nil {
		t.Fatal("expected error for non-safe result, got nil")
	}
	if cert != nil {
		t.Error("cert must be nil on error")
	}
}

func TestVerifyCertificate_CleanRoundtrip(t *testing.T) {
	signer := testSigner(t)
	cert, err := Issue(safeResult(), "plan-rt", testInvariants(), signer)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	ok, reason := VerifyCertificate(cert, NewEd25519Verifier())
	if !ok {
		t.Fatalf("VerifyCertificate failed: %v", reason)
	}
}

func TestVerifyCertificate_TamperedPayload_Fails(t *testing.T) {
	signer := testSigner(t)
	cert, err := Issue(safeResult(), "plan-tp", testInvariants(), signer)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	// Flip one byte in canonical payload.
	cert.CanonicalPayload[0] ^= 0xFF
	ok, reason := VerifyCertificate(cert, NewEd25519Verifier())
	if ok {
		t.Fatal("expected verify to fail after payload tamper")
	}
	if reason == nil {
		t.Fatal("expected non-nil reason")
	}
}

func TestVerifyCertificate_TamperedSignature_Fails(t *testing.T) {
	signer := testSigner(t)
	cert, err := Issue(safeResult(), "plan-ts", testInvariants(), signer)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	// Flip one byte in signature.
	cert.Signature[0] ^= 0xFF
	ok, reason := VerifyCertificate(cert, NewEd25519Verifier())
	if ok {
		t.Fatal("expected verify to fail after signature tamper")
	}
	if reason == nil {
		t.Fatal("expected non-nil reason")
	}
}

func TestVerifyCertificate_WrongPublicKey_Fails(t *testing.T) {
	signer := testSigner(t)
	cert, err := Issue(safeResult(), "plan-wpk", testInvariants(), signer)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	// Replace public key with a freshly-generated unrelated key.
	wrongPub, _, err2 := ed25519.GenerateKey(rand.Reader)
	if err2 != nil {
		t.Fatalf("GenerateKey: %v", err2)
	}
	cert.PublicKey = []byte(wrongPub)
	ok, reason := VerifyCertificate(cert, NewEd25519Verifier())
	if ok {
		t.Fatal("expected verify to fail with wrong public key")
	}
	if reason == nil {
		t.Fatal("expected non-nil reason")
	}
}

func TestCheckWithCert_SafePlan_ProducesCert(t *testing.T) {
	signer := testSigner(t)
	invs := []Invariant{AtLeastNHealthy(1)}
	result, cert, err := CheckWithCert(initialWorld(), safePlan(), invs, signer)
	if err != nil {
		t.Fatalf("CheckWithCert error: %v", err)
	}
	if !result.Safe {
		t.Fatal("expected safe result")
	}
	if cert == nil {
		t.Fatal("cert must not be nil for safe plan")
	}
	// Verify the produced certificate.
	ok, reason := VerifyCertificate(cert, NewEd25519Verifier())
	if !ok {
		t.Fatalf("certificate verification failed: %v", reason)
	}
}

func TestCheckWithCert_UnsafePlan_NoCert(t *testing.T) {
	signer := testSigner(t)
	invs := []Invariant{AtLeastNHealthy(2)}
	result, cert, err := CheckWithCert(initialWorld(), violatingPlan(), invs, signer)
	if err != nil {
		t.Fatalf("CheckWithCert error: %v", err)
	}
	if result.Safe {
		t.Fatal("expected violation result")
	}
	if cert != nil {
		t.Fatal("cert must be nil when plan violates invariant")
	}
}

func TestCertificateToJSON_RoundTrip(t *testing.T) {
	signer := testSigner(t)
	cert, err := Issue(safeResult(), "plan-json", testInvariants(), signer)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	data, err := CertificateToJSON(cert)
	if err != nil {
		t.Fatalf("CertificateToJSON: %v", err)
	}
	// Confirm it's valid JSON.
	if !json.Valid(data) {
		t.Fatal("CertificateToJSON produced invalid JSON")
	}
	decoded, err := CertificateFromJSON(data)
	if err != nil {
		t.Fatalf("CertificateFromJSON: %v", err)
	}
	ok, reason := VerifyCertificate(decoded, NewEd25519Verifier())
	if !ok {
		t.Fatalf("post-JSON-roundtrip verification failed: %v", reason)
	}
}

func TestCertificate_CanonicalPayload_Deterministic(t *testing.T) {
	// Use a fixed key so only the payload comparison matters.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	signer := LoadEd25519Signer("fixed-key", priv)

	result := safeResult()
	invs := testInvariants()

	cert1, err := Issue(result, "det-plan", invs, signer)
	if err != nil {
		t.Fatalf("Issue 1: %v", err)
	}
	// Small sleep would change VerifiedAt, so we compare only the sorted
	// invariants portion. Instead we verify same-second calls produce identical
	// structural fields by re-building the payload with the same time.
	cert2, err := Issue(result, "det-plan", invs, signer)
	if err != nil {
		t.Fatalf("Issue 2: %v", err)
	}

	// The canonical payloads must parse to the same structure (same fields),
	// even though VerifiedAt may differ by nanoseconds.
	// We verify determinism of the *sorted invariants* and *algorithm* fields
	// by parsing and comparing those.
	var p1, p2 canonicalPayload
	if err := json.Unmarshal(cert1.CanonicalPayload, &p1); err != nil {
		t.Fatalf("unmarshal p1: %v", err)
	}
	if err := json.Unmarshal(cert2.CanonicalPayload, &p2); err != nil {
		t.Fatalf("unmarshal p2: %v", err)
	}
	if p1.PlanID != p2.PlanID {
		t.Errorf("PlanID differs: %q vs %q", p1.PlanID, p2.PlanID)
	}
	if len(p1.Invariants) != len(p2.Invariants) {
		t.Fatalf("invariant count differs: %d vs %d", len(p1.Invariants), len(p2.Invariants))
	}
	for i := range p1.Invariants {
		if p1.Invariants[i] != p2.Invariants[i] {
			t.Errorf("invariant[%d] differs: %q vs %q", i, p1.Invariants[i], p2.Invariants[i])
		}
	}
	if p1.Algorithm != p2.Algorithm {
		t.Errorf("Algorithm differs")
	}
	if p1.KeyID != p2.KeyID {
		t.Errorf("KeyID differs")
	}
	if p1.StatesVisited != p2.StatesVisited {
		t.Errorf("StatesVisited differs")
	}
	if p1.Depth != p2.Depth {
		t.Errorf("Depth differs")
	}

	// Verify both certs are individually valid.
	v := NewEd25519Verifier()
	if ok, r := VerifyCertificate(cert1, v); !ok {
		t.Fatalf("cert1 invalid: %v", r)
	}
	if ok, r := VerifyCertificate(cert2, v); !ok {
		t.Fatalf("cert2 invalid: %v", r)
	}
}

func TestCertificate_CanonicalPayload_SameTimestamp_Deterministic(t *testing.T) {
	// Build two payloads with the exact same inputs and assert byte-for-byte equality.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	signer := LoadEd25519Signer("fixed-key-2", priv)

	// Build payload directly via the exported Issue path, but use a fixed time
	// by building the canonical payload helper directly.
	invs := testInvariants()
	names := make([]string, len(invs))
	for i, inv := range invs {
		names[i] = inv.Name
	}

	// We call buildCanonicalPayload twice with identical args.
	p1, err := buildCanonicalPayload("fixed-plan", names, fixedTime(), 3, 2, signer.Algorithm(), signer.KeyID())
	if err != nil {
		t.Fatalf("buildCanonicalPayload 1: %v", err)
	}
	p2, err := buildCanonicalPayload("fixed-plan", names, fixedTime(), 3, 2, signer.Algorithm(), signer.KeyID())
	if err != nil {
		t.Fatalf("buildCanonicalPayload 2: %v", err)
	}
	if !bytes.Equal(p1, p2) {
		t.Errorf("canonical payloads differ for identical inputs:\n%s\n---\n%s", p1, p2)
	}
}

func TestCertificate_InvariantsSorted(t *testing.T) {
	signer := testSigner(t)
	// Supply invariants in reverse lexicographic order.
	invs := []Invariant{
		NoMoreThanKUnhealthy(1),
		AtLeastNHealthy(1),
	}
	cert, err := Issue(safeResult(), "sort-plan", invs, signer)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Parse the canonical payload and check invariants are sorted.
	var p canonicalPayload
	if err := json.Unmarshal(cert.CanonicalPayload, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for i := 1; i < len(p.Invariants); i++ {
		if p.Invariants[i] < p.Invariants[i-1] {
			t.Errorf("invariants not sorted at index %d: %q < %q", i, p.Invariants[i], p.Invariants[i-1])
		}
	}
	// Also check Certificate.Invariants field is sorted.
	for i := 1; i < len(cert.Invariants); i++ {
		if cert.Invariants[i] < cert.Invariants[i-1] {
			t.Errorf("cert.Invariants not sorted at index %d: %q < %q", i, cert.Invariants[i], cert.Invariants[i-1])
		}
	}
}

// fixedTime returns a stable time.Time for deterministic payload testing.
func fixedTime() time.Time {
	// Use a package-level import; time is already imported via certificate.go
	// since we're in the same package.
	return time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
}
