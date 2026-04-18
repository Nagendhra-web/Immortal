package pqaudit

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func newTestLedgerWithSigner(t *testing.T) (*Ledger, func()) {
	t.Helper()
	s, err := NewEd25519Signer("worm-test-key")
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}
	l, err := New(Config{Signer: s})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return l, func() {}
}

func appendNEntries(t *testing.T, l *Ledger, n int) []*Entry {
	t.Helper()
	out := make([]*Entry, n)
	for i := 0; i < n; i++ {
		e, err := l.Append("action", "actor", "target", "detail", true)
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
		out[i] = e
	}
	return out
}

func TestWORM_AppendReadBack(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.ndjson")

	l, _ := newTestLedgerWithSigner(t)
	entries := appendNEntries(t, l, 10)

	w, err := NewWORMWriter(path)
	if err != nil {
		t.Fatalf("NewWORMWriter: %v", err)
	}
	for _, e := range entries {
		if err := w.Append(e); err != nil {
			t.Fatalf("WORM Append: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("WORMWriter.Close: %v", err)
	}

	r, err := NewWORMReader(path)
	if err != nil {
		t.Fatalf("NewWORMReader: %v", err)
	}
	defer r.Close()

	count := 0
	for {
		e, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("WORMReader.Next: %v", err)
		}
		if e.Seq != entries[count].Seq {
			t.Errorf("entry %d: got Seq %d, want %d", count, e.Seq, entries[count].Seq)
		}
		count++
	}
	if count != 10 {
		t.Errorf("expected 10 entries, got %d", count)
	}
}

func TestWORM_FsyncPerformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.ndjson")

	l, _ := newTestLedgerWithSigner(t)
	e, err := l.Append("action", "actor", "target", "detail", true)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	w, err := NewWORMWriter(path)
	if err != nil {
		t.Fatalf("NewWORMWriter: %v", err)
	}
	if err := w.Append(e); err != nil {
		t.Fatalf("WORM Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("WORMWriter.Close: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Size() == 0 {
		t.Error("expected non-empty ledger file after fsync")
	}
}

func TestWORM_ExclusiveLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.ndjson")

	w1, err := NewWORMWriter(path)
	if err != nil {
		t.Fatalf("first NewWORMWriter: %v", err)
	}
	defer w1.Close()

	_, err = NewWORMWriter(path)
	if err == nil {
		t.Error("expected second NewWORMWriter to fail while first holds the lock")
	}
}

func TestLoadWORM_RebuildsLedger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.ndjson")

	s, err := NewEd25519Signer("load-test-key")
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}
	l, err := New(Config{Signer: s})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	entries := appendNEntries(t, l, 5)

	w, err := NewWORMWriter(path)
	if err != nil {
		t.Fatalf("NewWORMWriter: %v", err)
	}
	for _, e := range entries {
		if err := w.Append(e); err != nil {
			t.Fatalf("WORM Append: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("WORMWriter.Close: %v", err)
	}

	l2, err := LoadWORM(path, Config{Signer: s})
	if err != nil {
		t.Fatalf("LoadWORM: %v", err)
	}
	if l2.Count() != 5 {
		t.Errorf("expected 5 entries, got %d", l2.Count())
	}
	ok, issues := l2.Verify()
	if !ok {
		t.Errorf("loaded ledger failed verification: %v", issues)
	}
}

func TestLoadWORM_DetectsTamperedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.ndjson")

	s, err := NewEd25519Signer("tamper-test-key")
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}
	l, err := New(Config{Signer: s})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	entries := appendNEntries(t, l, 5)

	w, err := NewWORMWriter(path)
	if err != nil {
		t.Fatalf("NewWORMWriter: %v", err)
	}
	for _, e := range entries {
		if err := w.Append(e); err != nil {
			t.Fatalf("WORM Append: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("WORMWriter.Close: %v", err)
	}

	// Corrupt a byte in the middle of the file.
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("open for mutation: %v", err)
	}
	fi, _ := f.Stat()
	mid := fi.Size() / 2
	buf := make([]byte, 1)
	f.ReadAt(buf, mid)
	buf[0] ^= 0xFF
	f.WriteAt(buf, mid)
	f.Close()

	// LoadWORM should fail (bad JSON or verification failure).
	_, err = LoadWORM(path, Config{Signer: s})
	if err == nil {
		t.Error("expected LoadWORM to fail on tampered file")
	}
}
