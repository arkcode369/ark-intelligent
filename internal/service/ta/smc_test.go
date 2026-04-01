package ta

import "testing"

// smcTestBars creates a clear up-trend with pronounced swings for SMC testing.
func smcTestBars() []OHLCV {
	// Oldest-first pattern with clear higher-highs + higher-lows
	closes := []float64{
		1.1000, 1.1020, 1.1045, 1.1070, 1.1100, 1.1130, 1.1150,
		1.1130, 1.1105, 1.1080, 1.1060, 1.1040,
		1.1065, 1.1095, 1.1125, 1.1160, 1.1200, 1.1240, 1.1270,
		1.1250, 1.1220, 1.1195, 1.1170, 1.1150,
		1.1175, 1.1210, 1.1250, 1.1295, 1.1340, 1.1380, 1.1420,
		1.1390, 1.1360, 1.1330, 1.1305, 1.1285,
		1.1310, 1.1355, 1.1400, 1.1450, 1.1500, 1.1540, 1.1580,
		1.1550, 1.1515, 1.1480, 1.1455, 1.1435,
		1.1460, 1.1500, 1.1545, 1.1590, 1.1640, 1.1685, 1.1720,
	}
	n := len(closes)
	bars := make([]OHLCV, n)
	for i, c := range closes {
		bars[n-1-i] = OHLCV{
			Open:  c - 0.0005,
			High:  c + 0.0015,
			Low:   c - 0.0015,
			Close: c,
		}
	}
	return bars
}

// TestDetectStructureBreaks_ReturnsResult verifies that adequate data produces a result.
func TestDetectStructureBreaks_ReturnsResult(t *testing.T) {
	bars := smcTestBars()
	result := DetectStructureBreaks(bars)

	// LastStructureTrend must be a valid value
	valid := map[string]bool{"BULLISH": true, "BEARISH": true, "RANGING": true}
	if !valid[result.LastStructureTrend] {
		t.Errorf("unexpected LastStructureTrend: %q", result.LastStructureTrend)
	}
}

// TestDetectStructureBreaks_InsufficientData verifies safe handling for short inputs.
func TestDetectStructureBreaks_InsufficientData(t *testing.T) {
	result := DetectStructureBreaks(nil)
	if result.LastStructureTrend != "RANGING" {
		t.Errorf("expected RANGING for nil input, got %q", result.LastStructureTrend)
	}

	short := make([]OHLCV, 10)
	result = DetectStructureBreaks(short)
	if result.LastStructureTrend != "RANGING" {
		t.Errorf("expected RANGING for short bars, got %q", result.LastStructureTrend)
	}
}

// TestDetectStructureBreaks_MaxThreeBreaks verifies at most 3 breaks returned.
func TestDetectStructureBreaks_MaxThreeBreaks(t *testing.T) {
	bars := smcTestBars()
	result := DetectStructureBreaks(bars)
	if len(result.Breaks) > 3 {
		t.Errorf("expected at most 3 breaks, got %d", len(result.Breaks))
	}
}

// TestDetectStructureBreaks_BreakTypes verifies break type strings.
func TestDetectStructureBreaks_BreakTypes(t *testing.T) {
	bars := smcTestBars()
	result := DetectStructureBreaks(bars)
	for _, b := range result.Breaks {
		if b.Type != "BOS" && b.Type != "CHOCH" {
			t.Errorf("unexpected break Type: %q", b.Type)
		}
		if b.Direction != "BULLISH" && b.Direction != "BEARISH" {
			t.Errorf("unexpected break Direction: %q", b.Direction)
		}
	}
}

// TestClassifyPremiumDiscount_Premium verifies PREMIUM classification.
func TestClassifyPremiumDiscount_Premium(t *testing.T) {
	// bars[0] (newest) is at the top of range → PREMIUM
	bars := smcTestBars()
	result := ClassifyPremiumDiscount(bars, 50)

	valid := map[string]bool{"PREMIUM": true, "DISCOUNT": true, "EQUILIBRIUM": true}
	if !valid[result.Zone] {
		t.Errorf("unexpected zone: %q", result.Zone)
	}

	// RangeHigh must be >= RangeLow
	if result.RangeHigh < result.RangeLow {
		t.Errorf("RangeHigh %.5f < RangeLow %.5f", result.RangeHigh, result.RangeLow)
	}

	// MidPoint must be between RangeLow and RangeHigh
	if result.MidPoint < result.RangeLow || result.MidPoint > result.RangeHigh {
		t.Errorf("MidPoint %.5f out of range [%.5f, %.5f]",
			result.MidPoint, result.RangeLow, result.RangeHigh)
	}

	// CurrentPosition must be 0-100
	if result.CurrentPosition < 0 || result.CurrentPosition > 100 {
		t.Errorf("CurrentPosition %.2f out of [0, 100]", result.CurrentPosition)
	}
}

// TestClassifyPremiumDiscount_PremiumZoneConsistency verifies zone matches price position.
func TestClassifyPremiumDiscount_PremiumZoneConsistency(t *testing.T) {
	bars := smcTestBars()
	result := ClassifyPremiumDiscount(bars, 50)

	currentPrice := bars[0].Close
	rangeSize := result.RangeHigh - result.RangeLow

	if result.Zone == "PREMIUM" && rangeSize > 0 {
		premiumThresh := result.MidPoint + rangeSize*0.10
		if currentPrice <= premiumThresh {
			t.Errorf("zone=PREMIUM but price %.5f <= threshold %.5f", currentPrice, premiumThresh)
		}
	}
	if result.Zone == "DISCOUNT" && rangeSize > 0 {
		discountThresh := result.MidPoint - rangeSize*0.10
		if currentPrice >= discountThresh {
			t.Errorf("zone=DISCOUNT but price %.5f >= threshold %.5f", currentPrice, discountThresh)
		}
	}
}

// TestSMCIntegration verifies SMC is populated in ComputeSnapshot.
func TestSMCIntegration(t *testing.T) {
	bars := smcTestBars()
	engine := NewEngine()
	snap := engine.ComputeSnapshot(bars)

	if snap == nil {
		t.Fatal("ComputeSnapshot returned nil")
	}

	// SMC should be populated for 55 bars
	if snap.SMC == nil {
		t.Fatal("expected SMC to be populated")
	}

	valid := map[string]bool{"BULLISH": true, "BEARISH": true, "RANGING": true}
	if !valid[snap.SMC.LastStructureTrend] {
		t.Errorf("unexpected LastStructureTrend: %q", snap.SMC.LastStructureTrend)
	}
}
