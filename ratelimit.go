package quote0

import (
	"context"
	"sync"
	"time"
)

// RateLimiter gates outgoing API calls so we stay under the documented 1 QPS.
// Implementations must honor context cancellation so callers can abort pending calls cleanly.
type RateLimiter interface {
	Wait(ctx context.Context) error
}

// RateLimiterFunc adapts a function into a RateLimiter.
type RateLimiterFunc func(ctx context.Context) error

// Wait implements the RateLimiter interface by invoking the underlying function.
func (f RateLimiterFunc) Wait(ctx context.Context) error {
	if f == nil {
		return nil
	}
	return f(ctx)
}

// NewFixedIntervalLimiter creates a simple, concurrency-safe limiter that enforces
// a minimum interval between requests. It is intentionally lightweight (mutex+timer)
// so it can run inside tiny IoT gateways or embedded controllers without extra deps.
// Use this for the official 1 QPS policy by passing time.Second, or customize as needed.
func NewFixedIntervalLimiter(interval time.Duration) RateLimiter {
	if interval <= 0 {
		interval = time.Second
	}
	return &fixedIntervalLimiter{minInterval: interval}
}

// fixedIntervalLimiter enforces a fixed minimum time interval between consecutive API calls.
// It tracks the next allowed request time and blocks callers until that time arrives.
type fixedIntervalLimiter struct {
	mu          sync.Mutex
	next        time.Time
	minInterval time.Duration
}

// Wait blocks the caller until the rate limit allows the next request.
// It respects context cancellation and returns ctx.Err() if the context is canceled before the wait completes.
func (l *fixedIntervalLimiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	wait := time.Duration(0)
	start := now
	if !l.next.IsZero() && now.Before(l.next) {
		wait = l.next.Sub(now)
		start = l.next
	}
	l.next = start.Add(l.minInterval)
	l.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	// Use a one-shot timer so concurrent callers queue without busy-waiting.
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		// If the caller's context is canceled, propagate that error so API calls stop immediately.
		return ctx.Err()
	case <-timer.C:
		// Timer fired, caller may proceed with the actual HTTP request.
		return nil
	}
}
