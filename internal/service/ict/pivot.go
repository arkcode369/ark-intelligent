package ict

import (
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// DetectDWMPivots identifies previous Day/Week/Month High and Low levels
// from the bar data. These are key institutional reference levels.
//
// Like the PineScript reference: PDH/PDL, PWH/PWL, PMH/PML are drawn as
// horizontal lines extending until broken.
func DetectDWMPivots(bars []ta.OHLCV) []DWMPivot {
	if len(bars) < 5 {
		return nil
	}

	var pivots []DWMPivot

	// Current price for broken check
	currentPrice := bars[0].Close

	// --- Previous Day High/Low ---
	pdh, pdl, pdStart, pdEnd := findPreviousPeriodBounds(bars, isSameDay, "day")
	if pdh > 0 {
		pivots = append(pivots, DWMPivot{
			Type:        "PDH",
			Level:       pdh,
			PeriodStart: pdStart,
			PeriodEnd:   pdEnd,
			Broken:      currentPrice > pdh,
		})
	}
	if pdl > 0 && pdl != pdh {
		pivots = append(pivots, DWMPivot{
			Type:        "PDL",
			Level:       pdl,
			PeriodStart: pdStart,
			PeriodEnd:   pdEnd,
			Broken:      currentPrice < pdl,
		})
	}

	// --- Previous Week High/Low ---
	pwh, pwl, pwStart, pwEnd := findPreviousPeriodBounds(bars, isSameWeek, "week")
	if pwh > 0 {
		pivots = append(pivots, DWMPivot{
			Type:        "PWH",
			Level:       pwh,
			PeriodStart: pwStart,
			PeriodEnd:   pwEnd,
			Broken:      currentPrice > pwh,
		})
	}
	if pwl > 0 && pwl != pwh {
		pivots = append(pivots, DWMPivot{
			Type:        "PWL",
			Level:       pwl,
			PeriodStart: pwStart,
			PeriodEnd:   pwEnd,
			Broken:      currentPrice < pwl,
		})
	}

	// --- Previous Month High/Low ---
	pmh, pml, pmStart, pmEnd := findPreviousPeriodBounds(bars, isSameMonth, "month")
	if pmh > 0 {
		pivots = append(pivots, DWMPivot{
			Type:        "PMH",
			Level:       pmh,
			PeriodStart: pmStart,
			PeriodEnd:   pmEnd,
			Broken:      currentPrice > pmh,
		})
	}
	if pml > 0 && pml != pmh {
		pivots = append(pivots, DWMPivot{
			Type:        "PML",
			Level:       pml,
			PeriodStart: pmStart,
			PeriodEnd:   pmEnd,
			Broken:      currentPrice < pml,
		})
	}

	return pivots
}

// findPreviousPeriodBounds finds the high/low of the PREVIOUS period
// (day, week, or month) relative to the most recent bar.
//
// bars are newest-first. We find the current period, then look back
// to the previous period and compute its high/low.
func findPreviousPeriodBounds(bars []ta.OHLCV, samePeriod func(time.Time, time.Time) bool, label string) (float64, float64, time.Time, time.Time) {
	if len(bars) == 0 {
		return 0, 0, time.Time{}, time.Time{}
	}

	// Find the current period's reference time
	currentRef := bars[0].Date

	// Find bars in the current period
	var currentPeriodStart, currentPeriodEnd time.Time
	foundCurrent := false

	for _, bar := range bars {
		if samePeriod(bar.Date, currentRef) {
			if currentPeriodStart.IsZero() || bar.Date.Before(currentPeriodStart) {
				currentPeriodStart = bar.Date
			}
			if currentPeriodEnd.IsZero() || bar.Date.After(currentPeriodEnd) {
				currentPeriodEnd = bar.Date
			}
			foundCurrent = true
		} else if foundCurrent {
			// We've left the current period — now we're in a previous period.
			break
		}
	}

	if !foundCurrent {
		return 0, 0, time.Time{}, time.Time{}
	}

	// Now find the PREVIOUS period (the one just before current)
	var prevPeriodBars []ta.OHLCV
	var prevPeriodStart, prevPeriodEnd time.Time
	inPrevPeriod := false

	for _, bar := range bars {
		if samePeriod(bar.Date, currentRef) {
			// Still in current period, skip
			inPrevPeriod = false
			continue
		}

		// This bar is in a different period
		if !inPrevPeriod {
			// First bar of a new (previous) period
			inPrevPeriod = true
			prevPeriodStart = bar.Date
			prevPeriodEnd = bar.Date
		}

		if inPrevPeriod {
			// Check if this bar is still in the same previous period
			if samePeriod(bar.Date, prevPeriodStart) {
				prevPeriodBars = append(prevPeriodBars, bar)
				if bar.Date.Before(prevPeriodStart) {
					prevPeriodStart = bar.Date
				}
				if bar.Date.After(prevPeriodEnd) {
					prevPeriodEnd = bar.Date
				}
			} else {
				// Moved to yet another period — stop
				break
			}
		}
	}

	if len(prevPeriodBars) == 0 {
		return 0, 0, time.Time{}, time.Time{}
	}

	// Compute high/low of the previous period
	periodHigh := prevPeriodBars[0].High
	periodLow := prevPeriodBars[0].Low
	for _, b := range prevPeriodBars {
		if b.High > periodHigh {
			periodHigh = b.High
		}
		if b.Low < periodLow {
			periodLow = b.Low
		}
	}

	return periodHigh, periodLow, prevPeriodStart, prevPeriodEnd
}
