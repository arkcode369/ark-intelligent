package ict

import (
	"math"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// swingLookbackForTF returns the number of bars on each side used to confirm
// a swing point, dynamically adjusted based on the timeframe.
//
// Lower timeframes need more bars to filter noise;
// higher timeframes need fewer bars since each bar is more significant.
func swingLookbackForTF(timeframe string) int {
	switch timeframe {
	case "15M", "15m":
		return 12
	case "30M", "30m":
		return 10
	case "H1", "1h", "1H":
		return 8
	case "H4", "4h", "4H":
		return 6
	case "H6", "6h", "6H":
		return 5
	case "H12", "12h", "12H":
		return 4
	case "D1", "daily", "1d":
		return 5
	case "W1", "weekly", "1w":
		return 3
	default:
		return 5
	}
}

// detectSwings identifies swing highs and lows using the default lookback (5).
// Deprecated: Use detectSwingsTF for dynamic lookback.
func detectSwings(bars []ta.OHLCV) []swingPoint {
	return detectSwingsWithLookback(bars, 5)
}

// detectSwingsTF detects swing points with dynamic lookback based on timeframe.
func detectSwingsTF(bars []ta.OHLCV, timeframe string) []swingPoint {
	return detectSwingsWithLookback(bars, swingLookbackForTF(timeframe))
}

// detectSwingsWithLookback detects swing points using the specified lookback window.
func detectSwingsWithLookback(bars []ta.OHLCV, lookback int) []swingPoint {
	n := len(bars)
	if n < 2*lookback+1 {
		return nil
	}

	// Work on a chronological copy (oldest first).
	chron := make([]ta.OHLCV, n)
	for i, b := range bars {
		chron[n-1-i] = b
	}

	var swings []swingPoint

	for i := lookback; i < n-lookback; i++ {
		// Swing High: bar[i].High is the highest in the window.
		isSwingHigh := true
		for j := i - lookback; j <= i+lookback; j++ {
			if j == i {
				continue
			}
			if chron[j].High >= chron[i].High {
				isSwingHigh = false
				break
			}
		}
		if isSwingHigh {
			swings = append(swings, swingPoint{
				isHigh:   true,
				level:    chron[i].High,
				barIndex: i,
				date:     chron[i].Date,
			})
		}

		// Swing Low: bar[i].Low is the lowest in the window.
		isSwingLow := true
		for j := i - lookback; j <= i+lookback; j++ {
			if j == i {
				continue
			}
			if chron[j].Low <= chron[i].Low {
				isSwingLow = false
				break
			}
		}
		if isSwingLow {
			swings = append(swings, swingPoint{
				isHigh:   false,
				level:    chron[i].Low,
				barIndex: i,
				date:     chron[i].Date,
			})
		}
	}

	return swings
}

// detectRelevantAnchors finds the most significant swing high and swing low
// from the swing points, scored by how much they stand out relative to ATR.
//
// The "relevant" anchor is the swing that:
//   - Is the most recent unbroken swing in its direction
//   - Has the highest ATR-multiple significance score
//
// This is critical for ICT: P&D zones, OTE, and structure all reference
// the most relevant high/low, not just any swing.
func detectRelevantAnchors(bars []ta.OHLCV, swings []swingPoint, atr float64) (*RelevantAnchor, *RelevantAnchor) {
	if len(swings) == 0 || atr <= 0 {
		return nil, nil
	}

	n := len(bars)

	var bestHigh, bestLow *RelevantAnchor

	for _, sw := range swings {
		score := swingSignificance(sw, swings, atr)

		if sw.isHigh {
			anchor := &RelevantAnchor{
				Type:        "HIGH",
				Level:       sw.level,
				BarIndex:    sw.barIndex,
				Date:        sw.date,
				ATRMultiple: score,
			}
			if bestHigh == nil || score > bestHigh.ATRMultiple ||
				(score == bestHigh.ATRMultiple && sw.barIndex > bestHigh.BarIndex) {
				if !isSwingBroken(bars, sw, n) {
					bestHigh = anchor
				}
			}
		} else {
			anchor := &RelevantAnchor{
				Type:        "LOW",
				Level:       sw.level,
				BarIndex:    sw.barIndex,
				Date:        sw.date,
				ATRMultiple: score,
			}
			if bestLow == nil || score > bestLow.ATRMultiple ||
				(score == bestLow.ATRMultiple && sw.barIndex > bestLow.BarIndex) {
				if !isSwingBroken(bars, sw, n) {
					bestLow = anchor
				}
			}
		}
	}

	return bestHigh, bestLow
}

// swingSignificance computes how significant a swing point is relative to ATR.
// A swing that stands out more ATRs from its nearest opposite swings is more significant.
func swingSignificance(sw swingPoint, allSwings []swingPoint, atr float64) float64 {
	if atr <= 0 {
		return 0
	}

	// Find the nearest opposite swing points (by barIndex proximity).
	var nearestBeforeLevel, nearestAfterLevel float64
	closestBeforeDist := -1
	closestAfterDist := -1

	for _, s := range allSwings {
		if s.isHigh == sw.isHigh {
			continue // same direction, skip
		}
		if s.barIndex < sw.barIndex {
			dist := sw.barIndex - s.barIndex
			if closestBeforeDist < 0 || dist < closestBeforeDist {
				closestBeforeDist = dist
				nearestBeforeLevel = s.level
			}
		}
		if s.barIndex > sw.barIndex {
			dist := s.barIndex - sw.barIndex
			if closestAfterDist < 0 || dist < closestAfterDist {
				closestAfterDist = dist
				nearestAfterLevel = s.level
			}
		}
	}

	totalDist := 0.0
	count := 0
	if closestBeforeDist >= 0 {
		totalDist += abs64(sw.level - nearestBeforeLevel)
		count++
	}
	if closestAfterDist >= 0 {
		totalDist += abs64(sw.level - nearestAfterLevel)
		count++
	}
	if count == 0 {
		return 0
	}
	return totalDist / float64(count) / atr
}

// isSwingBroken checks if a swing point has been broken by price action
// (a bar closing beyond the swing level after it was formed).
func isSwingBroken(bars []ta.OHLCV, sw swingPoint, n int) bool {
	newestIdx := n - 1 - sw.barIndex
	if newestIdx < 0 || newestIdx >= n {
		return true
	}
	for i := 0; i < newestIdx; i++ {
		if sw.isHigh && bars[i].Close > sw.level {
			return true
		}
		if !sw.isHigh && bars[i].Close < sw.level {
			return true
		}
	}
	return false
}

// maxFloat returns the maximum value in a slice.
func maxFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return math.Inf(-1)
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// minFloat returns the minimum value in a slice.
func minFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return math.Inf(1)
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// abs64 returns the absolute value of a float64.
func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// normalizeTimeframe returns a canonical timeframe string.
func normalizeTimeframe(tf string) string {
	switch tf {
	case "15m", "15M":
		return "15M"
	case "30m", "30M":
		return "30M"
	case "1h", "H1", "1H":
		return "H1"
	case "4h", "H4", "4H":
		return "H4"
	case "6h", "H6", "6H":
		return "H6"
	case "12h", "H12", "12H":
		return "H12"
	case "daily", "1d", "D1":
		return "D1"
	case "weekly", "1w", "W1":
		return "W1"
	default:
		return tf
	}
}

// isSameDay returns true if two times are on the same UTC calendar day.
func isSameDay(a, b time.Time) bool {
	aY, aM, aD := a.UTC().Date()
	bY, bM, bD := b.UTC().Date()
	return aY == bY && aM == bM && aD == bD
}

// isSameWeek returns true if two times are in the same ISO week.
func isSameWeek(a, b time.Time) bool {
	aY, aW := a.UTC().ISOWeek()
	bY, bW := b.UTC().ISOWeek()
	return aY == bY && aW == bW
}

// isSameMonth returns true if two times are in the same UTC calendar month.
func isSameMonth(a, b time.Time) bool {
	aY, aM, _ := a.UTC().Date()
	bY, bM, _ := b.UTC().Date()
	return aY == bY && aM == bM
}
