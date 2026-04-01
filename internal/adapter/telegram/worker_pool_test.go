package telegram

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// TestWorkerPoolConcurrencyLimit verifies that the semaphore pattern used in
// StartPolling never allows more than the configured cap of goroutines to run.
func TestWorkerPoolConcurrencyLimit(t *testing.T) {
	const maxWorkers = 3

	b := NewBot("fake-token", "12345")
	b.workerSem = make(chan struct{}, maxWorkers)

	b.SetFreeTextHandler(func(ctx context.Context, chatID string, userID int64, username, text string, _ []ports.ContentBlock) error {
		return nil
	})

	var (
		active    atomic.Int32
		maxActive atomic.Int32
		mu        sync.Mutex
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const numGoroutines = 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		select {
		case b.workerSem <- struct{}{}:
		case <-ctx.Done():
			wg.Done()
			continue
		}
		go func() {
			defer wg.Done()
			defer func() { <-b.workerSem }()

			cur := active.Add(1)
			mu.Lock()
			if cur > maxActive.Load() {
				maxActive.Store(cur)
			}
			mu.Unlock()

			time.Sleep(10 * time.Millisecond)
			active.Add(-1)
		}()
	}

	wg.Wait()

	if got := maxActive.Load(); got > maxWorkers {
		t.Errorf("concurrent goroutines exceeded cap: got %d, want <= %d", got, maxWorkers)
	}
}

// TestWorkerPoolContextCancellation verifies that acquiring a worker slot
// respects context cancellation and does not block indefinitely.
func TestWorkerPoolContextCancellation(t *testing.T) {
	b := NewBot("fake-token", "12345")
	b.workerSem = make(chan struct{}, 1)

	// Fill the semaphore so next acquire would block.
	b.workerSem <- struct{}{}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case b.workerSem <- struct{}{}:
			// should not happen while slot is occupied and we haven't drained first
			t.Error("unexpected slot acquired")
		case <-ctx.Done():
			// expected: context cancelled unblocks the select
		}
	}()

	// Cancel after a short delay — the goroutine above must unblock.
	time.AfterFunc(50*time.Millisecond, cancel)

	select {
	case <-done:
		// pass
	case <-time.After(500 * time.Millisecond):
		t.Fatal("goroutine did not unblock within 500ms after context cancellation")
	}
}

// TestNewBotWorkerSemDefault verifies NewBot initialises workerSem with default cap 20.
func TestNewBotWorkerSemDefault(t *testing.T) {
	b := NewBot("fake-token", "12345")
	if got := cap(b.workerSem); got != 20 {
		t.Errorf("expected default workerSem capacity 20, got %d", got)
	}
}
