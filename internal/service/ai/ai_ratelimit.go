package ai

import (
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
)

// aiRateLimiter enforces RPM (sliding window) and daily cap limits
// for AI API calls. Thread-safe via sync.Mutex.
type aiRateLimiter struct {
	mu         sync.Mutex
	maxRPM     int
	maxDaily   int
	timestamps []time.Time // sliding window for RPM
	dailyCount int
	dailyReset time.Time // next midnight WIB
}

// newAIRateLimiter creates a rate limiter with the given RPM and daily caps.
// Pass 0 for either limit to disable that check.
func newAIRateLimiter(maxRPM, maxDaily int) *aiRateLimiter {
	rl := &aiRateLimiter{
		maxRPM:   maxRPM,
		maxDaily: maxDaily,
	}
	rl.dailyReset = nextMidnightWIB()
	return rl
}

// Allow checks whether a new request is permitted under both RPM and daily
// limits. Returns false if the request should be rejected.
func (rl *aiRateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := timeutil.NowWIB()

	// Reset daily counter at midnight WIB
	if now.After(rl.dailyReset) {
		rl.dailyCount = 0
		rl.dailyReset = nextMidnightWIB()
	}

	// Check daily cap
	if rl.maxDaily > 0 && rl.dailyCount >= rl.maxDaily {
		return false
	}

	// Sliding window: remove timestamps older than 1 minute
	cutoff := now.Add(-time.Minute)
	fresh := rl.timestamps[:0]
	for _, ts := range rl.timestamps {
		if ts.After(cutoff) {
			fresh = append(fresh, ts)
		}
	}
	rl.timestamps = fresh

	// Check RPM
	if rl.maxRPM > 0 && len(rl.timestamps) >= rl.maxRPM {
		return false
	}

	// Admit the request
	rl.timestamps = append(rl.timestamps, now)
	rl.dailyCount++
	return true
}

// Stats returns current usage metrics for logging/status.
func (rl *aiRateLimiter) Stats() (rpm int, dailyUsed int, dailyMax int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := timeutil.NowWIB()

	// Reset daily counter if past midnight
	if now.After(rl.dailyReset) {
		rl.dailyCount = 0
		rl.dailyReset = nextMidnightWIB()
	}

	// Count requests in the last minute
	cutoff := now.Add(-time.Minute)
	count := 0
	for _, ts := range rl.timestamps {
		if ts.After(cutoff) {
			count++
		}
	}

	return count, rl.dailyCount, rl.maxDaily
}

// nextMidnightWIB returns the next midnight in WIB (UTC+7).
func nextMidnightWIB() time.Time {
	now := timeutil.NowWIB()
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return next
}
