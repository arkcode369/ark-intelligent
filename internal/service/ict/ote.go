package ict

import (
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// DetectOTE identifies Optimal Trade Entry zones from swing points and bars.
//
// For each impulse leg between consecutive swing points, it computes the
// 62% and 79% Fibonacci retracement levels. The zone between these two
// levels is the OTE — the "sweet spot" for entries in the direction of
// the impulse.
//
// Only the most recent 3 OTE zones are returned (to avoid clutter).
func DetectOTE(bars []ta.OHLCV, swings []swingPoint) []OTE {
	if len(swings) < 2 || len(bars) < 15 {
		return nil
	}

	var zones []OTE

	// Walk through consecutive swing points to find impulse legs.
	for i := 1; i < len(swings); i++ {
		prev := swings[i-1]
		curr := swings[i]

		// Bullish impulse: swing low → swing high
		if !prev.isHigh && curr.isHigh && curr.level > prev.level {
			zones = append(zones, calcOTE("BULLISH", prev.level, curr.level))
		}

		// Bearish impulse: swing high → swing low
		if prev.isHigh && !curr.isHigh && curr.level < prev.level {
			zones = append(zones, calcOTE("BEARISH", prev.level, curr.level))
		}
	}

	// Keep only the most recent 3 zones.
	if len(zones) > 3 {
		zones = zones[len(zones)-3:]
	}

	return zones
}

// DetectOTEFromAnchors computes OTE from the most relevant high/low anchors.
// This is the preferred method — it uses the most significant swing points
// as the impulse leg, ensuring the OTE zone is drawn from the most
// relevant structural reference points.
//
// If both anchors exist and form a valid impulse leg, one OTE zone is computed.
// Additional zones from recent swing impulses are also included.
func DetectOTEFromAnchors(bars []ta.OHLCV, swings []swingPoint, relHigh, relLow *RelevantAnchor) []OTE {
	if len(bars) < 15 {
		return nil
	}

	var zones []OTE

	// Primary OTE from relevant anchors.
	if relHigh != nil && relLow != nil && relHigh.Level > relLow.Level {
		// Determine direction: if high is more recent → bullish impulse (low→high)
		// if low is more recent → bearish impulse (high→low)
		if relHigh.BarIndex >= relLow.BarIndex {
			// Bullish impulse: low → high
			zones = append(zones, calcOTE("BULLISH", relLow.Level, relHigh.Level))
		} else {
			// Bearish impulse: high → low
			zones = append(zones, calcOTE("BEARISH", relHigh.Level, relLow.Level))
		}
	}

	// Add secondary OTEs from recent swing impulses (for confluence).
	if len(swings) >= 2 {
		// Only look at the most recent few swings for secondary zones.
		recentSwings := swings
		if len(recentSwings) > 8 {
			recentSwings = recentSwings[len(recentSwings)-8:]
		}

		for i := 1; i < len(recentSwings); i++ {
			prev := recentSwings[i-1]
			curr := recentSwings[i]

			var ote OTE
			isImpulse := false

			if !prev.isHigh && curr.isHigh && curr.level > prev.level {
				ote = calcOTE("BULLISH", prev.level, curr.level)
				isImpulse = true
			}
			if prev.isHigh && !curr.isHigh && curr.level < prev.level {
				ote = calcOTE("BEARISH", prev.level, curr.level)
				isImpulse = true
			}

			if isImpulse {
				// Skip if this duplicates the anchor-based OTE.
				duplicate := false
				for _, z := range zones {
					if abs64(z.High-ote.High) < 0.0001 && abs64(z.Low-ote.Low) < 0.0001 {
						duplicate = true
						break
					}
				}
				if !duplicate {
					zones = append(zones, ote)
				}
			}
		}
	}

	// Keep only the most recent 3 zones.
	if len(zones) > 3 {
		zones = zones[len(zones)-3:]
	}

	return zones
}

// calcOTE computes the OTE zone from an impulse leg's start and end prices.
//
// Fibonacci levels:
//   - 62% retracement = upper bound of the OTE zone
//   - 79% retracement = lower bound of the OTE zone
//   - 70.5% = midpoint (sweet spot)
//
// For a BULLISH impulse (low→high), retracement is measured DOWN from the high.
// For a BEARISH impulse (high→low), retracement is measured UP from the low.
func calcOTE(direction string, start, end float64) OTE {
	impulseSize := end - start

	var oteHigh, oteLow, midpoint float64
	switch direction {
	case "BULLISH":
		// Retracement down from the high: 62% and 79% of impulse
		oteHigh = end - 0.618*impulseSize // 62% retracement
		oteLow = end - 0.786*impulseSize  // 79% retracement (commonly 78.6%)
		midpoint = end - 0.705*impulseSize // sweet spot
	case "BEARISH":
		// Retracement up from the low: 62% and 79% of impulse
		oteLow = start + 0.618*impulseSize // 62% retracement
		oteHigh = start + 0.786*impulseSize // 79% retracement
		midpoint = start + 0.705*impulseSize // sweet spot
	}

	return OTE{
		Direction:    direction,
		High:         oteHigh,
		Low:          oteLow,
		Midpoint:     midpoint,
		ImpulseStart: start,
		ImpulseEnd:   end,
	}
}
