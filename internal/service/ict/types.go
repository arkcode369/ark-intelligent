// Package ict implements Inner Circle Trader (ICT)
// analysis engine for forex pairs. It detects Fair Value Gaps, Order Blocks,
// Breaker Blocks, Change of Character (CHoCH), Break of Structure (BOS),
// and Liquidity Sweeps from OHLCV data.
//
// FVG and Order Block detection is delegated to the canonical ta.CalcICT()
// implementation in internal/service/ta/ict.go. This package adds structure
// detection (BOS/CHoCH) and liquidity sweep analysis on top.
//
// Type field names are aligned with the ta package for consistency:
//   - Type (not Kind) for directional classification
//   - High/Low (not Top/Bottom) for price boundaries
package ict

import "time"

// ---------------------------------------------------------------------------
// Core Structs — field names aligned with ta.FVG / ta.OrderBlock
// ---------------------------------------------------------------------------

// FVGZone represents a Fair Value Gap — a 3-candle imbalance zone.
// Field names (High, Low, Type) match ta.FVG for consistency.
type FVGZone struct {
	Type      string    // "BULLISH" | "BEARISH"  (aligned with ta.FVG.Type)
	High      float64   // upper bound of the gap (aligned with ta.FVG.High)
	Low       float64   // lower bound of the gap (aligned with ta.FVG.Low)
	CreatedAt time.Time // timestamp of the middle candle
	BarIndex  int       // index of the middle candle (in the input slice)
	Filled    bool      // true if price has entered this zone
	FillPct   float64   // how far the gap has been filled (0–100%)
}

// OrderBlock represents an institutional demand/supply zone.
// A Bearish OB is the last bullish candle before a bearish impulse move.
// A Bullish OB is the last bearish candle before a bullish impulse move.
// Field names (High, Low, Type) match ta.OrderBlock for consistency.
type OrderBlock struct {
	Type     string  // "BULLISH" | "BEARISH"  (aligned with ta.OrderBlock.Type)
	High     float64 // high of the order block candle (aligned with ta.OrderBlock.High)
	Low      float64 // low of the order block candle  (aligned with ta.OrderBlock.Low)
	Volume   float64 // volume of the order block candle
	BarIndex int     // index in the input slice
	Broken   bool    // true = price has broken through → becomes a Breaker Block
}

// StructureEvent represents a BOS (Break of Structure) or CHoCH (Change of Character).
type StructureEvent struct {
	Type      string  // "CHOCH" | "BOS"
	Direction string  // "BULLISH" | "BEARISH" (direction of the break)
	Level     float64 // swing high/low that was broken
	BarIndex  int     // index of the candle that broke the level
}

// LiquiditySweep represents a candle that wicks through a prior swing high/low
// (grabbing stop-loss liquidity) before reversing.
type LiquiditySweep struct {
	Type      string  // "SWEEP_HIGH" | "SWEEP_LOW"
	Level     float64 // previous swing high/low that was swept
	SweepHigh float64 // high of the sweeping candle
	SweepLow  float64 // low of the sweeping candle
	BarIndex  int     // index of the sweeping candle
	Reversed  bool    // true if confirmed reversal after sweep (close opposite side)
}

// MarketMakerModel represents an ICT Market Maker pattern.
// ICT identifies three main models:
//
//	MMD — Market Maker Day:   daily accumulation vs distribution
//	MMW — Market Maker Week:  weekly pattern (accumulation → manipulation → distribution)
//	MMS — Market Maker Session: session engineered for selling (premium) or buying (discount)
type MarketMakerModel struct {
	Model     string  // "MMD" | "MMW" | "MMS"
	Phase     string  // "ACCUMULATION" | "MANIPULATION" | "DISTRIBUTION"
	Direction string  // "BULLISH" | "BEARISH" — institutional intent
	RangeHigh float64 // relevant range high
	RangeLow  float64 // relevant range low
}

// JudasSwing represents a false initial move at the start of a session
// that reverses in the opposite direction — the "Judas Swing".
// ICT teaches that the first move of a session is often a trap:
//   - London open: price drops first, then rallies (bullish Judas)
//   - London open: price rallies first, then drops (bearish Judas)
//   - NY open: similar pattern
//
// Detection: within the first 1-2 hours of a session, if price moves
// in one direction then reverses beyond the session open price, that
// initial move was a Judas Swing.
type JudasSwing struct {
	Session    string  // "LONDON" | "NEW_YORK"
	Direction  string  // "BULLISH" | "BEARISH" — direction of the TRUE move (after reversal)
	TrapHigh   float64 // high of the initial false move
	TrapLow    float64 // low of the initial false move
	OpenPrice  float64 // session open price (first bar of session)
	ReversalOK bool    // true if the reversal has been confirmed
}

// AMDPhase represents a phase in ICT's Power of 3 model:
// Accumulation → Manipulation → Distribution.
// Supports multi-timeframe: Weekly PO3, Daily PO3, Session PO3.
type AMDPhase struct {
	Session   string  // "ASIAN" | "LONDON" | "NEW_YORK" | "WEEKLY" | "DAILY"
	TF        string  // "WEEKLY" | "DAILY" | "SESSION" — timeframe level of this PO3
	Phase     string  // "ACCUMULATION" | "MANIPULATION" | "DISTRIBUTION" | "UNKNOWN"
	RangeHigh float64 // high of the current session/period
	RangeLow  float64 // low of the current session/period
	AccHigh   float64 // accumulation range high (first third)
	AccLow    float64 // accumulation range low (first third)
	Direction string  // "BULLISH" | "BEARISH" — direction of the distribution move
	Date      time.Time // date of the session/period
}

// SilverBullet represents a time window where FVGs are most reliable.
// ICT identifies three "Silver Bullet" windows per day:
//   - 10:00–11:00 UTC (London AM)
//   - 14:00–15:00 UTC (NY AM)
//   - 16:00–17:00 UTC (NY PM)
// If an unfilled FVG aligns with the current Silver Bullet window, it's
// considered a high-probability trade setup.
// High/Low capture the price range during the window for chart box drawing.
type SilverBullet struct {
	Window   string    // "LONDON_AM" | "NY_AM" | "NY_PM"
	StartUTC int       // start hour (UTC)
	EndUTC   int       // end hour (UTC)
	Active   bool      // true if current time is within this window
	FVGIndex int       // index of the matching FVG in ICTResult.FVGZones (-1 if none)
	Date     time.Time // date of the window
	High     float64   // highest price during the window (for box drawing)
	Low      float64   // lowest price during the window (for box drawing)
}

// OTE represents an Optimal Trade Entry zone — the 62-79% Fibonacci
// retracement of the most recent impulse leg. ICT traders use this zone
// for high-probability entries in the direction of the trend.
type OTE struct {
	Direction string  // "BULLISH" | "BEARISH" — direction of the impulse leg
	High      float64 // upper bound of the OTE zone (62% retracement)
	Low       float64 // lower bound of the OTE zone (79% retracement)
	Midpoint  float64 // 70.5% retracement (sweet spot)
	ImpulseStart float64 // start price of the impulse leg
	ImpulseEnd   float64 // end price of the impulse leg
}

// LiquidityLevel represents a cluster of equal highs or equal lows
// that act as liquidity pools (stop-loss magnets).
type LiquidityLevel struct {
	Price float64 // average price of the cluster
	Type  string  // "BUY_SIDE" (equal highs) | "SELL_SIDE" (equal lows)
	Count int     // number of pivots clustered at this level
	Swept bool    // true if price briefly pierced then closed back
}

// swingPoint is an internal struct for swing highs/lows (not exported to callers).
type swingPoint struct {
	isHigh   bool
	level    float64
	barIndex int
	date     time.Time // timestamp of the bar
}

// RelevantAnchor represents the most significant swing high/low used as
// a reference point for P&D zones, OTE, and other ICT concepts.
type RelevantAnchor struct {
	Type        string    // "HIGH" | "LOW"
	Level       float64   // price level of the anchor
	BarIndex    int       // index in the bar slice
	Date        time.Time
	ATRMultiple float64 // how many ATRs this swing stands out (significance score)
}

// KillzoneBox represents a killzone time window with its price range
// (high/low pivots) and mitigation status.
// Like the PineScript reference: each killzone is drawn as a box
// from session start to end, with pivot high/low lines extending
// until mitigated.
type KillzoneBox struct {
	Name      string    // "ASIAN", "LONDON", "NY_AM", "NY_LUNCH", "NY_PM"
	StartUTC  int       // start hour (UTC)
	EndUTC    int       // end hour (UTC)
	High      float64   // highest price during the killzone window
	Low       float64   // lowest price during the killzone window
	Mitigated bool      // true if price has broken above High or below Low
	Date      time.Time // date of the session
}

// DWMPivot represents a previous Day/Week/Month High or Low.
// These are key institutional reference levels - ICT traders watch
// PDH/PDL, PWH/PWL, PMH/PML as liquidity targets and structure anchors.
type DWMPivot struct {
	Type        string    // "PDH", "PDL", "PWH", "PWL", "PMH", "PML"
	Level       float64   // price level
	PeriodStart time.Time // start of the period
	PeriodEnd   time.Time // end of the period
	Broken      bool      // true if current price has broken this level
}

// ICTResult is the main output of the ICT engine.
type ICTResult struct {
	Symbol      string
	Timeframe   string
	FVGZones    []FVGZone
	OrderBlocks []OrderBlock
	Structure   []StructureEvent
	Sweeps      []LiquiditySweep
	Bias        string // "BULLISH" | "BEARISH" | "NEUTRAL"
	Killzone    string // current killzone if applicable (e.g. "London", "New York")
	Summary     string // human-readable narrative
	AnalyzedAt  time.Time

	// Premium/Discount zones (from ta.CalcICT)
	Equilibrium  float64 // 50% midpoint of recent significant range
	PremiumZone  bool    // price above equilibrium (sell zone in bullish bias)
	DiscountZone bool    // price below equilibrium (buy zone in bullish bias)
	CurrentPrice float64 // most recent close price

	// Liquidity levels (from ta.CalcICT)
	LiquidityLevels []LiquidityLevel

	// Optimal Trade Entry zones (from swing-based impulse legs)
	OTE []OTE

	// Silver Bullet time windows with FVG alignment
	SilverBullets []SilverBullet

	// Power of 3 (AMD) phase detection per session
	AMD []AMDPhase

	// Judas Swing detection per session
	JudasSwings []JudasSwing

	// Market Maker Models (MMD/MMW/MMS)
	MarketMakerModels []MarketMakerModel

	// Relevant anchors — most significant swing high/low for P&D and OTE
	RelevantHigh  *RelevantAnchor // most significant swing high
	RelevantLow   *RelevantAnchor // most significant swing low

	// Killzone boxes with price ranges + pivot lines
	KillzoneBoxes []KillzoneBox

	// DWM Pivots — previous Day/Week/Month High/Low
	DWMPivots []DWMPivot

	// ATR value used for dynamic calculations
	ATR float64
}
