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
}

// VerificationIssue describes a single integrity violation found during Verify.
type VerificationIssue struct {
	Seq    uint64
	Reason string
}

// Config holds Ledger construction parameters.
type Config struct {
	Signer     Signer
	MaxEntries int // 0 = unbounded
}

// Ledger is a cryptographically chained, signed audit ledger.
type Ledger struct {
	mu         sync.RWMutex
	entries    []Entry
	maxEntries int
	signer     Signer
	seq        uint64
}

// New creates a new Ledger. Signer must not be nil.
func New(cfg Config) (*Ledger, error) {
	if cfg.Signer == nil {
		return nil, errors.New("pqaudit: signer must not be nil")
	}
	return &Ledger{
		entries:    make([]Entry, 0),
		maxEntries: cfg.MaxEntries,
		signer:     cfg.Signer,
	}, nil
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
	return &out, nil
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
func canonicalBytes(e *Entry) ([]byte, error) {
	type canonical struct {
		Seq       uint64 `json:"seq"`
		Timestamp string `json:"timestamp"`
		Action    string `json:"action"`
		Actor     string `json:"actor"`
		Target    string `json:"target"`
		Detail    string `json:"detail"`
		Success   bool   `json:"success"`
		PrevHash  string `json:"prev_hash"`
		KeyID     string `json:"key_id"`
		Algorithm string `json:"algorithm"`
	}
	c := canonical{
		Seq:       e.Seq,
		Timestamp: e.Timestamp.UTC().Format(time.RFC3339Nano),
		Action:    e.Action,
		Actor:     e.Actor,
		Target:    e.Target,
		Detail:    e.Detail,
		Success:   e.Success,
		PrevHash:  hex.EncodeToString(e.PrevHash),
		KeyID:     e.KeyID,
		Algorithm: e.Algorithm,
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
