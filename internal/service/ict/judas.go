package ict

import (
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// judasSession defines the sessions where Judas Swings are most common.
var judasSessions = []struct {
	Name     string
	StartUTC int
	OpenHour int // first hour of the session (for open price)
	EndUTC   int
}{
	{"LONDON", 7, 7, 10},  // London open: 07:00–10:00 UTC
	{"NEW_YORK", 13, 13, 16}, // NY open: 13:00–16:00 UTC
}

// DetectJudasSwings identifies Judas Swing patterns at session opens.
//
// A Judas Swing occurs when:
//  1. At session open, price initially moves in one direction (the "trap").
//  2. Within the first 1-2 hours, price reverses and closes beyond the
//     session open price in the opposite direction.
//
// The "Direction" field indicates the TRUE move direction (after reversal).
// Only intraday timeframes are analyzed.
func DetectJudasSwings(bars []ta.OHLCV, timeframe string) []JudasSwing {
	if timeframe == "daily" || timeframe == "1d" || len(bars) < 6 {
		return nil
	}

	var swings []JudasSwing

	for _, sess := range judasSessions {
		// Collect bars from the first 3 hours of the most recent session.
		var sessionBars []ta.OHLCV
		for _, bar := range bars {
			h := bar.Date.UTC().Hour()
			if h >= sess.StartUTC && h < sess.EndUTC {
				sessionBars = append(sessionBars, bar)
			}
			// Stop once we go past the session.
			if len(sessionBars) > 0 && h < sess.StartUTC {
				break
			}
		}

		if len(sessionBars) < 3 {
			continue
		}

		js := classifyJudasSwing(sessionBars, sess.Name)
		if js != nil {
			swings = append(swings, *js)
		}
	}

	return swings
}

// classifyJudasSwing determines if a Judas Swing occurred in the given session bars.
func classifyJudasSwing(sessionBars []ta.OHLCV, sessionName string) *JudasSwing {
	n := len(sessionBars)

	// Session open price = close of the first bar (or open if available).
	openPrice := sessionBars[n-1].Close // bars are newest-first, so last = oldest
	if sessionBars[n-1].Open != 0 {
		openPrice = sessionBars[n-1].Open
	}

	// Look at the first 1-2 hours (first ~2-3 bars for H1, more for smaller TFs).
	// We need to find the initial move direction and then check for reversal.
	firstMoveBars := n / 2 // use first half for initial move detection
	if firstMoveBars < 2 {
		firstMoveBars = 2
	}
	if firstMoveBars > n-1 {
		firstMoveBars = n - 1
	}

	// Find the extreme of the initial move.
	// Since bars are newest-first, the "first" bars chronologically are at the end.
	initialHigh := sessionBars[n-1].High
	initialLow := sessionBars[n-1].Low
	for i := n - 1; i >= n-firstMoveBars && i >= 0; i-- {
		if sessionBars[i].High > initialHigh {
			initialHigh = sessionBars[i].High
		}
		if sessionBars[i].Low < initialLow {
			initialLow = sessionBars[i].Low
		}
	}

	// Determine initial move direction.
	initialMoveUp := initialHigh-openPrice > openPrice-initialLow

	// Check if later bars (newer, chronologically later) reversed beyond open.
	// These are bars[0..n-firstMoveBars-1] (newest-first).
	reversalUp := false
	reversalDown := false
	for i := 0; i < n-firstMoveBars; i++ {
		if sessionBars[i].Close > openPrice {
			reversalUp = true
		}
		if sessionBars[i].Close < openPrice {
			reversalDown = true
		}
	}

	// Judas Swing: initial move in one direction, then reversal in the other.
	var js *JudasSwing

	if initialMoveUp && reversalDown {
		// Initial move was UP (trap), then reversed DOWN — bearish Judas.
		js = &JudasSwing{
			Session:    sessionName,
			Direction:  "BEARISH",
			TrapHigh:   initialHigh,
			TrapLow:    initialLow,
			OpenPrice:  openPrice,
			ReversalOK: true,
		}
	} else if !initialMoveUp && reversalUp {
		// Initial move was DOWN (trap), then reversed UP — bullish Judas.
		js = &JudasSwing{
			Session:    sessionName,
			Direction:  "BULLISH",
			TrapHigh:   initialHigh,
			TrapLow:    initialLow,
			OpenPrice:  openPrice,
			ReversalOK: true,
		}
	} else if initialMoveUp && !reversalDown {
		// Initial move up, no reversal yet — potential bullish continuation
		// (not a Judas, but could become one).
		js = &JudasSwing{
			Session:    sessionName,
			Direction:  "BULLISH", // tentative
			TrapHigh:   initialHigh,
			TrapLow:    initialLow,
			OpenPrice:  openPrice,
			ReversalOK: false,
		}
	} else if !initialMoveUp && !reversalUp {
		js = &JudasSwing{
			Session:    sessionName,
			Direction:  "BEARISH", // tentative
			TrapHigh:   initialHigh,
			TrapLow:    initialLow,
			OpenPrice:  openPrice,
			ReversalOK: false,
		}
	}

	return js
}
