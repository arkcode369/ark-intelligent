package orderflow

import (
	"math"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// estimateDelta applies the tick-rule volume approximation to a single OHLCV bar.
//
// Logic (OHLCV tick-rule):
//   Bullish bar (close >= open):
//     BuyVol  = Volume × (Close - Low) / (High - Low)
//     SellVol = Volume - BuyVol
//   Bearish bar (close < open):
//     SellVol = Volume × (High - Close) / (High - Low)
//     BuyVol  = Volume - SellVol
//   Doji / zero range:
//     BuyVol = SellVol = Volume / 2
//
// The estimates are directionally consistent with the bar's price action.
func estimateDelta(bar ta.OHLCV) (buyVol, sellVol, delta float64) {
	rangeHL := bar.High - bar.Low
	if rangeHL < 1e-10 || bar.Volume < 1e-10 {
		// Doji or zero-volume bar: split evenly.
		half := bar.Volume / 2
		return half, half, 0
	}

	if bar.Close >= bar.Open {
		// Bullish: more buying pressure near the top.
		buyVol = bar.Volume * (bar.Close - bar.Low) / rangeHL
	} else {
		// Bearish: more selling pressure near the bottom.
		sellVol = bar.Volume * (bar.High - bar.Close) / rangeHL
		buyVol = bar.Volume - sellVol
		return buyVol, sellVol, buyVol - sellVol
	}

	sellVol = bar.Volume - buyVol
	delta = buyVol - sellVol
	return buyVol, sellVol, delta
}

// buildDeltaBars converts a slice of OHLCV bars (newest-first) into DeltaBar slices,
// computing per-bar delta and cumulative delta.
// The returned slice preserves the same newest-first ordering.
func buildDeltaBars(bars []ta.OHLCV) []DeltaBar {
	if len(bars) == 0 {
		return nil
	}

	// Compute per-bar delta (newest-first).
	result := make([]DeltaBar, len(bars))
	for i, b := range bars {
		bv, sv, d := estimateDelta(b)
		result[i] = DeltaBar{
			OHLCV:   b,
			BuyVol:  bv,
			SellVol: sv,
			Delta:   d,
		}
	}

	// Cumulative delta is computed oldest-to-newest, so reverse-iterate.
	var cum float64
	for i := len(result) - 1; i >= 0; i-- {
		cum += result[i].Delta
		result[i].CumDelta = cum
	}

	return result
}

// deltaTrend classifies the overall direction of cumulative delta.
// It compares the first half average vs the second half average of the bars (oldest-first view).
func deltaTrend(bars []DeltaBar) string {
	n := len(bars)
	if n < 4 {
		return "FLAT"
	}

	// bars is newest-first; oldest bars are at high index.
	half := n / 2
	// First half (older): indices [n-half..n-1]
	// Second half (newer): indices [0..half-1]
	var oldSum, newSum float64
	for i := n - half; i < n; i++ {
		oldSum += bars[i].Delta
	}
	for i := 0; i < half; i++ {
		newSum += bars[i].Delta
	}

	oldAvg := oldSum / float64(half)
	newAvg := newSum / float64(half)

	threshold := math.Abs(oldAvg) * 0.25 // 25% shift counts as trend
	if newAvg > oldAvg+threshold {
		return "RISING"
	} else if newAvg < oldAvg-threshold {
		return "FALLING"
	}
	return "FLAT"
}
