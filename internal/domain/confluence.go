package domain

import "time"

// ---------------------------------------------------------------------------
// ConfluenceBias — overall directional bias for a currency pair
// ---------------------------------------------------------------------------

// ConfluenceBias represents the directional bias derived from the confluence score.
type ConfluenceBias string

const (
	BiasBullish ConfluenceBias = "BULLISH"
	BiasBearish ConfluenceBias = "BEARISH"
	BiasNeutral ConfluenceBias = "NEUTRAL"
)

// ---------------------------------------------------------------------------
// FactorName — named confluence factor identifier
// ---------------------------------------------------------------------------

// FactorName identifies a specific confluence factor.
type FactorName string

// ---------------------------------------------------------------------------
// ConfluenceFactor — a single weighted input to the confluence score
// ---------------------------------------------------------------------------

// ConfluenceFactor represents one of the 6 independent factors that contribute
// to the overall confluence score for a currency pair.
type ConfluenceFactor struct {
	Name          FactorName `json:"name"`           // e.g., "COT Positioning"
	Weight        float64    `json:"weight"`          // Weighting in the total (e.g., 0.25 = 25%)
	RawScore      float64    `json:"raw_score"`       // Raw score 0-100
	WeightedScore float64    `json:"weighted_score"`  // RawScore * Weight
	Signal        string     `json:"signal"`          // "STRONG_BULLISH" | "BULLISH" | "NEUTRAL" | "BEARISH" | "STRONG_BEARISH"
	Detail        string     `json:"detail,omitempty"` // Optional human-readable detail
}

// ---------------------------------------------------------------------------
// ConfluenceScore — aggregated multi-factor score for a currency pair
// ---------------------------------------------------------------------------

// ConfluenceScore holds the final combined confluence analysis for a pair.
type ConfluenceScore struct {
	// Pair identification
	CurrencyPair  string `json:"currency_pair"`   // e.g., "EURUSD"
	BaseCurrency  string `json:"base_currency"`   // e.g., "EUR"
	QuoteCurrency string `json:"quote_currency"`  // e.g., "USD"

	// Score
	TotalScore float64        `json:"total_score"`  // Weighted sum of all factors (0-100)
	Bias       ConfluenceBias `json:"bias"`         // Overall directional bias
	AgreementPct float64      `json:"agreement_pct"` // % of factors agreeing with bias (0-100)

	// Factor breakdown
	Factors         []ConfluenceFactor `json:"factors"`
	StrongestFactor string             `json:"strongest_factor"`
	WeakestFactor   string             `json:"weakest_factor"`
	FactorsAligned  int                `json:"factors_aligned"` // Count of factors aligned with overall bias

	// Metadata
	Timestamp time.Time `json:"timestamp"`
}
