package backtest

import (
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

func makeSignal(direction string, strength int, confidence float64, ret1w, ret2w, ret4w float64) domain.PersistedSignal {
	s := domain.PersistedSignal{
		ContractCode: "099741",
		Currency:     "EUR",
		SignalType:   "SMART_MONEY",
		Direction:    direction,
		Strength:     strength,
		Confidence:   confidence,
		EntryPrice:   1.08,
		ReportDate:   time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC),
		Return1W:     ret1w,
		Return2W:     ret2w,
		Return4W:     ret4w,
	}

	// Classify outcomes
	if ret1w != 0 {
		s.Outcome1W = classifyOutcome(direction, ret1w)
	}
	if ret2w != 0 {
		s.Outcome2W = classifyOutcome(direction, ret2w)
	}
	if ret4w != 0 {
		s.Outcome4W = classifyOutcome(direction, ret4w)
	}
	return s
}

func TestComputeStatsEmpty(t *testing.T) {
	stats := computeStats(nil, "EMPTY")
	if stats.TotalSignals != 0 {
		t.Errorf("TotalSignals = %d, want 0", stats.TotalSignals)
	}
	if stats.GroupLabel != "EMPTY" {
		t.Errorf("GroupLabel = %q, want EMPTY", stats.GroupLabel)
	}
}

func TestComputeStatsWinRates(t *testing.T) {
	signals := []domain.PersistedSignal{
		makeSignal("BULLISH", 4, 70, 0.5, 1.2, 2.1),    // 1W win, 2W win, 4W win
		makeSignal("BULLISH", 3, 60, -0.3, 0.8, 1.5),    // 1W loss, 2W win, 4W win
		makeSignal("BEARISH", 5, 80, -0.7, -1.1, -2.3),  // 1W win (bearish+neg), 2W win, 4W win
		makeSignal("BEARISH", 2, 50, 0.2, 0.5, -0.8),    // 1W loss, 2W loss (bearish+pos=loss), 4W win
	}

	stats := computeStats(signals, "TEST")

	if stats.TotalSignals != 4 {
		t.Errorf("TotalSignals = %d, want 4", stats.TotalSignals)
	}

	// 1W: 2 wins out of 4 = 50%
	if stats.WinRate1W != 50 {
		t.Errorf("WinRate1W = %.2f, want 50", stats.WinRate1W)
	}

	// 2W: 3 wins out of 4 = 75%
	if stats.WinRate2W != 75 {
		t.Errorf("WinRate2W = %.2f, want 75", stats.WinRate2W)
	}

	// 4W: 4 wins out of 4 = 100%
	if stats.WinRate4W != 100 {
		t.Errorf("WinRate4W = %.2f, want 100", stats.WinRate4W)
	}

	// Best period should be 4W with 100%
	if stats.BestPeriod != "4W" {
		t.Errorf("BestPeriod = %q, want 4W", stats.BestPeriod)
	}
	if stats.BestWinRate != 100 {
		t.Errorf("BestWinRate = %.2f, want 100", stats.BestWinRate)
	}
}

func TestComputeStatsAvgReturns(t *testing.T) {
	signals := []domain.PersistedSignal{
		makeSignal("BULLISH", 4, 70, 1.0, 2.0, 3.0),
		makeSignal("BULLISH", 3, 60, -0.5, 1.0, 2.0),
	}

	stats := computeStats(signals, "TEST")

	// Avg 1W return: (1.0 + -0.5) / 2 = 0.25
	if stats.AvgReturn1W != 0.25 {
		t.Errorf("AvgReturn1W = %.4f, want 0.25", stats.AvgReturn1W)
	}

	// Avg 2W return: (2.0 + 1.0) / 2 = 1.5
	if stats.AvgReturn2W != 1.5 {
		t.Errorf("AvgReturn2W = %.4f, want 1.5", stats.AvgReturn2W)
	}
}

func TestComputeStatsStrengthBreakdown(t *testing.T) {
	signals := []domain.PersistedSignal{
		makeSignal("BULLISH", 5, 80, 1.0, 0, 0),  // high strength, win
		makeSignal("BULLISH", 4, 75, -0.5, 0, 0),  // high strength, loss
		makeSignal("BULLISH", 3, 60, 0.5, 0, 0),   // low strength, win
		makeSignal("BULLISH", 2, 50, -0.3, 0, 0),  // low strength, loss
		makeSignal("BULLISH", 1, 40, -0.2, 0, 0),  // low strength, loss
	}

	stats := computeStats(signals, "TEST")

	if stats.HighStrengthCount != 2 {
		t.Errorf("HighStrengthCount = %d, want 2", stats.HighStrengthCount)
	}
	if stats.LowStrengthCount != 3 {
		t.Errorf("LowStrengthCount = %d, want 3", stats.LowStrengthCount)
	}

	// High strength: 1 win / 2 total = 50%
	if stats.HighStrengthWinRate != 50 {
		t.Errorf("HighStrengthWinRate = %.2f, want 50", stats.HighStrengthWinRate)
	}

	// Low strength: 1 win / 3 total = 33.33%
	expected := 33.33
	if stats.LowStrengthWinRate != expected {
		t.Errorf("LowStrengthWinRate = %.2f, want %.2f", stats.LowStrengthWinRate, expected)
	}
}

func TestComputeStatsCalibration(t *testing.T) {
	// All signals with 80% confidence, but 50% actual win rate at 1W
	signals := []domain.PersistedSignal{
		makeSignal("BULLISH", 4, 80, 1.0, 1.5, 2.0),
		makeSignal("BULLISH", 4, 80, -0.5, -0.3, 0.5),
	}

	stats := computeStats(signals, "TEST")

	if stats.AvgConfidence != 80 {
		t.Errorf("AvgConfidence = %.2f, want 80", stats.AvgConfidence)
	}

	// Best win rate is 4W at 100% (both signals have positive 4W returns, both bullish)
	// Signal 1: ret4w=2.0 → WIN (bullish), Signal 2: ret4w=0.5 → WIN (bullish)
	if stats.ActualAccuracy != 100 {
		t.Errorf("ActualAccuracy = %.2f, want 100", stats.ActualAccuracy)
	}

	// Calibration error = |80 - 100| = 20
	if stats.CalibrationError != 20 {
		t.Errorf("CalibrationError = %.2f, want 20", stats.CalibrationError)
	}
}

func TestComputeStatsWinLossReturns(t *testing.T) {
	signals := []domain.PersistedSignal{
		makeSignal("BULLISH", 4, 70, 2.0, 0, 0),  // win
		makeSignal("BULLISH", 3, 60, 1.0, 0, 0),   // win
		makeSignal("BULLISH", 3, 60, -0.5, 0, 0),  // loss
	}

	stats := computeStats(signals, "TEST")

	// Avg win return at 1W: (2.0 + 1.0) / 2 = 1.5
	if stats.AvgWinReturn1W != 1.5 {
		t.Errorf("AvgWinReturn1W = %.4f, want 1.5", stats.AvgWinReturn1W)
	}

	// Avg loss return at 1W: -0.5 / 1 = -0.5
	if stats.AvgLossReturn1W != -0.5 {
		t.Errorf("AvgLossReturn1W = %.4f, want -0.5", stats.AvgLossReturn1W)
	}
}

func TestClassifyOutcome(t *testing.T) {
	tests := []struct {
		direction string
		ret       float64
		want      string
	}{
		{"BULLISH", 1.5, domain.OutcomeWin},
		{"BULLISH", -0.5, domain.OutcomeLoss},
		{"BULLISH", 0.0, domain.OutcomeLoss}, // zero = not a win
		{"BEARISH", -1.5, domain.OutcomeWin},
		{"BEARISH", 0.5, domain.OutcomeLoss},
		{"NEUTRAL", 1.0, domain.OutcomePending},
	}

	for _, tt := range tests {
		got := classifyOutcome(tt.direction, tt.ret)
		if got != tt.want {
			t.Errorf("classifyOutcome(%q, %.1f) = %q, want %q", tt.direction, tt.ret, got, tt.want)
		}
	}
}

func TestComputeReturn(t *testing.T) {
	tests := []struct {
		entry, exit float64
		inverse     bool
		want        float64
	}{
		{1.08, 1.10, false, 1.8519},   // (1.10-1.08)/1.08 * 100 ≈ 1.8519
		{1.08, 1.06, false, -1.8519},  // negative return
		{150.0, 152.0, true, -1.3333}, // inverse: price up = bearish for foreign ccy
		{150.0, 148.0, true, 1.3333},  // inverse: price down = bullish for foreign ccy
		{0, 1.0, false, 0},            // zero entry
	}

	for _, tt := range tests {
		got := computeReturn(tt.entry, tt.exit, tt.inverse)
		diff := got - tt.want
		if diff < -0.001 || diff > 0.001 {
			t.Errorf("computeReturn(%.2f, %.2f, %v) = %.4f, want ≈%.4f",
				tt.entry, tt.exit, tt.inverse, got, tt.want)
		}
	}
}
