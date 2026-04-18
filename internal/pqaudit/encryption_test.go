package pqaudit

import (
	"bytes"
	"testing"
)

func newTestKEK(t *testing.T) KEK {
	t.Helper()
	kek, err := NewInMemoryKEK("test-kek")
	if err != nil {
		t.Fatalf("NewInMemoryKEK: %v", err)
	}
	return kek
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	kek := newTestKEK(t)
	plaintext := []byte("sensitive audit detail")
	ad := []byte("entry-seq-1")

	env, err := Encrypt(kek, plaintext, ad)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if env == nil {
		t.Fatal("Encrypt returned nil envelope")
	}

	got, err := Decrypt(kek, env, ad)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("roundtrip mismatch: got %q, want %q", got, plaintext)
	}
}

func TestEncrypt_DifferentNoncesEachCall(t *testing.T) {
	kek := newTestKEK(t)
	plaintext := []byte("same plaintext")
	ad := []byte("entry-seq-2")

	env1, err := Encrypt(kek, plaintext, ad)
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	env2, err := Encrypt(kek, plaintext, ad)
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}

	if bytes.Equal(env1.Nonce, env2.Nonce) {
		t.Error("expected different nonces on each Encrypt call")
	}
	if bytes.Equal(env1.Ciphertext, env2.Ciphertext) {
		t.Error("expected different ciphertexts on each Encrypt call (different DEK+nonce)")
	}
}

func TestDecrypt_TamperedCiphertext_Fails(t *testing.T) {
	kek := newTestKEK(t)
	plaintext := []byte("tamper test")
	ad := []byte("entry-seq-3")

	env, err := Encrypt(kek, plaintext, ad)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Flip the first byte of the ciphertext (includes auth tag at end, but body first).
	tampered := make([]byte, len(env.Ciphertext))
	copy(tampered, env.Ciphertext)
	tampered[0] ^= 0xFF
	env.Ciphertext = tampered

	_, err = Decrypt(kek, env, ad)
	if err == nil {
		t.Error("expected Decrypt to fail on tampered ciphertext, got nil error")
	}
}

func TestDecrypt_WrongKEK_Fails(t *testing.T) {
	kek1 := newTestKEK(t)
	kek2 := newTestKEK(t) // different random key

	env, err := Encrypt(kek1, []byte("secret"), []byte("entry-seq-4"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = Decrypt(kek2, env, []byte("entry-seq-4"))
	if err == nil {
		t.Error("expected Decrypt to fail with wrong KEK, got nil error")
	}
}

func TestEncrypt_ADBinding(t *testing.T) {
	kek := newTestKEK(t)
	plaintext := []byte("bound detail")

	env, err := Encrypt(kek, plaintext, []byte("entry-seq-5"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Decrypting with a different AD must fail.
	_, err = Decrypt(kek, env, []byte("entry-seq-6"))
	if err == nil {
		t.Error("expected Decrypt to fail when AD does not match, got nil error")
	}
}

func TestInMemoryKEK_Roundtrip(t *testing.T) {
	kek := newTestKEK(t)

	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i)
	}

	wrapped, err := kek.Wrap(dek)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	if bytes.Equal(wrapped, dek) {
		t.Error("wrapped key should differ from plaintext DEK")
	}

	unwrapped, err := kek.Unwrap(wrapped)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}
	if !bytes.Equal(unwrapped, dek) {
		t.Error("Unwrap did not recover original DEK")
	}
}
