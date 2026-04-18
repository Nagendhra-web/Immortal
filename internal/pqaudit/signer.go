package pqaudit

import (
	"crypto/ed25519"
	"crypto/rand"
)

// Signer abstracts signature algorithms. Production: ed25519; future: post-quantum.
type Signer interface {
	Sign(digest []byte) ([]byte, error)
	Verify(digest, signature []byte) bool
	KeyID() string
	Algorithm() string
	PublicKeyBytes() []byte
}

type ed25519Signer struct {
	keyID string
	priv  ed25519.PrivateKey
	pub   ed25519.PublicKey
}

// NewEd25519Signer generates a fresh ed25519 keypair.
func NewEd25519Signer(keyID string) (Signer, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &ed25519Signer{keyID: keyID, priv: priv, pub: pub}, nil
}

// LoadEd25519Signer wraps an existing private key (the public key is derived from it).
func LoadEd25519Signer(keyID string, priv ed25519.PrivateKey) Signer {
	pub := priv.Public().(ed25519.PublicKey)
	return &ed25519Signer{keyID: keyID, priv: priv, pub: pub}
}

func (s *ed25519Signer) Sign(digest []byte) ([]byte, error) {
	// ed25519 signs the message directly (no external hashing step required by the library).
	sig := ed25519.Sign(s.priv, digest)
	return sig, nil
}

func (s *ed25519Signer) Verify(digest, signature []byte) bool {
	return ed25519.Verify(s.pub, digest, signature)
}

func (s *ed25519Signer) KeyID() string       { return s.keyID }
func (s *ed25519Signer) Algorithm() string   { return "ed25519" }
func (s *ed25519Signer) PublicKeyBytes() []byte {
	out := make([]byte, len(s.pub))
	copy(out, s.pub)
	return out
}
