package federated

import (
	"errors"
	"math/rand/v2"
)

// shamirPrime is the Mersenne prime 2^61 - 1, used as the field modulus.
// All Shamir arithmetic is performed mod this value.
// Secrets larger than this prime must be chunked before splitting.
const shamirPrime uint64 = (1 << 61) - 1

// Share is one Shamir share of a secret: (x, f(x)) where x is the evaluation
// point and y = f(x) is the polynomial evaluated at x, both mod shamirPrime.
type Share struct {
	X uint64
	Y uint64
}

// ShamirSplit splits secret into n shares with threshold t (any t shares can
// reconstruct). Uses a random polynomial of degree t-1 over GF(shamirPrime).
//
// Returns the n shares and the prime used (shamirPrime). secret must be < prime.
func ShamirSplit(secret uint64, t, n int, rng *rand.Rand) ([]Share, uint64, error) {
	if t < 2 {
		return nil, 0, errors.New("shamir: threshold t must be >= 2")
	}
	if n < t {
		return nil, 0, errors.New("shamir: n must be >= t")
	}
	if secret >= shamirPrime {
		return nil, 0, errors.New("shamir: secret must be < prime (2^61-1); chunk large secrets")
	}

	// Build a random polynomial f of degree t-1 with f(0) = secret.
	// coeffs[0] = secret (constant term), coeffs[1..t-1] = random coefficients.
	coeffs := make([]uint64, t)
	coeffs[0] = secret
	for i := 1; i < t; i++ {
		// Random coefficient in [1, prime-1].
		coeffs[i] = (rng.Uint64() % (shamirPrime - 1)) + 1
	}

	shares := make([]Share, n)
	for i := 0; i < n; i++ {
		x := uint64(i + 1) // evaluation points 1..n
		shares[i] = Share{X: x, Y: evalPoly(coeffs, x)}
	}
	return shares, shamirPrime, nil
}

// evalPoly evaluates the polynomial with given coefficients at x over GF(shamirPrime)
// using Horner's method: f(x) = c0 + c1*x + c2*x^2 + ...
func evalPoly(coeffs []uint64, x uint64) uint64 {
	result := uint64(0)
	for i := len(coeffs) - 1; i >= 0; i-- {
		result = addMod61(mulMod61(result, x), coeffs[i])
	}
	return result
}

// ShamirReconstruct recovers the secret from any subset of >= t shares using
// Lagrange interpolation at x=0 over GF(prime).
// prime must be the value returned by ShamirSplit (shamirPrime).
// Returns an error if fewer than 2 shares are provided.
func ShamirReconstruct(shares []Share, prime uint64) (uint64, error) {
	if len(shares) < 2 {
		return 0, errors.New("shamir: need at least 2 shares to reconstruct")
	}
	// Lagrange interpolation at x=0:
	// f(0) = Σ_i y_i * Π_{j≠i} (-x_j) / (x_i - x_j)  (mod prime)
	secret := uint64(0)
	for i, si := range shares {
		num := uint64(1)
		den := uint64(1)
		for j, sj := range shares {
			if i == j {
				continue
			}
			// num *= (0 - x_j) = prime - x_j  (mod prime)
			num = mulMod61(num, prime-sj.X%prime)
			// den *= (x_i - x_j) mod prime
			den = mulMod61(den, subMod61(si.X, sj.X))
		}
		inv := powMod61(den, prime-2) // Fermat's little theorem: den^{p-2} = den^{-1}
		term := mulMod61(si.Y, mulMod61(num, inv))
		secret = addMod61(secret, term)
	}
	return secret, nil
}

// addMod61 computes (a + b) mod (2^61-1).
// Both a and b must be < shamirPrime.
func addMod61(a, b uint64) uint64 {
	sum := a + b
	if sum >= shamirPrime {
		sum -= shamirPrime
	}
	return sum
}

// subMod61 computes (a - b) mod (2^61-1), handling underflow.
func subMod61(a, b uint64) uint64 {
	a = a % shamirPrime
	b = b % shamirPrime
	if a >= b {
		return a - b
	}
	return shamirPrime - (b - a)
}

// mulMod61 computes a*b mod (2^61-1) using schoolbook 128-bit multiply.
// Both a and b must be < shamirPrime (< 2^61).
func mulMod61(a, b uint64) uint64 {
	// Schoolbook: split a into 32-bit halves to avoid overflow.
	aLo, aHi := a&0xFFFFFFFF, a>>32
	bLo, bHi := b&0xFFFFFFFF, b>>32

	// Four partial products.
	p00 := aLo * bLo // bits [0, 63]
	p01 := aLo * bHi // bits [32, 95]
	p10 := aHi * bLo // bits [32, 95]
	p11 := aHi * bHi // bits [64, 127]

	// Combine into 128-bit [hi, lo].
	mid := (p00 >> 32) + (p01 & 0xFFFFFFFF) + (p10 & 0xFFFFFFFF)
	lo := (p00 & 0xFFFFFFFF) | (mid << 32)
	hi := p11 + (p01 >> 32) + (p10 >> 32) + (mid >> 32)

	// Reduce [hi, lo] mod (2^61-1).
	// Since a,b < 2^61: product < 2^122.
	// hi < 2^58 (product < 2^122, hi = product >> 64 < 2^58).
	// hi * 2^64 ≡ hi * 8 (mod 2^61-1) because 2^64 = 2^61 * 8 ≡ 8.
	// lo splits into lo_low (bits 0..60) and lo_high (bits 61..63).
	// lo_high * 2^61 ≡ lo_high (mod 2^61-1).

	loLow := lo & shamirPrime
	loHigh := lo >> 61
	hiContrib := hi * 8 // hi < 2^58, hi*8 < 2^61 — fits in uint64

	sum := loLow + loHigh + hiContrib
	// sum < (2^61-1) + 7 + (2^61-8) = 2*(2^61-1) — one more reduction needed.
	sum = (sum >> 61) + (sum & shamirPrime)
	if sum >= shamirPrime {
		sum -= shamirPrime
	}
	return sum
}

// powMod61 computes base^exp mod (2^61-1) via fast exponentiation.
func powMod61(base, exp uint64) uint64 {
	result := uint64(1)
	base = base % shamirPrime
	for exp > 0 {
		if exp&1 == 1 {
			result = mulMod61(result, base)
		}
		base = mulMod61(base, base)
		exp >>= 1
	}
	return result
}
