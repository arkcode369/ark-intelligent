package orderflow

import (
	"fmt"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

const minBarsForAnalysis = 10

// Engine runs order flow analysis on OHLCV bars.
type Engine struct{}

// NewEngine creates a new order flow Engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Analyze computes order flow metrics for the given OHLCV bars.
// bars must be newest-first (index 0 = most recent bar), consistent with the
// project-wide convention for []ta.OHLCV.
// symbol and timeframe are used only for display/metadata.
func (e *Engine) Analyze(symbol, timeframe string, bars []ta.OHLCV) *OrderFlowResult {
	result := &OrderFlowResult{
		Symbol:               symbol,
		Timeframe:            timeframe,
		PriceDeltaDivergence: "NONE",
		DeltaTrend:           "FLAT",
		Bias:                 "NEUTRAL",
		AnalyzedAt:           time.Now(),
	}

	if len(bars) < minBarsForAnalysis {
		result.Summary = fmt.Sprintf("Data tidak cukup: %d bar (min %d dibutuhkan)", len(bars), minBarsForAnalysis)
		return result
	}

	// 1. Build delta bars (newest-first preserved).
	deltaBars := buildDeltaBars(bars)
	result.Bars = deltaBars
	result.CumDelta = deltaBars[0].CumDelta // total cumulative delta (newest bar has running total)

	// 2. Point of Control.
	result.PointOfControl = pointOfControl(bars)

	// 3. Delta trend.
	result.DeltaTrend = deltaTrend(deltaBars)

	// 4. Price–delta divergence.
	result.PriceDeltaDivergence = detectDivergence(deltaBars)

	// 5. Absorption patterns.
	bull, bear := detectAbsorptions(deltaBars)
	result.BullishAbsorption = bull
	result.BearishAbsorption = bear

	// 6. Derive overall bias.
	result.Bias = deriveBias(result)

	// 7. Summary narrative.
	result.Summary = buildSummary(result)

	return result
}

// deriveBias combines all signals to produce a single directional bias.
func deriveBias(r *OrderFlowResult) string {
	bullScore := 0
	bearScore := 0

	switch r.PriceDeltaDivergence {
	case "BULLISH_DIV":
		bullScore += 2
	case "BEARISH_DIV":
		bearScore += 2
	}

	switch r.DeltaTrend {
	case "RISING":
		bullScore++
	case "FALLING":
		bearScore++
	}

	if len(r.BullishAbsorption) > 0 {
		bullScore++
	}
	if len(r.BearishAbsorption) > 0 {
		bearScore++
	}

	// Net cumulative delta direction.
	if r.CumDelta > 0 {
		bullScore++
	} else if r.CumDelta < 0 {
		bearScore++
	}

	switch {
	case bullScore > bearScore+1:
		return "BULLISH"
	case bearScore > bullScore+1:
		return "BEARISH"
	default:
		return "NEUTRAL"
	}
}

// buildSummary generates a human-readable summary of the order flow analysis.
func buildSummary(r *OrderFlowResult) string {
	switch r.Bias {
	case "BULLISH":
		if r.PriceDeltaDivergence == "BULLISH_DIV" {
			return "Delta divergence bullish terkonfirmasi — buyers menyerap tekanan jual. Potensi reversal ke atas."
		}
		if len(r.BullishAbsorption) > 0 {
			return "Bullish absorption terdeteksi — selling tidak mampu menekan harga. Buyer control meningkat."
		}
		return "Delta trend rising dengan cumulative delta positif — buyer mendominasi order flow."
	case "BEARISH":
		if r.PriceDeltaDivergence == "BEARISH_DIV" {
			return "Delta divergence bearish terkonfirmasi — selling tersembunyi di balik harga baru tinggi. Waspada reversal turun."
		}
		if len(r.BearishAbsorption) > 0 {
			return "Bearish absorption terdeteksi — buying tidak mampu mengangkat harga. Seller control meningkat."
		}
		return "Delta trend falling dengan cumulative delta negatif — seller mendominasi order flow."
	default:
		return "Order flow netral — tidak ada sinyal divergence atau absorption yang signifikan."
	}
}
