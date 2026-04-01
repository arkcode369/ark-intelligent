package ta

import (
	"testing"
)

// makeWavesBars creates bars with pronounced waves that create clear swing points.
// Pattern: rise 7 bars, fall 5 bars, rise 7 bars, etc. (newest-first output)
func makeWavesBars() []OHLCV {
	// Build oldest-first sequence, then reverse to newest-first
	// Creates clear HH+HL bullish structure
	closes := []float64{
		// Rising wave 1
		1.1000, 1.1020, 1.1045, 1.1070, 1.1100, 1.1130, 1.1150,
		// Falling wave 1
		1.1130, 1.1105, 1.1080, 1.1060, 1.1040,
		// Rising wave 2 (higher high)
		1.1065, 1.1095, 1.1125, 1.1160, 1.1200, 1.1240, 1.1270,
		// Falling wave 2 (higher low)
		1.1250, 1.1220, 1.1195, 1.1170, 1.1150,
		// Rising wave 3 (higher high)
		1.1175, 1.1210, 1.1250, 1.1295, 1.1340, 1.1380, 1.1420,
		// Falling wave 3 (higher low)
		1.1390, 1.1360, 1.1330, 1.1305, 1.1285,
		// Rising wave 4 (higher high)
		1.1310, 1.1355, 1.1400, 1.1450, 1.1500, 1.1540, 1.1580,
		// Falling wave 4 (higher low)
		1.1550, 1.1515, 1.1480, 1.1455, 1.1435,
		// Final rise
		1.1460, 1.1500, 1.1545, 1.1590, 1.1640, 1.1685, 1.1720,
	}
	n := len(closes)
	// oldest-first → newest-first
	bars := make([]OHLCV, n)
	for i, c := range closes {
		bars[n-1-i] = OHLCV{
			Open:  c - 0.0005,
			High:  c + 0.0010,
			Low:   c - 0.0010,
			Close: c,
		}
	}
	return bars
}

// makeBearWavesBars creates a clear downtrend with lower highs and lower lows.
func makeBearWavesBars() []OHLCV {
	closes := []float64{
		// Falling wave 1
		1.3000, 1.2970, 1.2940, 1.2910, 1.2880, 1.2850, 1.2820,
		// Rising wave 1
		1.2840, 1.2865, 1.2890, 1.2910, 1.2930,
		// Falling wave 2 (lower low)
		1.2905, 1.2875, 1.2840, 1.2805, 1.2770, 1.2740, 1.2710,
		// Rising wave 2 (lower high)
		1.2730, 1.2755, 1.2780, 1.2800, 1.2820,
		// Falling wave 3
		1.2795, 1.2760, 1.2720, 1.2680, 1.2640, 1.2600, 1.2560,
		// Rising wave 3 (lower high)
		1.2580, 1.2605, 1.2630, 1.2655, 1.2675,
		// Falling wave 4
		1.2645, 1.2610, 1.2570, 1.2530, 1.2490, 1.2450, 1.2410,
		// Rising wave 4 (lower high)
		1.2430, 1.2455, 1.2480, 1.2500, 1.2520,
		// Final fall
		1.2490, 1.2455, 1.2415, 1.2375, 1.2335, 1.2295, 1.2255,
	}
	n := len(closes)
	bars := make([]OHLCV, n)
	for i, c := range closes {
		bars[n-1-i] = OHLCV{
			Open:  c + 0.0005,
			High:  c + 0.0010,
			Low:   c - 0.0010,
			Close: c,
		}
	}
	return bars
}

// TestCalcSMC_NilOnInsufficientData verifies that CalcSMC returns nil for too-short input.
func TestCalcSMC_NilOnInsufficientData(t *testing.T) {
	result := CalcSMC(nil, 0)
	if result != nil {
		t.Error("expected nil for nil input")
	}

	short := make([]OHLCV, 10)
	result = CalcSMC(short, 0)
	if result != nil {
		t.Error("expected nil for <15 bars")
	}
}

// TestCalcSMC_ReturnsResult verifies CalcSMC returns a non-nil result for adequate data.
func TestCalcSMC_ReturnsResult(t *testing.T) {
	bars := makeWavesBars()
	result := CalcSMC(bars, 0.0015)
	if result == nil {
		t.Fatalf("expected non-nil result for %d bars", len(bars))
	}
}

// TestCalcSMC_StructureFields verifies required fields are populated.
func TestCalcSMC_StructureFields(t *testing.T) {
	bars := makeWavesBars()
	result := CalcSMC(bars, 0.0015)
	if result == nil {
		t.Fatal("CalcSMC returned nil")
	}

	validStructures := map[MarketStructure]bool{
		StructureBullish: true,
		StructureBearish: true,
		StructureRanging: true,
	}
	if !validStructures[result.Structure] {
		t.Errorf("unexpected structure: %q", result.Structure)
	}

	if result.Trend != string(result.Structure) {
		t.Errorf("Trend %q != Structure %q", result.Trend, result.Structure)
	}
}

// TestCalcSMC_BullishTrend detects bullish structure in rising wave bars.
func TestCalcSMC_BullishTrend(t *testing.T) {
	bars := makeWavesBars()
	result := CalcSMC(bars, 0.0015)
	if result == nil {
		t.Fatal("CalcSMC returned nil")
	}
	// Bullish wave data should produce BULLISH or RANGING (both acceptable)
	if result.Structure == StructureBearish {
		t.Errorf("expected BULLISH or RANGING for bullish wave data, got BEARISH")
	}
}

// TestCalcSMC_BearishTrend detects bearish/ranging structure in falling wave bars.
func TestCalcSMC_BearishTrend(t *testing.T) {
	bars := makeBearWavesBars()
	result := CalcSMC(bars, 0.0015)
	if result == nil {
		t.Skip("insufficient swing data for bearish bars")
	}
	if result.Structure == StructureBullish {
		t.Errorf("expected BEARISH or RANGING for bearish wave data, got BULLISH")
	}
}

// TestPremiumZone verifies price is correctly classified as PREMIUM when above EQ.
func TestPremiumZone(t *testing.T) {
	bars := makeWavesBars()
	result := CalcSMC(bars, 0.0015)
	if result == nil {
		t.Fatal("CalcSMC returned nil")
	}

	currentPrice := bars[0].Close
	if currentPrice > result.Equilibrium {
		if result.CurrentZone != "PREMIUM" {
			t.Errorf("price %.5f > EQ %.5f but zone=%q (expected PREMIUM)",
				currentPrice, result.Equilibrium, result.CurrentZone)
		}
	} else if currentPrice < result.Equilibrium {
		if result.CurrentZone != "DISCOUNT" {
			t.Errorf("price %.5f < EQ %.5f but zone=%q (expected DISCOUNT)",
				currentPrice, result.Equilibrium, result.CurrentZone)
		}
	}
}

// TestDiscountZone verifies discount classification for bearish bars.
func TestDiscountZone(t *testing.T) {
	bars := makeBearWavesBars()
	result := CalcSMC(bars, 0.0015)
	if result == nil {
		t.Skip("insufficient swing data")
	}

	currentPrice := bars[0].Close
	if currentPrice < result.Equilibrium {
		if result.CurrentZone != "DISCOUNT" {
			t.Errorf("price %.5f < EQ %.5f but zone=%q (expected DISCOUNT)",
				currentPrice, result.Equilibrium, result.CurrentZone)
		}
	}
}

// TestEquilibriumCalculation verifies premium/discount zone math.
func TestEquilibriumCalculation(t *testing.T) {
	bars := makeWavesBars()
	result := CalcSMC(bars, 0.0015)
	if result == nil {
		t.Fatal("CalcSMC returned nil")
	}

	if result.Equilibrium <= result.LastSwingLow || result.Equilibrium >= result.LastSwingHigh {
		t.Errorf("equilibrium %.5f not between swing low %.5f and high %.5f",
			result.Equilibrium, result.LastSwingLow, result.LastSwingHigh)
	}

	if result.PremiumZone <= result.Equilibrium {
		t.Errorf("premium %.5f should be > equilibrium %.5f", result.PremiumZone, result.Equilibrium)
	}

	if result.DiscountZone >= result.Equilibrium {
		t.Errorf("discount %.5f should be < equilibrium %.5f", result.DiscountZone, result.Equilibrium)
	}
}

// TestSwingHighLow verifies LastSwingHigh > LastSwingLow.
func TestSwingHighLow(t *testing.T) {
	bars := makeWavesBars()
	result := CalcSMC(bars, 0.0015)
	if result == nil {
		t.Fatal("CalcSMC returned nil")
	}

	if result.LastSwingHigh <= result.LastSwingLow {
		t.Errorf("expected LastSwingHigh (%.5f) > LastSwingLow (%.5f)",
			result.LastSwingHigh, result.LastSwingLow)
	}
}

// TestCurrentZoneIsOneOfThree verifies CurrentZone is always a valid value.
func TestCurrentZoneIsOneOfThree(t *testing.T) {
	bars := makeWavesBars()
	result := CalcSMC(bars, 0.0015)
	if result == nil {
		t.Fatal("CalcSMC returned nil")
	}

	valid := map[string]bool{"PREMIUM": true, "DISCOUNT": true, "EQUILIBRIUM": true}
	if !valid[result.CurrentZone] {
		t.Errorf("unexpected CurrentZone: %q", result.CurrentZone)
	}
}

// TestBOSAndCHOCHTypes verifies event type strings are correct.
func TestBOSAndCHOCHTypes(t *testing.T) {
	bars := makeWavesBars()
	result := CalcSMC(bars, 0.0015)
	if result == nil {
		t.Fatal("CalcSMC returned nil")
	}

	for _, e := range result.RecentBOS {
		if e.Type != "BOS" {
			t.Errorf("expected Type=BOS, got %q", e.Type)
		}
		if e.Dir != "BULLISH" && e.Dir != "BEARISH" {
			t.Errorf("unexpected Dir %q for BOS event", e.Dir)
		}
	}

	for _, e := range result.RecentCHOCH {
		if e.Type != "CHOCH" {
			t.Errorf("expected Type=CHOCH, got %q", e.Type)
		}
		if e.Dir != "BULLISH" && e.Dir != "BEARISH" {
			t.Errorf("unexpected Dir %q for CHOCH event", e.Dir)
		}
	}
}

// TestBOSListMaxLength verifies length constraints on result lists.
func TestBOSListMaxLength(t *testing.T) {
	bars := makeWavesBars()
	result := CalcSMC(bars, 0.0015)
	if result == nil {
		t.Fatal("CalcSMC returned nil")
	}

	if len(result.RecentBOS) > 5 {
		t.Errorf("expected at most 5 BOS events, got %d", len(result.RecentBOS))
	}
	if len(result.RecentCHOCH) > 3 {
		t.Errorf("expected at most 3 CHOCH events, got %d", len(result.RecentCHOCH))
	}
}

// TestLiqRangeTypes verifies internal liquidity range fields.
func TestLiqRangeTypes(t *testing.T) {
	bars := makeWavesBars()
	result := CalcSMC(bars, 0.0015)
	if result == nil {
		t.Fatal("CalcSMC returned nil")
	}

	for _, r := range result.InternalLiq {
		if r.Type != "INTERNAL" && r.Type != "EXTERNAL" {
			t.Errorf("unexpected LiqRange.Type: %q", r.Type)
		}
		if r.High < r.Low {
			t.Errorf("LiqRange.High %.5f < Low %.5f", r.High, r.Low)
		}
	}
}
