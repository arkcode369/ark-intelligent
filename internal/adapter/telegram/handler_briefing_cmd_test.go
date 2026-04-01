package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
)

// ---------------------------------------------------------------------------
// Unit tests for /briefing handler helpers and formatter
// ---------------------------------------------------------------------------

// TestBriefingFilterEvents verifies that only High/Medium impact events are kept
// and that they are sorted ascending by TimeWIB.
func TestBriefingFilterEvents_FiltersAndSorts(t *testing.T) {
	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	events := []domain.NewsEvent{
		{Event: "GDP", Impact: "low", TimeWIB: base.Add(1 * time.Hour)},
		{Event: "NFP", Impact: "high", TimeWIB: base.Add(3 * time.Hour)},
		{Event: "CPI", Impact: "medium", TimeWIB: base.Add(2 * time.Hour)},
		{Event: "Non-event", Impact: "non", TimeWIB: base.Add(30 * time.Minute)},
		{Event: "Retail Sales", Impact: "High", TimeWIB: base.Add(4 * time.Hour)}, // capital
	}

	filtered := briefingFilterEvents(events)

	if len(filtered) != 3 {
		t.Fatalf("expected 3 events (high+medium), got %d", len(filtered))
	}

	// Should be sorted: CPI(+2h), NFP(+3h), Retail Sales(+4h)
	if filtered[0].Event != "CPI" {
		t.Errorf("expected first event to be CPI, got %q", filtered[0].Event)
	}
	if filtered[1].Event != "NFP" {
		t.Errorf("expected second event to be NFP, got %q", filtered[1].Event)
	}
	if filtered[2].Event != "Retail Sales" {
		t.Errorf("expected third event to be Retail Sales, got %q", filtered[2].Event)
	}
}

// TestBriefingFilterEvents_Empty verifies no panic on empty input.
func TestBriefingFilterEvents_Empty(t *testing.T) {
	filtered := briefingFilterEvents(nil)
	if filtered != nil && len(filtered) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(filtered))
	}

	filtered = briefingFilterEvents([]domain.NewsEvent{})
	if len(filtered) != 0 {
		t.Errorf("expected empty result for empty input, got %d", len(filtered))
	}
}

// TestBriefingFilterEvents_ZeroTimeWIB verifies that events with zero TimeWIB
// are sorted to the end (after events with valid times).
func TestBriefingFilterEvents_ZeroTimeWIBLast(t *testing.T) {
	base := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)

	events := []domain.NewsEvent{
		{Event: "Tentative", Impact: "high"}, // zero TimeWIB
		{Event: "NFP", Impact: "high", TimeWIB: base},
	}

	filtered := briefingFilterEvents(events)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 events, got %d", len(filtered))
	}
	if filtered[0].Event != "NFP" {
		t.Errorf("expected NFP first (has TimeWIB), got %q", filtered[0].Event)
	}
	if filtered[1].Event != "Tentative" {
		t.Errorf("expected Tentative last (zero TimeWIB), got %q", filtered[1].Event)
	}
}

// TestFormatBriefing_NoData verifies the formatter handles all-nil / empty data gracefully.
func TestFormatBriefing_NoData(t *testing.T) {
	f := NewFormatter()
	now := time.Date(2026, 4, 1, 6, 0, 0, 0, time.UTC)

	html := f.FormatBriefing(now, nil, nil)

	if !strings.Contains(html, "ARK Daily Briefing") {
		t.Error("missing briefing header")
	}
	if !strings.Contains(html, "Events Hari Ini") {
		t.Error("missing events section")
	}
	if !strings.Contains(html, "Top COT Signals") {
		t.Error("missing COT signals section")
	}
	if !strings.Contains(html, "Updated:") {
		t.Error("missing updated timestamp")
	}
}

// TestFormatBriefing_WithData verifies that conviction scores and events appear in output.
func TestFormatBriefing_WithData(t *testing.T) {
	f := NewFormatter()
	now := time.Date(2026, 4, 1, 6, 30, 0, 0, time.UTC)

	events := []domain.NewsEvent{
		{Event: "NFP", Impact: "high", Currency: "USD", Time: "08:30", TimeWIB: now.Add(2 * time.Hour)},
		{Event: "CPI", Impact: "medium", Currency: "EUR", Time: "10:00", TimeWIB: now.Add(4 * time.Hour)},
	}

	convictions := []cot.ConvictionScore{
		{Currency: "EUR", Score: 72.5, COTBias: "BULLISH", Direction: "LONG"},
		{Currency: "JPY", Score: -65.0, COTBias: "BEARISH", Direction: "SHORT"},
	}

	html := f.FormatBriefing(now, events, convictions)

	if !strings.Contains(html, "NFP") {
		t.Error("expected NFP event in output")
	}
	if !strings.Contains(html, "EUR") {
		t.Error("expected EUR conviction in output")
	}
	if !strings.Contains(html, "LONG") {
		t.Error("expected LONG direction in output")
	}
	if !strings.Contains(html, "JPY") {
		t.Error("expected JPY conviction in output")
	}
	// Output should not exceed 3000 chars
	if len(html) > 3000 {
		t.Errorf("briefing output too long: %d chars (max 3000)", len(html))
	}
}

// TestFormatBriefing_BiasGroups verifies that BULLISH/BEARISH/NEUTRAL currencies
// are correctly grouped in the bias summary.
func TestFormatBriefing_BiasGroups(t *testing.T) {
	f := NewFormatter()
	now := time.Now()

	convictions := []cot.ConvictionScore{
		{Currency: "EUR", COTBias: "BULLISH", Score: 80},
		{Currency: "GBP", COTBias: "BULLISH", Score: 60},
		{Currency: "JPY", COTBias: "BEARISH", Score: -70},
		{Currency: "CHF", COTBias: "NEUTRAL", Score: 5},
	}

	html := f.FormatBriefing(now, nil, convictions)

	if !strings.Contains(html, "EUR, GBP") {
		t.Errorf("expected EUR, GBP in bullish group; got:\n%s", html)
	}
	if !strings.Contains(html, "JPY") {
		t.Errorf("expected JPY in bearish group; got:\n%s", html)
	}
}

// TestBriefingMenu verifies the keyboard has the expected buttons.
func TestBriefingMenu_Structure(t *testing.T) {
	kb := NewKeyboardBuilder()
	menu := kb.BriefingMenu()

	if len(menu.Rows) == 0 {
		t.Fatal("BriefingMenu should have at least one row")
	}

	// Check for Refresh button
	found := false
	for _, row := range menu.Rows {
		for _, btn := range row {
			if btn.CallbackData == "briefing:refresh" {
				found = true
			}
		}
	}
	if !found {
		t.Error("BriefingMenu missing 'briefing:refresh' button")
	}
}
