package ict

import (
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// killzoneDef defines a killzone time window (matching the PineScript reference).
type killzoneDef struct {
	Name     string
	StartUTC int
	EndUTC   int
}

var killzoneDefs = []killzoneDef{
	{"ASIAN", 20, 24},    // 20:00–00:00 UTC (Asia session)
	{"LONDON", 2, 5},     // 02:00–05:00 UTC (London open)
	{"NY_AM", 9, 11},     // 09:30–11:00 UTC (NY AM, adjusted to whole hours)
	{"NY_LUNCH", 12, 13}, // 12:00–13:00 UTC (NY Lunch)
	{"NY_PM", 13, 16},    // 13:30–16:00 UTC (NY PM, adjusted to whole hours)
}

// DetectKillzoneBoxes identifies killzone time windows in the bar data
// and computes the high/low pivot for each window, along with mitigation status.
//
// Like the PineScript reference, each killzone is drawn as a box from
// session start to end, with pivot high/low lines extending until mitigated.
//
// Only the most recent N days of killzone boxes are returned (to avoid clutter).
func DetectKillzoneBoxes(bars []ta.OHLCV, timeframe string) []KillzoneBox {
	if timeframe == "daily" || timeframe == "1d" || len(bars) < 10 {
		return nil
	}

	var boxes []KillzoneBox
	seenDays := make(map[string]bool) // track which days we've processed

	// bars are newest-first
	for _, bar := range bars {
		dayStr := bar.Date.UTC().Format("2006-01-02")
		if seenDays[dayStr] {
			continue
		}
		seenDays[dayStr] = true

		// Limit to 3 most recent days
		if len(seenDays) > 3 {
			break
		}

		dayBoxes := detectKillzoneBoxesForDay(bars, bar.Date)
		boxes = append(boxes, dayBoxes...)
	}

	return boxes
}

// detectKillzoneBoxesForDay detects killzone boxes for a specific day.
func detectKillzoneBoxesForDay(bars []ta.OHLCV, refDate time.Time) []KillzoneBox {
	refDay := refDate.UTC().Format("2006-01-02")

	var boxes []KillzoneBox

	for _, kz := range killzoneDefs {
		var windowBars []ta.OHLCV

		for _, bar := range bars {
			barDay := bar.Date.UTC().Format("2006-01-02")
			if barDay != refDay {
				continue
			}
			h := bar.Date.UTC().Hour()

			// Handle overnight sessions (e.g., ASIAN 20-24)
			if kz.StartUTC > kz.EndUTC {
				// wraps around midnight
				if h >= kz.StartUTC || h < kz.EndUTC {
					windowBars = append(windowBars, bar)
				}
			} else {
				if h >= kz.StartUTC && h < kz.EndUTC {
					windowBars = append(windowBars, bar)
				}
			}
		}

		if len(windowBars) < 2 {
			continue
		}

		// Compute high/low of the window
		kzHigh := windowBars[0].High
		kzLow := windowBars[0].Low
		for _, b := range windowBars {
			if b.High > kzHigh {
				kzHigh = b.High
			}
			if b.Low < kzLow {
				kzLow = b.Low
			}
		}

		// Check if the pivot has been mitigated (price broke above high or below low
		// after the killzone window ended).
		mitigated := false
		lastWindowHour := kz.EndUTC
		if kz.StartUTC > kz.EndUTC {
			lastWindowHour = kz.EndUTC
		}

		for _, bar := range bars {
			barDay := bar.Date.UTC().Format("2006-01-02")
			if barDay != refDay {
				continue
			}
			h := bar.Date.UTC().Hour()
			if h >= lastWindowHour {
				if bar.High > kzHigh || bar.Low < kzLow {
					mitigated = true
					break
				}
			}
		}

		box := KillzoneBox{
			Name:      kz.Name,
			StartUTC:  kz.StartUTC,
			EndUTC:    kz.EndUTC,
			High:      kzHigh,
			Low:       kzLow,
			Mitigated: mitigated,
			Date:      time.Date(refDate.UTC().Year(), refDate.UTC().Month(), refDate.UTC().Day(), 0, 0, 0, 0, time.UTC),
		}
		boxes = append(boxes, box)
	}

	return boxes
}
