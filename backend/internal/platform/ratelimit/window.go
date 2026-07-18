package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Window là rate limiter fixed-window in-memory theo key (vd "login:<email>").
// Implements identity ports.LoginLimiter.
type Window struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
	now    func() time.Time
}

func NewWindow(limit int, window time.Duration) *Window {
	return &Window{
		hits:   make(map[string][]time.Time),
		limit:  limit,
		window: window,
		now:    time.Now,
	}
}

func (w *Window) Allow(_ context.Context, key string) (bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.now()
	cutoff := now.Add(-w.window)
	kept := w.hits[key][:0]
	for _, t := range w.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= w.limit {
		w.hits[key] = kept
		return false, nil
	}
	w.hits[key] = append(kept, now)
	return true, nil
}

func (w *Window) Reset(_ context.Context, key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.hits, key)
}
