package pqaudit

// AES-256-GCM envelope encryption following the DEK/KEK pattern described in
// AWS KMS documentation (https://docs.aws.amazon.com/kms/latest/developerguide/concepts.html#enveloped_encryption):
//
//  1. Generate a fresh 256-bit Data Encryption Key (DEK) per plaintext.
//  2. Encrypt the plaintext with the DEK using AES-256-GCM (authenticated encryption).
//  3. Wrap (encrypt) the DEK with the Key Encryption Key (KEK) using AES-KeyWrap (RFC 3394).
//  4. Store: wrapped DEK + GCM nonce + GCM ciphertext together as an Envelope.
//
// The Additional Data (AD) parameter binds the envelope to its logical context
// (e.g., "entry-seq-5"), preventing a valid envelope from being replayed at a
// different position in the audit chain.
//
// RFC 3161 (trusted timestamping) is NOT implemented here but can be layered on
// top: a TSA would counter-sign the EntryHash, providing non-repudiation with a
// trusted third-party clock. Document the hook in Config.TSAEndpoint if needed.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// aesKeyWrapBlockSize is the 64-bit semi-block size used in RFC 3394 AES-KeyWrap.
const aesKeyWrapBlockSize = 8

// wrapIV is the default initial value defined in RFC 3394 §2.2.3.
var wrapIV = [8]byte{0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6}

// Envelope holds everything needed to decrypt a sealed plaintext.
// All fields are required; zero values indicate a corrupt or missing envelope.
type Envelope struct {
	WrappedDEK []byte `json:"wrapped_dek"` // AES-KeyWrap(KEK, DEK)
	Nonce      []byte `json:"nonce"`       // 12-byte GCM nonce
	Ciphertext []byte `json:"ciphertext"`  // AES-256-GCM output (includes 16-byte auth tag)
	KEKKeyID   string `json:"kek_key_id"`  // identifies which KEK was used
}

// KEK is the Key Encryption Key interface. Real deployments plug in a KMS or
// HSM-backed implementation. Wrap/Unwrap must be inverse operations.
type KEK interface {
	Wrap(dek []byte) (wrapped []byte, err error)
	Unwrap(wrapped []byte) (dek []byte, err error)
	KeyID() string
}

// inMemoryKEK holds a 256-bit AES KEK in process memory.
type inMemoryKEK struct {
	id  string
	key []byte // 32 bytes
}

// NewInMemoryKEK generates a random 256-bit KEK suitable for single-process use.
func NewInMemoryKEK(id string) (KEK, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("pqaudit: generate KEK: %w", err)
	}
	return &inMemoryKEK{id: id, key: key}, nil
}

// LoadInMemoryKEK wraps an existing 32-byte key as a KEK.
func LoadInMemoryKEK(id string, keyBytes []byte) KEK {
	k := make([]byte, len(keyBytes))
	copy(k, keyBytes)
	return &inMemoryKEK{id: id, key: k}
}

func (k *inMemoryKEK) KeyID() string { return k.id }

// Wrap encrypts dek using AES-KeyWrap (RFC 3394).
func (k *inMemoryKEK) Wrap(dek []byte) ([]byte, error) {
	return aesKeyWrap(k.key, dek)
}

// Unwrap decrypts a wrapped DEK using AES-KeyWrap (RFC 3394).
func (k *inMemoryKEK) Unwrap(wrapped []byte) ([]byte, error) {
	return aesKeyUnwrap(k.key, wrapped)
}

// Encrypt seals plaintext with a freshly generated DEK using AES-256-GCM.
// ad (additional data) is authenticated but not encrypted — use it to bind the
// envelope to its audit entry (e.g., fmt.Sprintf("entry-seq-%d", seq)).
func Encrypt(kek KEK, plaintext []byte, ad []byte) (*Envelope, error) {
	// 1. Generate fresh 256-bit DEK.
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, fmt.Errorf("pqaudit: generate DEK: %w", err)
	}

	// 2. Wrap DEK with KEK.
	wrappedDEK, err := kek.Wrap(dek)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: wrap DEK: %w", err)
	}

	// 3. Seal plaintext with DEK via AES-256-GCM.
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: new GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("pqaudit: generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, ad)

	return &Envelope{
		WrappedDEK: wrappedDEK,
		Nonce:      nonce,
		Ciphertext: ciphertext,
		KEKKeyID:   kek.KeyID(),
	}, nil
}

// Decrypt reverses Encrypt. Returns an error if the KEK cannot unwrap the DEK,
// the ciphertext is tampered, or the additional data does not match.
func Decrypt(kek KEK, env *Envelope, ad []byte) ([]byte, error) {
	if env == nil {
		return nil, errors.New("pqaudit: nil envelope")
	}

	// 1. Unwrap DEK.
	dek, err := kek.Unwrap(env.WrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: unwrap DEK: %w", err)
	}

	// 2. Open ciphertext.
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: new GCM: %w", err)
	}
	plaintext, err := gcm.Open(nil, env.Nonce, env.Ciphertext, ad)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: decrypt: %w", err)
	}
	return plaintext, nil
}

// ---------------------------------------------------------------------------
// AES-KeyWrap (RFC 3394) — no external dependency required.
// ---------------------------------------------------------------------------

// aesKeyWrap wraps plainKey with wrappingKey per RFC 3394.
// plainKey must be a multiple of 8 bytes. Returns wrapped output of len(plainKey)+8.
func aesKeyWrap(wrappingKey, plainKey []byte) ([]byte, error) {
	if len(plainKey)%aesKeyWrapBlockSize != 0 {
		return nil, errors.New("pqaudit: key to wrap must be a multiple of 8 bytes")
	}
	block, err := aes.NewCipher(wrappingKey)
	if err != nil {
		return nil, err
	}

	n := len(plainKey) / aesKeyWrapBlockSize
	// R[0..n-1] = plainKey split into 8-byte blocks
	r := make([][]byte, n)
	for i := range r {
		r[i] = make([]byte, aesKeyWrapBlockSize)
		copy(r[i], plainKey[i*aesKeyWrapBlockSize:])
	}

	a := wrapIV // A = default IV
	buf := make([]byte, 16)

	for j := 0; j < 6; j++ {
		for i := 0; i < n; i++ {
			copy(buf[:8], a[:])
			copy(buf[8:], r[i])
			block.Encrypt(buf, buf)
			t := uint64(n*j+i+1)
			a[0] = buf[0] ^ byte(t>>56)
			a[1] = buf[1] ^ byte(t>>48)
			a[2] = buf[2] ^ byte(t>>40)
			a[3] = buf[3] ^ byte(t>>32)
			a[4] = buf[4] ^ byte(t>>24)
			a[5] = buf[5] ^ byte(t>>16)
			a[6] = buf[6] ^ byte(t>>8)
			a[7] = buf[7] ^ byte(t)
			copy(r[i], buf[8:])
		}
	}

	out := make([]byte, (n+1)*aesKeyWrapBlockSize)
	copy(out[:8], a[:])
	for i, ri := range r {
		copy(out[(i+1)*aesKeyWrapBlockSize:], ri)
	}
	return out, nil
}

// aesKeyUnwrap reverses aesKeyWrap. Returns an error if the integrity check fails.
func aesKeyUnwrap(wrappingKey, wrapped []byte) ([]byte, error) {
	if len(wrapped) < 16 || len(wrapped)%aesKeyWrapBlockSize != 0 {
		return nil, errors.New("pqaudit: invalid wrapped key length")
	}
	block, err := aes.NewCipher(wrappingKey)
	if err != nil {
		return nil, err
	}

	n := len(wrapped)/aesKeyWrapBlockSize - 1
	r := make([][]byte, n)
	for i := range r {
		r[i] = make([]byte, aesKeyWrapBlockSize)
		copy(r[i], wrapped[(i+1)*aesKeyWrapBlockSize:])
	}
	var a [8]byte
	copy(a[:], wrapped[:8])

	buf := make([]byte, 16)
	for j := 5; j >= 0; j-- {
		for i := n - 1; i >= 0; i-- {
			t := uint64(n*j + i + 1)
			a[0] ^= byte(t >> 56)
			a[1] ^= byte(t >> 48)
			a[2] ^= byte(t >> 40)
			a[3] ^= byte(t >> 32)
			a[4] ^= byte(t >> 24)
			a[5] ^= byte(t >> 16)
			a[6] ^= byte(t >> 8)
			a[7] ^= byte(t)
			copy(buf[:8], a[:])
			copy(buf[8:], r[i])
			block.Decrypt(buf, buf)
			copy(a[:], buf[:8])
			copy(r[i], buf[8:])
		}
	}

	if a != wrapIV {
		return nil, errors.New("pqaudit: key unwrap integrity check failed — wrong KEK or corrupted data")
	}

	out := make([]byte, n*aesKeyWrapBlockSize)
	for i, ri := range r {
		copy(out[i*aesKeyWrapBlockSize:], ri)
	}
	return out, nil
}
