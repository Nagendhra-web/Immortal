package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

type State string

const (
	StateClosed   State = "closed"
	StateOpen     State = "open"
	StateHalfOpen State = "half_open"
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type Breaker struct {
	mu            sync.RWMutex
	state         State
	failures      int
	successes     int
	maxFailures   int
	resetTimeout  time.Duration
	halfOpenMax   int
	lastFailure   time.Time
	onStateChange func(from, to State)
}

func New(maxFailures int, resetTimeout time.Duration) *Breaker {
	return &Breaker{
		state:        StateClosed,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		halfOpenMax:  1,
	}
}

func (b *Breaker) Execute(fn func() error) error {
	if !b.canExecute() {
		return ErrCircuitOpen
	}
	err := fn()
	b.record(err)
	return err
}

func (b *Breaker) canExecute() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(b.lastFailure) > b.resetTimeout {
			b.transition(StateHalfOpen)
			b.successes = 0
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

func (b *Breaker) record(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err != nil {
		b.failures++
		b.lastFailure = time.Now()
		if b.state == StateHalfOpen || b.failures >= b.maxFailures {
			b.transition(StateOpen)
		}
	} else {
		if b.state == StateHalfOpen {
			b.successes++
			if b.successes >= b.halfOpenMax {
				b.failures = 0
				b.transition(StateClosed)
			}
		} else {
			b.failures = 0
		}
	}
}

func (b *Breaker) transition(to State) {
	from := b.state
	b.state = to
	if b.onStateChange != nil {
		go b.onStateChange(from, to)
	}
}

func (b *Breaker) State() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

func (b *Breaker) OnStateChange(fn func(from, to State)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onStateChange = fn
}

func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = StateClosed
	b.failures = 0
	b.successes = 0
}
