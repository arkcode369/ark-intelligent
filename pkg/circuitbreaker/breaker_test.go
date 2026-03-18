package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

func TestBreakerClosedState(t *testing.T) {
	cb := New("test", 3, 100*time.Millisecond)

	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if cb.State() != Closed {
		t.Fatalf("expected Closed, got %v", cb.State())
	}
}

func TestBreakerOpensAfterMaxFailures(t *testing.T) {
	cb := New("test", 3, 100*time.Millisecond)
	testErr := errors.New("fail")

	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error { return testErr })
	}

	if cb.State() != Open {
		t.Fatalf("expected Open after 3 failures, got %v", cb.State())
	}

	err := cb.Execute(func() error { return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestBreakerHalfOpenAfterTimeout(t *testing.T) {
	cb := New("test", 2, 50*time.Millisecond)
	testErr := errors.New("fail")

	_ = cb.Execute(func() error { return testErr })
	_ = cb.Execute(func() error { return testErr })

	if cb.State() != Open {
		t.Fatalf("expected Open, got %v", cb.State())
	}

	time.Sleep(60 * time.Millisecond)

	if cb.State() != HalfOpen {
		t.Fatalf("expected HalfOpen after timeout, got %v", cb.State())
	}
}

func TestBreakerHalfOpenSuccessCloses(t *testing.T) {
	cb := New("test", 2, 50*time.Millisecond)
	testErr := errors.New("fail")

	_ = cb.Execute(func() error { return testErr })
	_ = cb.Execute(func() error { return testErr })

	time.Sleep(60 * time.Millisecond)

	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("expected success in HalfOpen probe, got %v", err)
	}
	if cb.State() != Closed {
		t.Fatalf("expected Closed after successful probe, got %v", cb.State())
	}
	if cb.Failures() != 0 {
		t.Fatalf("expected 0 failures after reset, got %d", cb.Failures())
	}
}

func TestBreakerHalfOpenFailureReopens(t *testing.T) {
	cb := New("test", 2, 50*time.Millisecond)
	testErr := errors.New("fail")

	_ = cb.Execute(func() error { return testErr })
	_ = cb.Execute(func() error { return testErr })

	time.Sleep(60 * time.Millisecond)

	_ = cb.Execute(func() error { return testErr })
	if cb.State() != Open {
		t.Fatalf("expected Open after failed probe, got %v", cb.State())
	}
}

func TestBreakerReset(t *testing.T) {
	cb := New("test", 2, time.Hour)
	testErr := errors.New("fail")

	_ = cb.Execute(func() error { return testErr })
	_ = cb.Execute(func() error { return testErr })

	if cb.State() != Open {
		t.Fatalf("expected Open, got %v", cb.State())
	}

	cb.Reset()
	if cb.State() != Closed {
		t.Fatalf("expected Closed after Reset, got %v", cb.State())
	}
}

func TestBreakerSuccessResetsFailureCount(t *testing.T) {
	cb := New("test", 3, time.Hour)
	testErr := errors.New("fail")

	_ = cb.Execute(func() error { return testErr })
	_ = cb.Execute(func() error { return testErr })
	// 2 failures, one more would open

	_ = cb.Execute(func() error { return nil }) // success resets
	if cb.Failures() != 0 {
		t.Fatalf("expected 0 failures after success, got %d", cb.Failures())
	}

	_ = cb.Execute(func() error { return testErr })
	_ = cb.Execute(func() error { return testErr })
	// 2 failures again, still closed
	if cb.State() != Closed {
		t.Fatalf("expected Closed (2 < 3), got %v", cb.State())
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{Closed, "CLOSED"},
		{Open, "OPEN"},
		{HalfOpen, "HALF_OPEN"},
		{State(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
