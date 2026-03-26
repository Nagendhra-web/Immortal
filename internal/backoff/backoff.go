package backoff

import (
	"math"
	"math/rand"
	"time"
)

type Backoff struct {
	Initial    time.Duration
	Max        time.Duration
	Multiplier float64
	Jitter     bool
	attempt    int
}

func New(initial, max time.Duration) *Backoff {
	return &Backoff{Initial: initial, Max: max, Multiplier: 2.0, Jitter: true}
}

func (b *Backoff) Next() time.Duration {
	d := time.Duration(float64(b.Initial) * math.Pow(b.Multiplier, float64(b.attempt)))
	if d > b.Max {
		d = b.Max
	}
	b.attempt++
	if b.Jitter {
		jitter := time.Duration(rand.Float64() * float64(d) * 0.3)
		d += jitter
	}
	return d
}

func (b *Backoff) Reset()      { b.attempt = 0 }
func (b *Backoff) Attempt() int { return b.attempt }

func Retry(maxAttempts int, b *Backoff, fn func() error) error {
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if i < maxAttempts-1 {
			time.Sleep(b.Next())
		}
	}
	return lastErr
}
