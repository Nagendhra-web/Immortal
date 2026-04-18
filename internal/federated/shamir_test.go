package federated

import (
	"math/rand/v2"
	"testing"
)

func newTestRng(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed^0xdeadbeef))
}

// TestShamirSplit_ReconstructWithExactThreshold verifies that a secret split
// with t=3, n=5 can be exactly reconstructed from any t shares.
func TestShamirSplit_ReconstructWithExactThreshold(t *testing.T) {
	const secret = uint64(12345)
	const threshold = 3
	const n = 5

	shares, prime, err := ShamirSplit(secret, threshold, n, newTestRng(1))
	if err != nil {
		t.Fatalf("ShamirSplit: %v", err)
	}

	// Use exactly the first 3 shares.
	got, err := ShamirReconstruct(shares[:threshold], prime)
	if err != nil {
		t.Fatalf("ShamirReconstruct: %v", err)
	}
	if got != secret {
		t.Errorf("got %d, want %d", got, secret)
	}
}

// TestShamirSplit_FewerThanThreshold_Fails verifies that fewer than t shares
// return an error (not a correct reconstruction, as that would be insecure).
func TestShamirSplit_FewerThanThreshold_Fails(t *testing.T) {
	const secret = uint64(99999)
	const threshold = 3
	const n = 5

	shares, prime, err := ShamirSplit(secret, threshold, n, newTestRng(2))
	if err != nil {
		t.Fatalf("ShamirSplit: %v", err)
	}

	// Give only 2 shares (fewer than threshold).
	got, err := ShamirReconstruct(shares[:2], prime)
	// ShamirReconstruct succeeds (2 >= minimum of 2) but the recovered value
	// will NOT equal the secret because the polynomial has degree >= 2.
	// With t-1=2 degree polynomial, 2 points do NOT uniquely determine f(0).
	// Document: we verify the value is wrong (not equal to secret) with high probability.
	// For the error path: test 1 share.
	_, err1 := ShamirReconstruct(shares[:1], prime)
	if err1 == nil {
		t.Error("expected error with 1 share, got nil")
	}

	// With 2 shares and a degree-2 polynomial, result should differ from secret
	// (probability of collision ≈ 1/prime ≈ 2^{-61}).
	if err == nil && got == secret {
		t.Errorf("reconstructed secret with fewer shares than threshold: got %d (should differ)", got)
	}
}

// TestShamirSplit_Randomness verifies that splitting the same secret twice
// produces different shares (due to random polynomial coefficients).
func TestShamirSplit_Randomness(t *testing.T) {
	const secret = uint64(42)
	rng1 := newTestRng(10)
	rng2 := newTestRng(20) // different seed → different polynomial

	shares1, prime1, err1 := ShamirSplit(secret, 3, 5, rng1)
	shares2, prime2, err2 := ShamirSplit(secret, 3, 5, rng2)
	if err1 != nil || err2 != nil {
		t.Fatalf("ShamirSplit errors: %v, %v", err1, err2)
	}
	if prime1 != prime2 {
		t.Errorf("primes differ: %d vs %d", prime1, prime2)
	}

	different := false
	for i := range shares1 {
		if shares1[i].Y != shares2[i].Y {
			different = true
			break
		}
	}
	if !different {
		t.Error("both splits produced identical shares — polynomial randomness not working")
	}
}

// TestShamirReconstruct_AnyThreeOfFive verifies all C(5,3)=10 subsets of
// 3-of-5 shares reconstruct the original secret.
func TestShamirReconstruct_AnyThreeOfFive(t *testing.T) {
	const secret = uint64(987654321)
	const threshold = 3
	const n = 5

	shares, prime, err := ShamirSplit(secret, threshold, n, newTestRng(3))
	if err != nil {
		t.Fatalf("ShamirSplit: %v", err)
	}

	// Test all C(5,3) = 10 subsets.
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for k := j + 1; k < n; k++ {
				subset := []Share{shares[i], shares[j], shares[k]}
				got, err := ShamirReconstruct(subset, prime)
				if err != nil {
					t.Errorf("subset (%d,%d,%d): %v", i, j, k, err)
					continue
				}
				if got != secret {
					t.Errorf("subset (%d,%d,%d): got %d, want %d", i, j, k, got, secret)
				}
			}
		}
	}
}

// TestShamir_LargeSecret_RequiresChunking documents that secrets >= shamirPrime
// are rejected, and callers must chunk large secrets themselves.
// The dropout implementation handles this by splitting each 8-byte seed chunk.
func TestShamir_LargeSecret_RequiresChunking(t *testing.T) {
	// shamirPrime = 2^61-1. A value >= prime must be rejected.
	tooLarge := shamirPrime // exactly equal to prime — must fail
	_, _, err := ShamirSplit(tooLarge, 2, 3, newTestRng(4))
	if err == nil {
		t.Error("expected error for secret >= prime, got nil")
	}

	// Values < prime work fine, including large ones like 2^60.
	large := uint64(1 << 60)
	shares, prime, err := ShamirSplit(large, 2, 3, newTestRng(5))
	if err != nil {
		t.Fatalf("ShamirSplit large valid secret: %v", err)
	}
	got, err := ShamirReconstruct(shares[:2], prime)
	if err != nil {
		t.Fatalf("ShamirReconstruct: %v", err)
	}
	if got != large {
		t.Errorf("got %d, want %d", got, large)
	}
}

// TestShamirSplit_AllNShares verifies reconstruction using all n=5 shares.
func TestShamirSplit_AllNShares(t *testing.T) {
	const secret = uint64(1111111111)
	shares, prime, err := ShamirSplit(secret, 3, 5, newTestRng(6))
	if err != nil {
		t.Fatalf("ShamirSplit: %v", err)
	}
	got, err := ShamirReconstruct(shares, prime)
	if err != nil {
		t.Fatalf("ShamirReconstruct: %v", err)
	}
	if got != secret {
		t.Errorf("got %d, want %d", got, secret)
	}
}

// TestShamirSplit_ZeroSecret verifies the zero secret round-trips.
func TestShamirSplit_ZeroSecret(t *testing.T) {
	shares, prime, err := ShamirSplit(0, 2, 4, newTestRng(7))
	if err != nil {
		t.Fatalf("ShamirSplit: %v", err)
	}
	got, err := ShamirReconstruct(shares[:2], prime)
	if err != nil {
		t.Fatalf("ShamirReconstruct: %v", err)
	}
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

// TestShamirSplit_InvalidParams verifies error cases.
func TestShamirSplit_InvalidParams(t *testing.T) {
	rng := newTestRng(8)
	if _, _, err := ShamirSplit(1, 1, 3, rng); err == nil {
		t.Error("expected error for t=1")
	}
	if _, _, err := ShamirSplit(1, 3, 2, rng); err == nil {
		t.Error("expected error for n < t")
	}
}
