package ict

import (
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// silverBulletWindows defines the three ICT Silver Bullet time windows.
var silverBulletWindows = []struct {
	Name     string
	StartUTC int
	EndUTC   int
}{
	{"LONDON_AM", 10, 11},
	{"NY_AM", 14, 15},
	{"NY_PM", 16, 17},
}

// DetectSilverBullets identifies which Silver Bullet windows are currently
// active and whether any unfilled FVGs align with them.
//
// For intraday timeframes, the function checks all three windows and
// captures the high/low price range during each window for chart box drawing.
// For daily timeframes, no Silver Bullet detection is meaningful.
func DetectSilverBullets(bars []ta.OHLCV, fvgs []FVGZone, timeframe string) []SilverBullet {
	// Silver Bullets are only relevant for intraday timeframes.
	if timeframe == "daily" || timeframe == "1d" || timeframe == "D1" {
		return nil
	}

	if len(bars) == 0 {
		return nil
	}

	now := bars[0].Date
	h := now.UTC().Hour()
	today := now.UTC().Format("2006-01-02")

	var bullets []SilverBullet

	for _, w := range silverBulletWindows {
		sb := SilverBullet{
			Window:   w.Name,
			StartUTC: w.StartUTC,
			EndUTC:   w.EndUTC,
			Active:   h >= w.StartUTC && h < w.EndUTC,
			FVGIndex: -1,
			Date:     time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC),
		}

		// Collect bars within this window for today to compute High/Low.
		var windowBars []ta.OHLCV
		for _, bar := range bars {
			barDay := bar.Date.UTC().Format("2006-01-02")
			if barDay != today {
				continue
			}
			bh := bar.Date.UTC().Hour()
			if bh >= w.StartUTC && bh < w.EndUTC {
				windowBars = append(windowBars, bar)
			}
		}

		if len(windowBars) > 0 {
			sb.High = windowBars[0].High
			sb.Low = windowBars[0].Low
			for _, b := range windowBars {
				if b.High > sb.High {
					sb.High = b.High
				}
				if b.Low < sb.Low {
					sb.Low = b.Low
				}
			}
		}

		// If this window is active, look for an unfilled FVG.
		if sb.Active {
			for i, fvg := range fvgs {
				if !fvg.Filled && fvg.FillPct < 50 {
					sb.FVGIndex = i
					break
				}
			}
		}

		bullets = append(bullets, sb)
	}

	return bullets
}
