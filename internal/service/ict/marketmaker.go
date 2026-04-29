package ict

import (
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// DetectMarketMakerModels identifies the three ICT Market Maker models
// from the given bar data.
//
//   - MMD (Market Maker Day): Based on the current day's price action.
//     If the day's range is narrow and price is near the open → Accumulation.
//     If price swept a prior high/low → Manipulation.
//     If price broke out directionally → Distribution.
//
//   - MMW (Market Maker Week): Based on the day of the week.
//     Mon–Tue: Accumulation, Wed: Manipulation, Thu–Fri: Distribution.
//
//   - MMS (Market Maker Session): Based on the current session's
//     premium/discount positioning. Premium = sell model, Discount = buy model.
func DetectMarketMakerModels(bars []ta.OHLCV, bias string, premiumZone, discountZone bool) []MarketMakerModel {
	if len(bars) < 5 {
		return nil
	}

	var models []MarketMakerModel

	// MMD — Market Maker Day
	if mmd := detectMMD(bars); mmd != nil {
		models = append(models, *mmd)
	}

	// MMW — Market Maker Week
	if mmw := detectMMW(bars); mmw != nil {
		models = append(models, *mmw)
	}

	// MMS — Market Maker Session
	mms := detectMMS(bars, bias, premiumZone, discountZone)
	if mms != nil {
		models = append(models, *mms)
	}

	return models
}

// detectMMD determines the Market Maker Day model.
func detectMMD(bars []ta.OHLCV) *MarketMakerModel {
	// Use the most recent day's bars.
	// Bars are newest-first; collect bars from the same day.
	today := bars[0].Date.UTC().Truncate(24 * time.Hour)
	var dayBars []ta.OHLCV
	for _, b := range bars {
		barDay := b.Date.UTC().Truncate(24 * time.Hour)
		if barDay.Before(today) {
			break
		}
		dayBars = append(dayBars, b)
	}

	if len(dayBars) < 3 {
		return nil
	}

	dayHigh := dayBars[0].High
	dayLow := dayBars[0].Low
	dayOpen := dayBars[len(dayBars)-1].Open
	if dayOpen == 0 {
		dayOpen = dayBars[len(dayBars)-1].Close
	}

	for _, b := range dayBars {
		if b.High > dayHigh {
			dayHigh = b.High
		}
		if b.Low < dayLow {
			dayLow = b.Low
		}
	}

	dayRange := dayHigh - dayLow
	currentClose := dayBars[0].Close

	mmd := &MarketMakerModel{
		Model:     "MMD",
		RangeHigh: dayHigh,
		RangeLow:  dayLow,
	}

	// Determine phase based on price position relative to day's range.
	// Narrow range + near open → Accumulation.
	// Sweep beyond prior range → Manipulation.
	// Strong close above/below → Distribution.
	closeToOpen := abs64(currentClose-dayOpen) < dayRange*0.2
	narrowRange := dayRange > 0 && len(dayBars) > 5 && dayRange < (dayBars[0].High-dayBars[0].Low)*2

	switch {
	case closeToOpen && narrowRange:
		mmd.Phase = "ACCUMULATION"
		mmd.Direction = "NEUTRAL"
	case currentClose > dayHigh-dayRange*0.1:
		mmd.Phase = "DISTRIBUTION"
		mmd.Direction = "BULLISH"
	case currentClose < dayLow+dayRange*0.1:
		mmd.Phase = "DISTRIBUTION"
		mmd.Direction = "BEARISH"
	default:
		mmd.Phase = "MANIPULATION"
		// Direction based on which side was swept.
		if currentClose > dayOpen {
			mmd.Direction = "BULLISH"
		} else {
			mmd.Direction = "BEARISH"
		}
	}

	return mmd
}

// detectMMW determines the Market Maker Week model based on the weekday.
func detectMMW(bars []ta.OHLCV) *MarketMakerModel {
	if len(bars) == 0 {
		return nil
	}

	weekday := bars[0].Date.UTC().Weekday()

	// Compute weekly range so far (from Monday).
	var weekBars []ta.OHLCV
	monday := bars[0].Date.UTC()
	for monday.Weekday() != time.Monday {
		monday = monday.AddDate(0, 0, -1)
	}
	for _, b := range bars {
		if b.Date.UTC().Before(monday) {
			break
		}
		weekBars = append(weekBars, b)
	}

	weekHigh := bars[0].High
	weekLow := bars[0].Low
	for _, b := range weekBars {
		if b.High > weekHigh {
			weekHigh = b.High
		}
		if b.Low < weekLow {
			weekLow = b.Low
		}
	}

	mmw := &MarketMakerModel{
		Model:     "MMW",
		RangeHigh: weekHigh,
		RangeLow:  weekLow,
	}

	switch weekday {
	case time.Monday, time.Tuesday:
		mmw.Phase = "ACCUMULATION"
		mmw.Direction = "NEUTRAL"
	case time.Wednesday:
		mmw.Phase = "MANIPULATION"
		mmw.Direction = "NEUTRAL"
	case time.Thursday, time.Friday:
		mmw.Phase = "DISTRIBUTION"
		// Direction based on weekly close relative to range.
		currentClose := bars[0].Close
		equilibrium := (weekHigh + weekLow) / 2
		if currentClose > equilibrium {
			mmw.Direction = "BULLISH"
		} else {
			mmw.Direction = "BEARISH"
		}
	default: // weekend
		mmw.Phase = "ACCUMULATION"
		mmw.Direction = "NEUTRAL"
	}

	return mmw
}

// detectMMS determines the Market Maker Session model.
func detectMMS(bars []ta.OHLCV, bias string, premiumZone, discountZone bool) *MarketMakerModel {
	if len(bars) == 0 {
		return nil
	}

	mms := &MarketMakerModel{
		Model: "MMS",
	}

	// Determine session range.
	// Use the bars from the current session.
	h := bars[0].Date.UTC().Hour()
	var sessionName string
	switch {
	case h >= 0 && h < 7:
		sessionName = "ASIAN"
	case h >= 7 && h < 13:
		sessionName = "LONDON"
	default:
		sessionName = "NEW_YORK"
	}

	// Collect session bars.
	var sessionBars []ta.OHLCV
	for _, b := range bars {
		bh := b.Date.UTC().Hour()
		// Simple heuristic: same session block.
		inSession := false
		switch sessionName {
		case "ASIAN":
			inSession = bh >= 0 && bh < 7
		case "LONDON":
			inSession = bh >= 7 && bh < 13
		case "NEW_YORK":
			inSession = bh >= 13
		}
		if !inSession {
			break
		}
		sessionBars = append(sessionBars, b)
	}

	if len(sessionBars) > 0 {
		mms.RangeHigh = sessionBars[0].High
		mms.RangeLow = sessionBars[0].Low
		for _, b := range sessionBars {
			if b.High > mms.RangeHigh {
				mms.RangeHigh = b.High
			}
			if b.Low < mms.RangeLow {
				mms.RangeLow = b.Low
			}
		}
	}

	// MMS direction based on premium/discount zone.
	switch {
	case premiumZone:
		mms.Phase = "DISTRIBUTION"
		mms.Direction = "BEARISH" // premium = sell model
	case discountZone:
		mms.Phase = "ACCUMULATION"
		mms.Direction = "BULLISH" // discount = buy model
	default:
		mms.Phase = "MANIPULATION"
		mms.Direction = bias
	}

	return mms
}
