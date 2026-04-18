package pqaudit

import (
	"testing"
)

func newTestLedgerWithKEK(t *testing.T) (*Ledger, KEK) {
	t.Helper()
	s, err := NewEd25519Signer("conf-test-key")
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}
	kek, err := NewInMemoryKEK("conf-kek")
	if err != nil {
		t.Fatalf("NewInMemoryKEK: %v", err)
	}
	l, err := New(Config{Signer: s, KEK: kek})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return l, kek
}

func TestAppendEncrypted_ChainAndDecrypt(t *testing.T) {
	l, _ := newTestLedgerWithKEK(t)

	const secret = "top-secret detail"
	e, err := l.AppendEncrypted("login", "alice", "system", secret, true)
	if err != nil {
		t.Fatalf("AppendEncrypted: %v", err)
	}

	// Detail must be redacted in the stored entry.
	if e.Detail != "" {
		t.Errorf("expected Detail to be empty in stored entry, got %q", e.Detail)
	}
	if e.EncryptedDetail == nil {
		t.Fatal("expected EncryptedDetail to be set")
	}
	if e.EncryptedDetailHash == "" {
		t.Error("expected EncryptedDetailHash to be set")
	}

	// Chain must verify cleanly.
	ok, issues := l.Verify()
	if !ok {
		t.Fatalf("Verify failed after AppendEncrypted: %v", issues)
	}

	// Must be able to recover the plaintext with the correct KEK.
	plaintext, err := l.DecryptDetail(e)
	if err != nil {
		t.Fatalf("DecryptDetail: %v", err)
	}
	if plaintext != secret {
		t.Errorf("DecryptDetail: got %q, want %q", plaintext, secret)
	}
}

func TestAppendEncrypted_NoKEK_Fails(t *testing.T) {
	s, err := NewEd25519Signer("no-kek-key")
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}
	l, err := New(Config{Signer: s}) // no KEK
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = l.AppendEncrypted("login", "bob", "system", "detail", true)
	if err == nil {
		t.Error("expected AppendEncrypted to fail when no KEK is configured")
	}
}

func TestAppendEncrypted_AuditorWithoutKEK_CannotDecrypt(t *testing.T) {
	l, _ := newTestLedgerWithKEK(t)

	e, err := l.AppendEncrypted("login", "charlie", "system", "secret", true)
	if err != nil {
		t.Fatalf("AppendEncrypted: %v", err)
	}

	// Build a second ledger with a DIFFERENT KEK (simulating an auditor who has
	// the signer public key but not the data encryption KEK).
	s2, err := NewEd25519Signer("auditor-key")
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}
	wrongKEK, err := NewInMemoryKEK("wrong-kek")
	if err != nil {
		t.Fatalf("NewInMemoryKEK: %v", err)
	}
	l2, err := New(Config{Signer: s2, KEK: wrongKEK})
	if err != nil {
		t.Fatalf("New l2: %v", err)
	}

	_, err = l2.DecryptDetail(e)
	if err == nil {
		t.Error("expected DecryptDetail to fail with wrong KEK")
	}
}
