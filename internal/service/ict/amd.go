package ict

import (
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// sessionBounds defines the UTC hour boundaries for each trading session.
type sessionBounds struct {
	Name     string
	StartUTC int
	EndUTC   int
}

var sessions = []sessionBounds{
	{"ASIAN", 0, 7},
	{"LONDON", 7, 13},
	{"NEW_YORK", 13, 21},
}

// DetectAMD identifies the Power of 3 (AMD) phase for each trading session
// and for higher timeframe periods (Weekly, Daily).
//
// Multi-TF PO3 logic:
//   - Weekly PO3: looks at daily bars to find the week's AMD cycle
//   - Daily PO3: looks at intraday bars to find the day's AMD cycle
//   - Session PO3: looks at session bars for intra-session AMD
//
// Logic per period:
//  1. Accumulation: narrow range in the first ~1/3 of the period.
//  2. Manipulation: a false breakout beyond the accumulation range
//     (price wicks above/below the range but closes back inside).
//  3. Distribution: a strong move in the opposite direction of the
//     manipulation, breaking out of the accumulation range.
func DetectAMD(bars []ta.OHLCV, timeframe string) []AMDPhase {
	if len(bars) < 10 {
		return nil
	}

	var phases []AMDPhase

	// --- Weekly PO3 (from daily bars or higher) ---
	if timeframe == "D1" || timeframe == "daily" || timeframe == "1d" {
		weeklyPhase := detectWeeklyPO3(bars)
		if weeklyPhase != nil {
			phases = append(phases, *weeklyPhase)
		}
	}

	// --- Daily PO3 (from intraday bars) ---
	if timeframe != "daily" && timeframe != "1d" {
		dailyPhase := detectDailyPO3(bars)
		if dailyPhase != nil {
			phases = append(phases, *dailyPhase)
		}
	}

	// --- Session PO3 (from intraday bars) ---
	if timeframe != "daily" && timeframe != "1d" {
		for _, sess := range sessions {
			var sessionBars []ta.OHLCV
			for _, bar := range bars {
				h := bar.Date.UTC().Hour()
				if h >= sess.StartUTC && h < sess.EndUTC {
					sessionBars = append(sessionBars, bar)
				}
				if len(sessionBars) > 0 && h < sess.StartUTC {
					break
				}
			}

			if len(sessionBars) < 3 {
				continue
			}

			phase := classifyAMD(sessionBars, sess.Name, "SESSION")
			phases = append(phases, phase)
		}
	}

	return phases
}

// detectWeeklyPO3 detects the Power of 3 cycle for the current week.
// Uses daily candle logic: first 1-2 days = accumulation, mid-week = manipulation, end = distribution.
func detectWeeklyPO3(bars []ta.OHLCV) *AMDPhase {
	if len(bars) < 5 {
		return nil
	}

	// Collect bars from the current week
	var weekBars []ta.OHLCV
	currentWeek := bars[0].Date

	for _, bar := range bars {
		if isSameWeek(bar.Date, currentWeek) {
			weekBars = append(weekBars, bar)
		} else {
			break
		}
	}

	if len(weekBars) < 3 {
		return nil
	}

	phase := classifyAMD(weekBars, "WEEKLY", "WEEKLY")
	return &phase
}

// detectDailyPO3 detects the Power of 3 cycle for the current day.
// Uses intraday bar logic: first third = accumulation, mid = manipulation, end = distribution.
func detectDailyPO3(bars []ta.OHLCV) *AMDPhase {
	if len(bars) < 5 {
		return nil
	}

	// Collect bars from the current day
	var dayBars []ta.OHLCV
	currentDay := bars[0].Date

	for _, bar := range bars {
		if isSameDay(bar.Date, currentDay) {
			dayBars = append(dayBars, bar)
		} else {
			break
		}
	}

	if len(dayBars) < 5 {
		return nil
	}

	phase := classifyAMD(dayBars, "DAILY", "DAILY")
	return &phase
}

// classifyAMD determines the AMD phase for a given period's bars.
func classifyAMD(periodBars []ta.OHLCV, sessionName, tfLevel string) AMDPhase {
	n := len(periodBars)

	// Compute full period range.
	rangeHigh := periodBars[0].High
	rangeLow := periodBars[0].Low
	for _, b := range periodBars {
		if b.High > rangeHigh {
			rangeHigh = b.High
		}
		if b.Low < rangeLow {
			rangeLow = b.Low
		}
	}

	// Determine the date of this period (use the first bar's date)
	var periodDate time.Time
	if len(periodBars) > 0 {
		periodDate = periodBars[0].Date
	}

	result := AMDPhase{
		Session:   sessionName,
		TF:        tfLevel,
		RangeHigh: rangeHigh,
		RangeLow:  rangeLow,
		Date:      periodDate,
	}

	// Split period into first third (accumulation) and remainder.
	thirdIdx := n / 3
	if thirdIdx < 1 {
		thirdIdx = 1
	}

	// Accumulation range from first third.
	accHigh := periodBars[0].High
	accLow := periodBars[0].Low
	for i := 0; i < thirdIdx && i < n; i++ {
		if periodBars[i].High > accHigh {
			accHigh = periodBars[i].High
		}
		if periodBars[i].Low < accLow {
			accLow = periodBars[i].Low
		}
	}
	accRange := accHigh - accLow

	result.AccHigh = accHigh
	result.AccLow = accLow

	// Look at the remainder for manipulation + distribution.
	manipulationUp := false
	manipulationDown := false
	distributionUp := false
	distributionDown := false

	for i := thirdIdx; i < n; i++ {
		b := periodBars[i]

		if b.High > accHigh && b.Close < accHigh && accRange > 0 {
			manipulationUp = true
		}
		if b.Low < accLow && b.Close > accLow && accRange > 0 {
			manipulationDown = true
		}

		if b.Close > accHigh {
			distributionUp = true
		}
		if b.Close < accLow {
			distributionDown = true
		}
	}

	switch {
	case distributionUp && manipulationDown:
		result.Phase = "DISTRIBUTION"
		result.Direction = "BULLISH"
	case distributionDown && manipulationUp:
		result.Phase = "DISTRIBUTION"
		result.Direction = "BEARISH"
	case manipulationUp || manipulationDown:
		result.Phase = "MANIPULATION"
		if manipulationUp {
			result.Direction = "BEARISH"
		} else {
			result.Direction = "BULLISH"
		}
	default:
		result.Phase = "ACCUMULATION"
		result.Direction = "NEUTRAL"
	}

	return result
}
