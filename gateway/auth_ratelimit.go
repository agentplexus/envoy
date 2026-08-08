package gateway

import (
	"context"
	"sync"
	"time"
)

const (
	// authFailureBaseDelay is the delay applied to the first failed
	// authentication attempt from a source.
	authFailureBaseDelay = 250 * time.Millisecond

	// authFailureMaxDelay caps the escalating delay.
	authFailureMaxDelay = 5 * time.Second
)

// authFailureLimiter applies a bounded, escalating delay to failed
// authentication attempts, keyed by source (client IP).
//
// Properties:
//   - The delay is applied only after the credential comparison, so correct
//     credentials are never delayed.
//   - The delay escalates per penalty window (base doubling to a cap), not
//     per attempt: concurrent failures for one key share a single deadline
//     and wait it out together, so parallel guessing cannot buy free
//     attempts and per-key state stays bounded.
//   - No source is ever locked out — loopback callers included — they only
//     pay the bounded delay.
//   - A successful authentication resets the penalty state for its key.
type authFailureLimiter struct {
	mu        sync.Mutex
	penalties map[string]*authPenalty
	baseDelay time.Duration
	maxDelay  time.Duration
	now       func() time.Time // injectable clock for tests
}

// authPenalty tracks the escalating delay state for one key.
type authPenalty struct {
	delay time.Duration // delay applied in the current window
	until time.Time     // shared deadline all concurrent failures wait for
}

// newAuthFailureLimiter creates a limiter with the default delays.
func newAuthFailureLimiter() *authFailureLimiter {
	return &authFailureLimiter{
		penalties: make(map[string]*authPenalty),
		baseDelay: authFailureBaseDelay,
		maxDelay:  authFailureMaxDelay,
		now:       time.Now,
	}
}

// recordFailureAndDelay registers a failed authentication attempt for key and
// blocks until the penalty deadline passes or ctx is done. It returns ctx.Err()
// when the wait was cut short, nil otherwise.
//
// Failures that arrive while a penalty window is active adopt the existing
// deadline instead of escalating, so one burst of concurrent guesses pays one
// (escalated) delay rather than compounding per attempt.
func (l *authFailureLimiter) recordFailureAndDelay(ctx context.Context, key string) error {
	l.mu.Lock()
	now := l.now()
	l.pruneLocked(now)

	p, ok := l.penalties[key]
	if !ok {
		p = &authPenalty{}
		l.penalties[key] = p
	}

	if !now.Before(p.until) {
		// Window expired (or first failure): escalate and open a new window.
		if p.delay == 0 {
			p.delay = l.baseDelay
		} else {
			p.delay *= 2
			if p.delay > l.maxDelay {
				p.delay = l.maxDelay
			}
		}
		p.until = now.Add(p.delay)
	}
	deadline := p.until
	l.mu.Unlock()

	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// reset clears the penalty state for key after a successful authentication.
func (l *authFailureLimiter) reset(key string) {
	l.mu.Lock()
	delete(l.penalties, key)
	l.mu.Unlock()
}

// pruneLocked drops penalty entries whose window has been idle for more
// than twice the maximum delay, keeping the map bounded. A source idle that
// long restarts from the base delay — an accepted trade-off: at one guess
// per prune-interval the delay is not the primary defense, the constant-time
// key comparison is. Caller must hold l.mu.
func (l *authFailureLimiter) pruneLocked(now time.Time) {
	for key, p := range l.penalties {
		if now.Sub(p.until) > l.maxDelay*2 {
			delete(l.penalties, key)
		}
	}
}
