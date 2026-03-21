package domain

import "time"

// ---------------------------------------------------------------------------
// Persisted Signal — Signal snapshot for backtesting
// ---------------------------------------------------------------------------

// PersistedSignal stores a signal snapshot at generation time,
// including the price at the moment of detection and eventual outcomes.
type PersistedSignal struct {
	// Signal identity
	ContractCode string  `json:"contract_code"`
	Currency     string  `json:"currency"`
	SignalType   string  `json:"signal_type"` // e.g. "SMART_MONEY"
	Direction    string  `json:"direction"`    // "BULLISH" or "BEARISH"
	Strength     int     `json:"strength"`     // 1-5
	Confidence   float64 `json:"confidence"`   // 0-100
	Description  string  `json:"description"`

	// Timing
	ReportDate time.Time `json:"report_date"` // COT report date
	DetectedAt time.Time `json:"detected_at"` // When signal was generated

	// Price at detection
	EntryPrice float64 `json:"entry_price"` // Close price on detection week
	Inverse    bool    `json:"inverse"`      // Whether pair is inverse (USD/JPY etc.)

	// Context at detection (for later analysis)
	SentimentScore  float64 `json:"sentiment_score,omitempty"`
	COTIndex        float64 `json:"cot_index,omitempty"`
	ConvictionScore float64 `json:"conviction_score,omitempty"`
	FREDRegime      string  `json:"fred_regime,omitempty"`

	// Outcome (populated later by evaluator)
	Price1W float64 `json:"price_1w,omitempty"` // Close price +1 week
	Price2W float64 `json:"price_2w,omitempty"` // Close price +2 weeks
	Price4W float64 `json:"price_4w,omitempty"` // Close price +4 weeks

	Return1W float64 `json:"return_1w,omitempty"` // % change from entry
	Return2W float64 `json:"return_2w,omitempty"`
	Return4W float64 `json:"return_4w,omitempty"`

	Outcome1W string `json:"outcome_1w,omitempty"` // "WIN", "LOSS", "PENDING"
	Outcome2W string `json:"outcome_2w,omitempty"`
	Outcome4W string `json:"outcome_4w,omitempty"`

	EvaluatedAt time.Time `json:"evaluated_at,omitempty"`
}

// Signal outcome constants.
const (
	OutcomeWin     = "WIN"
	OutcomeLoss    = "LOSS"
	OutcomePending = "PENDING"
)

// IsFullyEvaluated returns true if all three time horizons have outcomes.
func (s *PersistedSignal) IsFullyEvaluated() bool {
	return s.Outcome1W != "" && s.Outcome1W != OutcomePending &&
		s.Outcome2W != "" && s.Outcome2W != OutcomePending &&
		s.Outcome4W != "" && s.Outcome4W != OutcomePending
}

// NeedsEvaluation returns true if any outcome is still pending or empty.
func (s *PersistedSignal) NeedsEvaluation(now time.Time) bool {
	if s.EntryPrice == 0 {
		return false // No price data at detection — cannot evaluate
	}
	// Need at least 1 week to have passed for 1W evaluation
	return now.Sub(s.ReportDate) >= 7*24*time.Hour &&
		(s.Outcome1W == "" || s.Outcome1W == OutcomePending)
}

// ---------------------------------------------------------------------------
// Backtest Statistics — Aggregate metrics
// ---------------------------------------------------------------------------

// BacktestStats holds aggregate statistics for a group of signals.
type BacktestStats struct {
	GroupLabel   string `json:"group_label"`   // e.g. "EUR", "SMART_MONEY", "ALL"
	TotalSignals int    `json:"total_signals"` // Total persisted signals
	Evaluated    int    `json:"evaluated"`     // Signals with outcomes

	// Win rates by holding period (0-100%)
	WinRate1W float64 `json:"win_rate_1w"`
	WinRate2W float64 `json:"win_rate_2w"`
	WinRate4W float64 `json:"win_rate_4w"`

	// Average returns by holding period (%)
	AvgReturn1W float64 `json:"avg_return_1w"`
	AvgReturn2W float64 `json:"avg_return_2w"`
	AvgReturn4W float64 `json:"avg_return_4w"`

	// Risk metrics
	AvgWinReturn1W  float64 `json:"avg_win_return_1w,omitempty"`  // Avg return on winning trades
	AvgLossReturn1W float64 `json:"avg_loss_return_1w,omitempty"` // Avg return on losing trades (negative)

	// Optimal holding period
	BestPeriod  string  `json:"best_period"`  // "1W", "2W", "4W"
	BestWinRate float64 `json:"best_win_rate"`

	// Confidence calibration
	AvgConfidence    float64 `json:"avg_confidence"`    // Average stated confidence (0-100)
	ActualAccuracy   float64 `json:"actual_accuracy"`    // Actual win rate at best period
	CalibrationError float64 `json:"calibration_error"` // |confidence - accuracy|

	// Strength breakdown
	HighStrengthWinRate float64 `json:"high_strength_win_rate"` // Strength 4-5
	LowStrengthWinRate  float64 `json:"low_strength_win_rate"`  // Strength 1-3
	HighStrengthCount   int     `json:"high_strength_count"`
	LowStrengthCount    int     `json:"low_strength_count"`
}
