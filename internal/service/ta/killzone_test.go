package ta

import (
	"testing"
	"time"
)

func TestClassifyKillzone_LondonOpen(t *testing.T) {
	// 08:30 UTC = London Open Killzone
	ts := time.Date(2026, 4, 1, 8, 30, 0, 0, time.UTC)
	result := ClassifyKillzone(ts)

	if result.ActiveKillzone != "LONDON_OPEN" {
		t.Errorf("got %q, want LONDON_OPEN", result.ActiveKillzone)
	}
	if !result.IsActive {
		t.Error("expected IsActive=true")
	}
}

func TestClassifyKillzone_NYOpen(t *testing.T) {
	ts := time.Date(2026, 4, 1, 13, 0, 0, 0, time.UTC)
	result := ClassifyKillzone(ts)

	if result.ActiveKillzone != "NY_OPEN" {
		t.Errorf("got %q, want NY_OPEN", result.ActiveKillzone)
	}
	if !result.IntersessionOverlap {
		t.Error("expected IntersessionOverlap=true at 13:00 UTC")
	}
}

func TestClassifyKillzone_OffHours(t *testing.T) {
	// 05:00 UTC = between Asia and London
	ts := time.Date(2026, 4, 1, 5, 0, 0, 0, time.UTC)
	result := ClassifyKillzone(ts)

	if result.ActiveKillzone != "OFF_HOURS" {
		t.Errorf("got %q, want OFF_HOURS", result.ActiveKillzone)
	}
	if result.IsActive {
		t.Error("expected IsActive=false")
	}
}

func TestNextKillzoneInfo(t *testing.T) {
	// At 05:00 UTC, next should be LONDON_OPEN at 07:00 = 120 min
	ts := time.Date(2026, 4, 1, 5, 0, 0, 0, time.UTC)
	result := ClassifyKillzone(ts)

	if result.NextKillzone != "LONDON_OPEN" {
		t.Errorf("next: got %q, want LONDON_OPEN", result.NextKillzone)
	}
	if result.MinutesUntilNext != 120 {
		t.Errorf("minutes: got %d, want 120", result.MinutesUntilNext)
	}
}
