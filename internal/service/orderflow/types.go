// Package orderflow provides estimated delta and order flow analysis from OHLCV data.
// Since tick data is unavailable for forex, a "tick rule" volume approximation is used.
// For crypto, Bybit trades data is preferred; for forex, OHLCV estimation is the fallback.
package orderflow

import (
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// DeltaBar extends a single OHLCV bar with estimated buy/sell volume decomposition.
type DeltaBar struct {
	OHLCV   ta.OHLCV
	BuyVol  float64 // estimated buy volume (tick-rule approximation)
	SellVol float64 // estimated sell volume
	Delta   float64 // BuyVol - SellVol (positive = buyer dominated)
	CumDelta float64 // cumulative delta from the start of the lookback window
}

// OrderFlowResult holds the complete order flow analysis for a symbol/timeframe.
type OrderFlowResult struct {
	Symbol    string
	Timeframe string
	Bars      []DeltaBar // newest-first, same order as input OHLCV slice

	// Price–delta divergence signals
	PriceDeltaDivergence string // "BULLISH_DIV" | "BEARISH_DIV" | "NONE"

	// Point of Control — price level with the highest aggregated volume
	PointOfControl float64

	// Absorption pattern bar indices (into Bars slice)
	BullishAbsorption []int // selling absorbed by buyers
	BearishAbsorption []int // buying absorbed by sellers

	// Delta trend summary
	DeltaTrend string  // "RISING" | "FALLING" | "FLAT"
	CumDelta   float64 // total cumulative delta across all bars

	Bias    string // "BULLISH" | "BEARISH" | "NEUTRAL"
	Summary string
	AnalyzedAt time.Time
}
