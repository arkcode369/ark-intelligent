package telegram

import (
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Per-user rate limiter (sliding window)
// ---------------------------------------------------------------------------

const (
	rateLimitWindow  = 60 * time.Second // sliding window duration
	rateLimitMax     = 10               // max commands per window
	staleEntryTTL    = 5 * time.Minute  // cleanup threshold for idle users
	cleanupInterval  = 2 * time.Minute  // how often the cleanup goroutine runs
)

// userWindow tracks command timestamps for a single user.
type userWindow struct {
	timestamps []time.Time
}

// userRateLimiter enforces per-user command rate limits using a sliding window.
type userRateLimiter struct {
	mu    sync.Mutex
	users map[int64]*userWindow
	stop  chan struct{}
}

// newUserRateLimiter creates a rate limiter and starts the background cleanup.
func newUserRateLimiter() *userRateLimiter {
	rl := &userRateLimiter{
		users: make(map[int64]*userWindow),
		stop:  make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Allow reports whether the user is allowed to execute a command right now.
// If allowed, it records the current timestamp; otherwise it returns false.
func (rl *userRateLimiter) Allow(userID int64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	w, ok := rl.users[userID]
	if !ok {
		w = &userWindow{}
		rl.users[userID] = w
	}

	// Slide the window: drop timestamps older than the window.
	cutoff := now.Add(-rateLimitWindow)
	start := 0
	for start < len(w.timestamps) && w.timestamps[start].Before(cutoff) {
		start++
	}
	w.timestamps = w.timestamps[start:]

	if len(w.timestamps) >= rateLimitMax {
		return false
	}

	w.timestamps = append(w.timestamps, now)
	return true
}

// cleanupLoop periodically removes entries for users who have been idle
// longer than staleEntryTTL to prevent unbounded memory growth.
func (rl *userRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stop:
			return
		case <-ticker.C:
			rl.cleanup()
		}
	}
}

// cleanup removes user entries whose newest timestamp is older than staleEntryTTL.
func (rl *userRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-staleEntryTTL)
	for uid, w := range rl.users {
		if len(w.timestamps) == 0 || w.timestamps[len(w.timestamps)-1].Before(cutoff) {
			delete(rl.users, uid)
		}
	}
}

// Stop terminates the background cleanup goroutine.
func (rl *userRateLimiter) Stop() {
	close(rl.stop)
}
