package regime

// types.go — RegimeOverlay types for the Market Regime Overlay Engine

// RegimeOverlay is the unified market regime assessment combining HMM, GARCH, ADX, and COT.
// UnifiedScore ranges from -100 (extreme bearish/crisis) to +100 (extreme bullish/trending).
type RegimeOverlay struct {
	// HMM sub-model
	HMMState      string  `json:"hmm_state"`      // RISK_ON, RISK_OFF, CRISIS
	HMMConfidence float64 `json:"hmm_confidence"` // 0..1, probability of current state
	HMMScore      float64 `json:"hmm_score"`      // -100..+100 sub-score
	HMMAvailable  bool    `json:"hmm_available"`

	// GARCH sub-model
	GARCHVolRegime string  `json:"garch_vol_regime"` // EXPANDING, CONTRACTING, NORMAL
	VolRatio       float64 `json:"vol_ratio"`        // CurrentVol / LongRunVol
	GARCHScore     float64 `json:"garch_score"`      // -100..+100 sub-score
	GARCHAvailable bool    `json:"garch_available"`

	// ADX sub-model
	ADXValue     float64 `json:"adx_value"`
	ADXStrength  string  `json:"adx_strength"` // STRONG, MODERATE, WEAK
	ADXScore     float64 `json:"adx_score"`    // -100..+100 sub-score
	ADXAvailable bool    `json:"adx_available"`

	// COT sub-model
	COTSentiment float64 `json:"cot_sentiment"` // raw sentiment score from domain
	COTScore     float64 `json:"cot_score"`     // -100..+100 sub-score
	COTAvailable bool    `json:"cot_available"`

	// Unified output
	UnifiedScore float64 `json:"unified_score"` // -100..+100 (weighted composite)
	OverlayColor string  `json:"overlay_color"` // 🟢 / 🟡 / 🔴
	Label        string  `json:"label"`         // BULLISH, NEUTRAL, BEARISH, CRISIS
	Description  string  `json:"description"`   // Full overlay header string
	WeightsUsed  float64 `json:"weights_used"`  // sum of effective weights (0..1)
}

// volRegimeLabel classifies VolRatio into a human-readable regime string.
func volRegimeLabel(volRatio float64) string {
	switch {
	case volRatio > 1.25:
		return "EXPANDING"
	case volRatio < 0.80:
		return "CONTRACTING"
	default:
		return "NORMAL"
	}
}
