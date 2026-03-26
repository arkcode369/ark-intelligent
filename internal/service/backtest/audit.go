package backtest

// ---------------------------------------------------------------------------
// Survivorship & Look-Ahead Bias Audit — institutional due-diligence log
// ---------------------------------------------------------------------------

// SignalTypeAudit documents the theoretical basis and potential biases
// for each signal type, supporting institutional-grade transparency.
type SignalTypeAudit struct {
	SignalType       string   `json:"signal_type"`
	Hypothesis       string   `json:"hypothesis"`
	TheoreticalBasis string   `json:"theoretical_basis"`
	DateAdded        string   `json:"date_added"`
	PotentialBiases  []string `json:"potential_biases"`
}

// AuditLog contains bias audit entries for all signal types.
var AuditLog = []SignalTypeAudit{
	{
		SignalType:       "SMART_MONEY",
		Hypothesis:       "Commercial hedgers have informational edge",
		TheoreticalBasis: "Briese (2008), Larry Williams COT analysis",
		DateAdded:        "2024-01-15",
		PotentialBiases: []string{
			"Survivorship bias: only profitable commercials persist in data",
			"Look-ahead bias: COT data is released with 3-day lag",
			"Regime bias: commercial edge may diminish in low-volatility regimes",
		},
	},
	{
		SignalType:       "EXTREME_POSITIONING",
		Hypothesis:       "Mean reversion at extremes",
		TheoreticalBasis: "DeMark sequential, contrarian theory",
		DateAdded:        "2024-01-15",
		PotentialBiases: []string{
			"Threshold selection bias: extreme levels chosen after seeing outcomes",
			"Lookback bias: percentile thresholds depend on historical window length",
			"Crowding risk: widely-known extremes may lose predictive power",
		},
	},
	{
		SignalType:       "DIVERGENCE",
		Hypothesis:       "Spec/commercial divergence precedes reversals",
		TheoreticalBasis: "COT analysis literature, inter-market divergence theory",
		DateAdded:        "2024-02-01",
		PotentialBiases: []string{
			"Confirmation bias: divergences are common; only some lead to reversals",
			"Duration bias: divergence can persist far longer than expected",
			"Normalization bias: divergence magnitude depends on scaling method",
		},
	},
	{
		SignalType:       "MOMENTUM_SHIFT",
		Hypothesis:       "Positioning momentum carries forward",
		TheoreticalBasis: "Trend-following theory, Jegadeesh & Titman (1993)",
		DateAdded:        "2024-02-01",
		PotentialBiases: []string{
			"Whipsaw bias: momentum signals in range-bound markets generate false signals",
			"Lag bias: COT data delay means momentum may already be priced in",
			"Overfitting risk: lookback period for momentum detection tuned to historical data",
		},
	},
	{
		SignalType:       "CONCENTRATION",
		Hypothesis:       "Top trader concentration creates unwind risk",
		TheoreticalBasis: "Market microstructure theory, position concentration literature",
		DateAdded:        "2024-03-01",
		PotentialBiases: []string{
			"Data granularity bias: CFTC top-trader data aggregates heterogeneous actors",
			"Survivorship bias: only active contracts with sufficient open interest reported",
			"Threshold bias: concentration thresholds may be overfit to historical extremes",
		},
	},
	{
		SignalType:       "CROWD_CONTRARIAN",
		Hypothesis:       "Crowded trades unwind",
		TheoreticalBasis: "Behavioral finance, herding models (Scharfstein & Stein, 1990)",
		DateAdded:        "2024-03-01",
		PotentialBiases: []string{
			"Timing bias: crowded trades can persist and intensify before unwinding",
			"Definition bias: crowd threshold is subjective and regime-dependent",
			"Selection bias: contrarian signals filtered by additional criteria may inflate backtest",
		},
	},
	{
		SignalType:       "THIN_MARKET",
		Hypothesis:       "Low participation equals instability",
		TheoreticalBasis: "Liquidity theory, Kyle (1985) market microstructure",
		DateAdded:        "2024-04-01",
		PotentialBiases: []string{
			"Seasonality bias: thin markets correlate with holidays and quarter-ends",
			"Execution bias: backtest assumes fills at reported prices; thin markets have wider spreads",
			"Directional ambiguity: thin market flags risk but does not indicate direction",
		},
	},
}

// GetAuditSummary returns the full signal type audit log.
func GetAuditSummary() []SignalTypeAudit {
	return AuditLog
}
