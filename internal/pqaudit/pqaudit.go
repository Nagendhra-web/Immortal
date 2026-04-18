package pqaudit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// envelopeJSON returns a stable JSON encoding of env used as additional input
// to the canonical hash when an entry is encrypted.
func envelopeJSON(env *Envelope) ([]byte, error) {
	return json.Marshal(env)
}

// Entry is a single tamper-evident audit record, linked to its predecessor.
type Entry struct {
	Seq       uint64    `json:"seq"`
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Actor     string    `json:"actor"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	Success   bool      `json:"success"`
	PrevHash  []byte    `json:"prev_hash"`  // SHA-256 of previous Entry canonical bytes
	EntryHash []byte    `json:"entry_hash"` // SHA-256 of this Entry's canonical bytes (excluding Signature, EntryHash)
	Signature []byte    `json:"signature"`  // Signer.Sign(EntryHash)
	KeyID     string    `json:"key_id"`
	Algorithm string    `json:"algorithm"`

	// EncryptedDetail is set by AppendEncrypted. When non-nil, Detail is empty
	// in the stored entry; the canonical hash covers the envelope's JSON hash
	// so the integrity chain still works without the plaintext.
	EncryptedDetail     *Envelope `json:"encrypted_detail,omitempty"`
	// EncryptedDetailHash is sha256(JSON(EncryptedDetail)) in hex, included in
	// canonical bytes when EncryptedDetail is present so tampering breaks the chain.
	EncryptedDetailHash string    `json:"encrypted_detail_hash,omitempty"`
}

// VerificationIssue describes a single integrity violation found during Verify.
type VerificationIssue struct {
	Seq    uint64
	Reason string
}

// Config holds Ledger construction parameters.
type Config struct {
	Signer     Signer
	MaxEntries int    // 0 = unbounded
	KEK        KEK    // optional: if set, AppendEncrypted encrypts Detail
	WORMPath   string // optional: if set, every Append is fsynced to this file
}

// Ledger is a cryptographically chained, signed audit ledger.
type Ledger struct {
	mu         sync.RWMutex
	entries    []Entry
	maxEntries int
	signer     Signer
	seq        uint64
	kek        KEK
	worm       *WORMWriter
}

// New creates a new Ledger. Signer must not be nil.
func New(cfg Config) (*Ledger, error) {
	if cfg.Signer == nil {
		return nil, errors.New("pqaudit: signer must not be nil")
	}
	l := &Ledger{
		entries:    make([]Entry, 0),
		maxEntries: cfg.MaxEntries,
		signer:     cfg.Signer,
		kek:        cfg.KEK,
	}
	if cfg.WORMPath != "" {
		w, err := NewWORMWriter(cfg.WORMPath)
		if err != nil {
			return nil, fmt.Errorf("pqaudit: open WORM: %w", err)
		}
		l.worm = w
	}
	return l, nil
}

// Close releases resources held by the Ledger (e.g., the WORM file handle).
// It is safe to call Close on a Ledger without a WORMPath.
func (l *Ledger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.worm != nil {
		return l.worm.Close()
	}
	return nil
}

// Append adds a new entry to the ledger and returns it.
func (l *Ledger) Append(action, actor, target, detail string, success bool) (*Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.seq++
	seq := l.seq

	var prevHash []byte
	if len(l.entries) == 0 {
		prevHash = make([]byte, 32) // zeros for genesis
	} else {
		prev := l.entries[len(l.entries)-1]
		prevHash = make([]byte, len(prev.EntryHash))
		copy(prevHash, prev.EntryHash)
	}

	e := Entry{
		Seq:       seq,
		Timestamp: time.Now().UTC(),
		Action:    action,
		Actor:     actor,
		Target:    target,
		Detail:    detail,
		Success:   success,
		PrevHash:  prevHash,
		KeyID:     l.signer.KeyID(),
		Algorithm: l.signer.Algorithm(),
	}

	canonical, err := canonicalBytes(&e)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: canonical encoding: %w", err)
	}
	sum := sha256.Sum256(canonical)
	e.EntryHash = sum[:]

	sig, err := l.signer.Sign(e.EntryHash)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: signing: %w", err)
	}
	e.Signature = sig

	l.entries = append(l.entries, e)
	if l.maxEntries > 0 && len(l.entries) > l.maxEntries {
		l.entries = l.entries[len(l.entries)-l.maxEntries:]
	}

	out := l.entries[len(l.entries)-1]

	if l.worm != nil {
		if werr := l.worm.Append(&out); werr != nil {
			return &out, fmt.Errorf("pqaudit: WORM append: %w", werr)
		}
	}

	return &out, nil
}

// AppendEncrypted is like Append but stores Detail as an AES-256-GCM encrypted
// Envelope. The integrity chain still links correctly; anyone without the KEK
// cannot read Detail. Returns an error if no KEK is configured.
//
// Canonical bytes for the hash use Detail="" and include EncryptedDetailHash
// (sha256 of the envelope JSON in hex), so tampering with the envelope breaks
// the chain even without access to the plaintext.
func (l *Ledger) AppendEncrypted(action, actor, target, detail string, success bool) (*Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.kek == nil {
		return nil, errors.New("pqaudit: AppendEncrypted requires a KEK in Config")
	}

	l.seq++
	seq := l.seq

	var prevHash []byte
	if len(l.entries) == 0 {
		prevHash = make([]byte, 32)
	} else {
		prev := l.entries[len(l.entries)-1]
		prevHash = make([]byte, len(prev.EntryHash))
		copy(prevHash, prev.EntryHash)
	}

	// Additional data binds the envelope to this specific entry sequence number,
	// preventing a valid envelope from being replayed at a different position.
	ad := []byte(fmt.Sprintf("entry-seq-%d", seq))

	env, err := Encrypt(l.kek, []byte(detail), ad)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: encrypt detail: %w", err)
	}

	envJSON, err := envelopeJSON(env)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: envelope json: %w", err)
	}
	envHash := sha256.Sum256(envJSON)

	e := Entry{
		Seq:                 seq,
		Timestamp:           time.Now().UTC(),
		Action:              action,
		Actor:               actor,
		Target:              target,
		Detail:              "", // redacted; plaintext only in EncryptedDetail
		Success:             success,
		PrevHash:            prevHash,
		KeyID:               l.signer.KeyID(),
		Algorithm:           l.signer.Algorithm(),
		EncryptedDetail:     env,
		EncryptedDetailHash: hex.EncodeToString(envHash[:]),
	}

	canonical, err := canonicalBytes(&e)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: canonical encoding: %w", err)
	}
	sum := sha256.Sum256(canonical)
	e.EntryHash = sum[:]

	sig, err := l.signer.Sign(e.EntryHash)
	if err != nil {
		return nil, fmt.Errorf("pqaudit: signing: %w", err)
	}
	e.Signature = sig

	l.entries = append(l.entries, e)
	if l.maxEntries > 0 && len(l.entries) > l.maxEntries {
		l.entries = l.entries[len(l.entries)-l.maxEntries:]
	}

	out := l.entries[len(l.entries)-1]

	if l.worm != nil {
		if werr := l.worm.Append(&out); werr != nil {
			return &out, fmt.Errorf("pqaudit: WORM append: %w", werr)
		}
	}

	return &out, nil
}

// DecryptDetail returns the plaintext Detail for an encrypted entry.
// Returns an error if:
//   - the entry has no EncryptedDetail
//   - the Ledger has no KEK configured
//   - the KEK cannot unwrap the DEK (wrong key)
//   - the ciphertext is tampered
func (l *Ledger) DecryptDetail(e *Entry) (string, error) {
	if e.EncryptedDetail == nil {
		return "", errors.New("pqaudit: entry is not encrypted")
	}
	if l.kek == nil {
		return "", errors.New("pqaudit: no KEK configured — cannot decrypt")
	}
	ad := []byte(fmt.Sprintf("entry-seq-%d", e.Seq))
	plaintext, err := Decrypt(l.kek, e.EncryptedDetail, ad)
	if err != nil {
		return "", fmt.Errorf("pqaudit: DecryptDetail: %w", err)
	}
	return string(plaintext), nil
}

// Entries returns a snapshot copy of all entries in order (oldest first).
func (l *Ledger) Entries() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	cp := make([]Entry, len(l.entries))
	copy(cp, l.entries)
	return cp
}

// Count returns the number of stored entries.
func (l *Ledger) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// Head returns the EntryHash of the most recent entry, or nil if empty.
func (l *Ledger) Head() []byte {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if len(l.entries) == 0 {
		return nil
	}
	h := l.entries[len(l.entries)-1].EntryHash
	out := make([]byte, len(h))
	copy(out, h)
	return out
}

// Verify checks the entire chain for hash and signature integrity.
// Returns (true, nil) when clean, (false, issues) when tampered.
func (l *Ledger) Verify() (bool, []VerificationIssue) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var issues []VerificationIssue

	for i, e := range l.entries {
		// 1. PrevHash linkage
		var expectedPrev []byte
		if i == 0 {
			expectedPrev = make([]byte, 32)
		} else {
			expectedPrev = l.entries[i-1].EntryHash
		}
		if !bytes.Equal(e.PrevHash, expectedPrev) {
			issues = append(issues, VerificationIssue{
				Seq:    e.Seq,
				Reason: "PrevHashMismatch",
			})
			continue // subsequent checks would cascade-fail
		}

		// 2. EntryHash matches canonical bytes
		canonical, err := canonicalBytes(&e)
		if err != nil {
			issues = append(issues, VerificationIssue{Seq: e.Seq, Reason: "CanonicalEncodeError"})
			continue
		}
		sum := sha256.Sum256(canonical)
		if !bytes.Equal(e.EntryHash, sum[:]) {
			issues = append(issues, VerificationIssue{
				Seq:    e.Seq,
				Reason: "EntryHashMismatch",
			})
			continue
		}

		// 3. Signature over EntryHash
		if !l.signer.Verify(e.EntryHash, e.Signature) {
			issues = append(issues, VerificationIssue{
				Seq:    e.Seq,
				Reason: "SignatureMismatch",
			})
		}
	}

	return len(issues) == 0, issues
}

// MerkleRoot returns the Merkle root over all entries. Each leaf is recomputed
// from canonical bytes so that any field mutation (even without touching
// EntryHash) propagates into the root and is detectable by a verifier.
func (l *Ledger) MerkleRoot() []byte {
	l.mu.RLock()
	defer l.mu.RUnlock()
	hashes := make([][]byte, len(l.entries))
	for i, e := range l.entries {
		canonical, err := canonicalBytes(&e)
		if err != nil {
			hashes[i] = make([]byte, 32)
			continue
		}
		sum := sha256.Sum256(canonical)
		hashes[i] = sum[:]
	}
	return MerkleRoot(hashes)
}

// exportPayload is the JSON envelope written by ExportJSON.
type exportPayload struct {
	Entries []Entry `json:"entries"`
}

// ExportJSON writes the full ledger as JSON to w.
func (l *Ledger) ExportJSON(w io.Writer) error {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return json.NewEncoder(w).Encode(exportPayload{Entries: l.entries})
}

// ImportJSON reads a ledger previously written by ExportJSON, replaces the
// current entries, and runs Verify. Returns an error if verification fails.
func (l *Ledger) ImportJSON(r io.Reader) error {
	var payload exportPayload
	if err := json.NewDecoder(r).Decode(&payload); err != nil {
		return fmt.Errorf("pqaudit: decode: %w", err)
	}

	l.mu.Lock()
	l.entries = payload.Entries
	if len(l.entries) > 0 {
		l.seq = l.entries[len(l.entries)-1].Seq
	}
	l.mu.Unlock()

	ok, issues := l.Verify()
	if !ok {
		return fmt.Errorf("pqaudit: import verification failed: %v", issues)
	}
	return nil
}

// canonicalBytes produces the deterministic byte representation of an entry
// used as the pre-image for EntryHash. Signature and EntryHash are excluded.
//
// For encrypted entries (EncryptedDetail != nil), Detail is always "" and
// EncryptedDetailHash (sha256 of the envelope JSON in hex) is included so that
// tampering with the envelope breaks the hash chain even without the plaintext.
func canonicalBytes(e *Entry) ([]byte, error) {
	type canonical struct {
		Seq                 uint64 `json:"seq"`
		Timestamp           string `json:"timestamp"`
		Action              string `json:"action"`
		Actor               string `json:"actor"`
		Target              string `json:"target"`
		Detail              string `json:"detail"`
		Success             bool   `json:"success"`
		PrevHash            string `json:"prev_hash"`
		KeyID               string `json:"key_id"`
		Algorithm           string `json:"algorithm"`
		EncryptedDetailHash string `json:"encrypted_detail_hash,omitempty"`
	}
	c := canonical{
		Seq:                 e.Seq,
		Timestamp:           e.Timestamp.UTC().Format(time.RFC3339Nano),
		Action:              e.Action,
		Actor:               e.Actor,
		Target:              e.Target,
		Detail:              e.Detail,
		Success:             e.Success,
		PrevHash:            hex.EncodeToString(e.PrevHash),
		KeyID:               e.KeyID,
		Algorithm:           e.Algorithm,
		EncryptedDetailHash: e.EncryptedDetailHash,
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(c); err != nil {
		return nil, err
	}
	// trim trailing newline added by json.Encoder
	b := buf.Bytes()
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return b, nil
}
