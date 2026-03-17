package domain

import "time"

// ---------------------------------------------------------------------------
// CurrencyCode — typed currency code string
// ---------------------------------------------------------------------------

// CurrencyCode is a 3-letter ISO currency identifier.
type CurrencyCode string

// String returns the currency code as a plain string.
func (c CurrencyCode) String() string { return string(c) }

// ---------------------------------------------------------------------------
// CurrencyScore — composite strength score for a single currency
// ---------------------------------------------------------------------------

// CurrencyScore holds the multi-dimensional strength score for one currency.
type CurrencyScore struct {
	Code             CurrencyCode `json:"code"`
	InterestRateScore float64     `json:"interest_rate_score"` // 0-100
	InflationScore   float64     `json:"inflation_score"`     // 0-100
	GDPScore         float64     `json:"gdp_score"`           // 0-100
	EmploymentScore  float64     `json:"employment_score"`    // 0-100
	COTScore         float64     `json:"cot_score"`           // 0-100
	SurpriseScore    float64     `json:"surprise_score"`      // 0-100
	CompositeScore   float64     `json:"composite_score"`     // Weighted aggregate 0-100
}

// ---------------------------------------------------------------------------
// RankedCurrency — a currency with its ordinal rank
// ---------------------------------------------------------------------------

// RankedCurrency wraps a CurrencyScore with a rank ordinal.
type RankedCurrency struct {
	Rank  int           `json:"rank"`
	Score CurrencyScore `json:"score"`
}

// ---------------------------------------------------------------------------
// CurrencyRanking — full sorted ranking of all major currencies
// ---------------------------------------------------------------------------

// CurrencyRanking is the complete sorted list of ranked currencies.
type CurrencyRanking struct {
	Rankings  []RankedCurrency `json:"rankings"`
	Timestamp time.Time        `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// PairAnalysis — strength differential between two currencies
// ---------------------------------------------------------------------------

// PairAnalysis represents the directional analysis for a currency pair.
type PairAnalysis struct {
	Base              CurrencyCode  `json:"base"`
	Quote             CurrencyCode  `json:"quote"`
	BaseScore         CurrencyScore `json:"base_score"`
	QuoteScore        CurrencyScore `json:"quote_score"`
	ScoreDifferential float64       `json:"score_differential"`
	Direction         string        `json:"direction"`  // "STRONG_BUY" | "BUY" | "NEUTRAL" | "SELL" | "STRONG_SELL"
	Strength          float64       `json:"strength"`   // Magnitude 0-100
}


