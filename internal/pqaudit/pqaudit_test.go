package pqaudit

import (
	"bytes"
	"sync"
	"testing"
)

func newTestLedger(t *testing.T) *Ledger {
	t.Helper()
	s, err := NewEd25519Signer("test-key")
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}
	l, err := New(Config{Signer: s})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return l
}

func appendN(t *testing.T, l *Ledger, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		_, err := l.Append("action", "actor", "target", "detail", true)
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
}

func TestAppendAndVerify_Clean(t *testing.T) {
	l := newTestLedger(t)
	appendN(t, l, 10)
	if l.Count() != 10 {
		t.Fatalf("want 10 entries, got %d", l.Count())
	}
	ok, issues := l.Verify()
	if !ok || len(issues) != 0 {
		t.Fatalf("expected clean ledger, got issues: %v", issues)
	}
}

func TestChainLinkedCorrectly(t *testing.T) {
	l := newTestLedger(t)
	appendN(t, l, 5)
	entries := l.Entries()
	for i := 1; i < len(entries); i++ {
		if !bytes.Equal(entries[i].PrevHash, entries[i-1].EntryHash) {
			t.Errorf("entry %d: PrevHash != prior EntryHash", i)
		}
	}
	// genesis prev hash must be all zeros
	if !bytes.Equal(entries[0].PrevHash, make([]byte, 32)) {
		t.Error("genesis entry PrevHash is not zero")
	}
}

func TestTamperDetected_FieldMutation(t *testing.T) {
	l := newTestLedger(t)
	appendN(t, l, 10)

	// mutate Action of entry at index 4 (Seq 5) directly via unexported field
	l.mu.Lock()
	l.entries[4].Action = "tampered"
	l.mu.Unlock()

	ok, issues := l.Verify()
	if ok {
		t.Fatal("expected Verify to return false after field mutation")
	}
	found := false
	for _, iss := range issues {
		if iss.Seq == 5 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected issue at Seq 5, got: %v", issues)
	}
}

func TestTamperDetected_HashMutation(t *testing.T) {
	l := newTestLedger(t)
	appendN(t, l, 5)

	// flip a byte in EntryHash of entry 2 (index 2)
	l.mu.Lock()
	l.entries[2].EntryHash[0] ^= 0xFF
	l.mu.Unlock()

	ok, issues := l.Verify()
	if ok {
		t.Fatal("expected Verify to return false after hash mutation")
	}
	// entry 3 should fail PrevHash linkage (cascades from 2), or entry 2 itself fails SignatureMismatch
	found := false
	for _, iss := range issues {
		if iss.Seq == 3 || iss.Seq == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected issue near Seq 2 or 3, got: %v", issues)
	}
}

func TestTamperDetected_SignatureMutation(t *testing.T) {
	l := newTestLedger(t)
	appendN(t, l, 5)

	// flip a byte in the signature of entry 1 (index 1)
	l.mu.Lock()
	l.entries[1].Signature[0] ^= 0xFF
	l.mu.Unlock()

	ok, issues := l.Verify()
	if ok {
		t.Fatal("expected Verify to return false after signature mutation")
	}
	found := false
	for _, iss := range issues {
		if iss.Seq == 2 && iss.Reason == "SignatureMismatch" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected SignatureMismatch at Seq 2, got: %v", issues)
	}
}

func TestMerkleRootStable(t *testing.T) {
	l := newTestLedger(t)
	appendN(t, l, 8)
	r1 := l.MerkleRoot()
	r2 := l.MerkleRoot()
	if !bytes.Equal(r1, r2) {
		t.Error("MerkleRoot is not deterministic")
	}
}

func TestMerkleProofRoundtrip(t *testing.T) {
	l := newTestLedger(t)
	appendN(t, l, 8)

	entries := l.Entries()
	hashes := make([][]byte, len(entries))
	for i, e := range entries {
		hashes[i] = e.EntryHash
	}

	root := MerkleRoot(hashes)
	proof, err := MerkleProof(hashes, 3)
	if err != nil {
		t.Fatalf("MerkleProof: %v", err)
	}
	if !VerifyMerkleProof(hashes[3], proof, 3, root) {
		t.Error("VerifyMerkleProof returned false for a valid proof")
	}
}

func TestExportImportRoundtrip(t *testing.T) {
	s, err := NewEd25519Signer("roundtrip-key")
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}
	l1, err := New(Config{Signer: s})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	appendN(t, l1, 5)

	var buf bytes.Buffer
	if err := l1.ExportJSON(&buf); err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	// Import into a new ledger using the SAME signer (same pub key for verification).
	l2, err := New(Config{Signer: s})
	if err != nil {
		t.Fatalf("New l2: %v", err)
	}
	if err := l2.ImportJSON(&buf); err != nil {
		t.Fatalf("ImportJSON: %v", err)
	}

	ok, issues := l2.Verify()
	if !ok {
		t.Fatalf("imported ledger failed verification: %v", issues)
	}
	if l2.Count() != 5 {
		t.Fatalf("want 5 entries after import, got %d", l2.Count())
	}
}

func TestConcurrentAppends(t *testing.T) {
	l := newTestLedger(t)
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := l.Append("concurrent", "goroutine", "ledger", "stress", true)
			if err != nil {
				t.Errorf("concurrent Append: %v", err)
			}
		}()
	}
	wg.Wait()

	if l.Count() != n {
		t.Fatalf("want %d entries, got %d", n, l.Count())
	}

	// Seq values must be 1..n unique
	entries := l.Entries()
	seen := make(map[uint64]bool, n)
	for _, e := range entries {
		if seen[e.Seq] {
			t.Errorf("duplicate Seq %d", e.Seq)
		}
		seen[e.Seq] = true
	}
	for s := uint64(1); s <= n; s++ {
		if !seen[s] {
			t.Errorf("missing Seq %d", s)
		}
	}

	ok, issues := l.Verify()
	if !ok {
		t.Fatalf("concurrent ledger failed verification: %v", issues)
	}
}
