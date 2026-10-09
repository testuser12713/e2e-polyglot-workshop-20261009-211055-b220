// Package ratelimit provides a small in-memory sliding-window limiter for the
// endpoints the security criteria bound (login attempts, public lookups).
package ratelimit

import (
	"sync"
	"time"
)

// Limiter allows at most limit requests per key within window. It is safe for
// concurrent use. A limit of zero or less disables limiting.
type Limiter struct {
	limit  int
	window time.Duration

	mu   sync.Mutex
	hits map[string][]time.Time
	now  func() time.Time
}

// New returns a limiter that permits limit events per window for each key.
func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		limit:  limit,
		window: window,
		hits:   make(map[string][]time.Time),
		now:    time.Now,
	}
}

// Allow records an event for key and reports whether it fits inside the
// current window. It returns false once the key has reached the limit.
func (l *Limiter) Allow(key string) bool {
	if l.limit <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)

	kept := l.hits[key][:0]
	for _, at := range l.hits[key] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) >= l.limit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}
