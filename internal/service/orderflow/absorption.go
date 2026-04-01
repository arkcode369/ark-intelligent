package orderflow

import (
	"math"
)

// absorptionThresholds defines the criteria for an absorption pattern.
const (
	// A bar is high-volume if its volume exceeds avgVolume × absorptionVolMult.
	absorptionVolMult = 1.5
	// A bar has "limited range" if its HL range is smaller than avgRange × absorptionRangePct.
	absorptionRangePct = 0.75
)

// detectAbsorptions scans DeltaBars (newest-first) and returns indices where
// bullish or bearish absorption patterns are detected.
//
// Bullish Absorption:
//   A bearish bar (delta < 0) with high volume but a small HL range.
//   Interpretation: Sellers are aggressively selling, but buyers absorb the supply
//   — price doesn't fall much despite the selling pressure.
//
// Bearish Absorption:
//   A bullish bar (delta > 0) with high volume but a small HL range.
//   Interpretation: Buyers are aggressively buying, but sellers absorb the demand
//   — price doesn't rise much despite the buying pressure.
func detectAbsorptions(bars []DeltaBar) (bullishAbs, bearishAbs []int) {
	if len(bars) < 3 {
		return nil, nil
	}

	// Compute average volume and average HL range.
	var sumVol, sumRange float64
	for _, b := range bars {
		sumVol += b.OHLCV.Volume
		sumRange += b.OHLCV.High - b.OHLCV.Low
	}
	avgVol := sumVol / float64(len(bars))
	avgRange := sumRange / float64(len(bars))

	for i, b := range bars {
		hlRange := b.OHLCV.High - b.OHLCV.Low
		isHighVol := b.OHLCV.Volume > avgVol*absorptionVolMult
		isSmallRange := hlRange < avgRange*absorptionRangePct && avgRange > 1e-10

		if !isHighVol || !isSmallRange {
			continue
		}

		if b.Delta < 0 {
			// Selling pressure absorbed by buyers.
			bullishAbs = append(bullishAbs, i)
		} else if b.Delta > 0 {
			// Buying pressure absorbed by sellers.
			bearishAbs = append(bearishAbs, i)
		}
	}
	return bullishAbs, bearishAbs
}

// detectDivergence checks for price–delta divergence across the lookback window.
//
// Bearish Divergence: price makes a higher high, but cumulative delta makes a lower high.
// Bullish Divergence: price makes a lower low, but cumulative delta makes a higher low.
//
// The function compares the most recent bar against the prior bars using a simplified
// two-pass approach: split the bars (newest-first) into recent half vs older half.
func detectDivergence(bars []DeltaBar) string {
	n := len(bars)
	if n < 6 {
		return "NONE"
	}

	// newest-first; bars[0] is most recent.
	half := n / 2

	// Recent window: bars[0..half-1]  (newer)
	// Prior window:  bars[half..n-1]  (older)

	recentHigh := bars[0].OHLCV.High
	recentLow := bars[0].OHLCV.Low
	recentCumDelta := bars[0].CumDelta
	for i := 1; i < half; i++ {
		recentHigh = math.Max(recentHigh, bars[i].OHLCV.High)
		recentLow = math.Min(recentLow, bars[i].OHLCV.Low)
		recentCumDelta = math.Max(recentCumDelta, bars[i].CumDelta)
	}
	recentCumDeltaLow := bars[0].CumDelta
	for i := 1; i < half; i++ {
		recentCumDeltaLow = math.Min(recentCumDeltaLow, bars[i].CumDelta)
	}

	priorHigh := bars[half].OHLCV.High
	priorLow := bars[half].OHLCV.Low
	priorCumDelta := bars[half].CumDelta
	for i := half + 1; i < n; i++ {
		priorHigh = math.Max(priorHigh, bars[i].OHLCV.High)
		priorLow = math.Min(priorLow, bars[i].OHLCV.Low)
		priorCumDelta = math.Max(priorCumDelta, bars[i].CumDelta)
	}
	priorCumDeltaLow := bars[half].CumDelta
	for i := half + 1; i < n; i++ {
		priorCumDeltaLow = math.Min(priorCumDeltaLow, bars[i].CumDelta)
	}

	// Bearish divergence: price higher high, delta lower high.
	if recentHigh > priorHigh && recentCumDelta < priorCumDelta {
		return "BEARISH_DIV"
	}

	// Bullish divergence: price lower low, delta higher low (less negative).
	if recentLow < priorLow && recentCumDeltaLow > priorCumDeltaLow {
		return "BULLISH_DIV"
	}

	return "NONE"
}
