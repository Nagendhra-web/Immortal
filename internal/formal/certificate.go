package formal

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// Certificate is a cryptographically-signed attestation that a plan was
// verified safe against a list of invariants at a specific time using a
// specific public key. Auditable artifact.
type Certificate struct {
	PlanID           string    `json:"plan_id"`
	Invariants       []string  `json:"invariants"`
	VerifiedAt       time.Time `json:"verified_at"`
	StatesVisited    int       `json:"states_visited"`
	Depth            int       `json:"depth"`
	Algorithm        string    `json:"algorithm"`
	KeyID            string    `json:"key_id"`
	PublicKey        []byte    `json:"public_key"`
	CanonicalPayload []byte    `json:"canonical_payload"`
	Signature        []byte    `json:"signature"`
}

// CertSigner abstracts the signing primitive. Production plugs an ed25519 key;
// tests can plug a stub.
type CertSigner interface {
	KeyID() string
	PublicKey() []byte
	Sign(digest []byte) ([]byte, error)
	Algorithm() string
}

// CertVerifier abstracts the verification primitive.
type CertVerifier interface {
	Verify(publicKey []byte, digest, signature []byte) bool
	Algorithm() string
}

// canonicalPayload is the struct whose JSON bytes are signed.
// Fields must remain in this exact order; encoding/json preserves struct field order.
type canonicalPayload struct {
	PlanID        string   `json:"plan_id"`
	Invariants    []string `json:"invariants"`
	VerifiedAt    string   `json:"verified_at"`
	StatesVisited int      `json:"states_visited"`
	Depth         int      `json:"depth"`
	Algorithm     string   `json:"algorithm"`
	KeyID         string   `json:"key_id"`
}

// buildCanonicalPayload produces the sorted, deterministic JSON bytes for signing.
func buildCanonicalPayload(planID string, invariants []string, verifiedAt time.Time, statesVisited, depth int, algorithm, keyID string) ([]byte, error) {
	// Sort invariant names lexicographically for determinism.
	sorted := make([]string, len(invariants))
	copy(sorted, invariants)
	sort.Strings(sorted)

	p := canonicalPayload{
		PlanID:        planID,
		Invariants:    sorted,
		VerifiedAt:    verifiedAt.UTC().Format(time.RFC3339Nano),
		StatesVisited: statesVisited,
		Depth:         depth,
		Algorithm:     algorithm,
		KeyID:         keyID,
	}
	return json.Marshal(p)
}

// Issue signs a Certificate for a Result. Returns error if result is not Safe.
func Issue(result Result, planID string, invariants []Invariant, signer CertSigner) (*Certificate, error) {
	if !result.Safe {
		return nil, errors.New("certificate: cannot issue for non-safe result")
	}

	names := make([]string, len(invariants))
	for i, inv := range invariants {
		names[i] = inv.Name
	}

	verifiedAt := time.Now().UTC()
	payload, err := buildCanonicalPayload(planID, names, verifiedAt, result.StatesVisited, result.Depth, signer.Algorithm(), signer.KeyID())
	if err != nil {
		return nil, fmt.Errorf("certificate: marshal payload: %w", err)
	}

	digest := sha256.Sum256(payload)
	sig, err := signer.Sign(digest[:])
	if err != nil {
		return nil, fmt.Errorf("certificate: sign: %w", err)
	}

	// Sorted invariant names for the Certificate struct (same sort as payload).
	sortedNames := make([]string, len(names))
	copy(sortedNames, names)
	sort.Strings(sortedNames)

	return &Certificate{
		PlanID:           planID,
		Invariants:       sortedNames,
		VerifiedAt:       verifiedAt,
		StatesVisited:    result.StatesVisited,
		Depth:            result.Depth,
		Algorithm:        signer.Algorithm(),
		KeyID:            signer.KeyID(),
		PublicKey:        signer.PublicKey(),
		CanonicalPayload: payload,
		Signature:        sig,
	}, nil
}

// VerifyCertificate re-checks a Certificate's signature against its canonical payload.
// Returns (true, nil) on clean, (false, reason) when broken.
// Does NOT re-run the formal check.
func VerifyCertificate(c *Certificate, v CertVerifier) (bool, error) {
	if c == nil {
		return false, errors.New("verify: nil certificate")
	}
	if v.Algorithm() != c.Algorithm {
		return false, fmt.Errorf("verify: algorithm mismatch: cert=%q verifier=%q", c.Algorithm, v.Algorithm())
	}
	digest := sha256.Sum256(c.CanonicalPayload)
	if !v.Verify(c.PublicKey, digest[:], c.Signature) {
		return false, errors.New("verify: signature invalid")
	}
	return true, nil
}

// CheckWithCert composes Check + Issue into a single call. If the plan
// violates any invariant, returns (result, nil, nil). Otherwise returns
// (safe result, signed cert, nil).
func CheckWithCert(initial World, plan Plan, invariants []Invariant, signer CertSigner) (Result, *Certificate, error) {
	result := Check(initial, plan, invariants)
	if !result.Safe {
		return result, nil, nil
	}
	cert, err := Issue(result, plan.ID, invariants, signer)
	if err != nil {
		return result, nil, fmt.Errorf("CheckWithCert: %w", err)
	}
	return result, cert, nil
}

// CertificateToJSON marshals a Certificate to JSON for the audit artifact file format.
func CertificateToJSON(c *Certificate) ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// CertificateFromJSON unmarshals a Certificate from the audit artifact file format.
func CertificateFromJSON(b []byte) (*Certificate, error) {
	var c Certificate
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("CertificateFromJSON: %w", err)
	}
	return &c, nil
}

// --- Ed25519 implementations (stdlib only) ---

type ed25519Signer struct {
	keyID string
	priv  ed25519.PrivateKey
}

// NewEd25519Signer generates a fresh ed25519 key pair and returns a CertSigner.
func NewEd25519Signer(keyID string) (CertSigner, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("NewEd25519Signer: %w", err)
	}
	return &ed25519Signer{keyID: keyID, priv: priv}, nil
}

// LoadEd25519Signer wraps an existing private key as a CertSigner.
func LoadEd25519Signer(keyID string, priv ed25519.PrivateKey) CertSigner {
	return &ed25519Signer{keyID: keyID, priv: priv}
}

func (s *ed25519Signer) KeyID() string    { return s.keyID }
func (s *ed25519Signer) Algorithm() string { return "ed25519" }
func (s *ed25519Signer) PublicKey() []byte {
	pub := s.priv.Public().(ed25519.PublicKey)
	return []byte(pub)
}

// Sign signs a pre-computed digest using ed25519.Sign.
// Note: ed25519.Sign expects the message, not a pre-hashed digest; we pass the
// SHA-256 digest bytes as the message so the payload is bound via double-hash.
func (s *ed25519Signer) Sign(digest []byte) ([]byte, error) {
	sig := ed25519.Sign(s.priv, digest)
	return sig, nil
}

type ed25519Verifier struct{}

// NewEd25519Verifier returns a CertVerifier that uses stdlib ed25519.Verify.
func NewEd25519Verifier() CertVerifier { return ed25519Verifier{} }

func (ed25519Verifier) Algorithm() string { return "ed25519" }
func (ed25519Verifier) Verify(publicKey []byte, digest, signature []byte) bool {
	return ed25519.Verify(ed25519.PublicKey(publicKey), digest, signature)
}
