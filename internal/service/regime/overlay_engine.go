package regime

// overlay_engine.go — Market Regime Overlay Engine (TASK-110)
//
// Combines HMM (30%), GARCH (25%), ADX (25%), COT (20%) into a unified
// market health score (-100 to +100). Graceful degradation: if a sub-model
// fails, remaining weights are re-normalised so the score remains valid.

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ---------------------------------------------------------------------------
// Weights
// ---------------------------------------------------------------------------

const (
	weightHMM   = 0.30
	weightGARCH = 0.25
	weightADX   = 0.25
	weightCOT   = 0.20
)

// ---------------------------------------------------------------------------
// Interfaces
// ---------------------------------------------------------------------------

// WeeklyPriceProvider fetches weekly OHLC records for HMM / GARCH.
type WeeklyPriceProvider interface {
	FetchWeekly(ctx context.Context, mapping domain.PriceSymbolMapping, weeks int) ([]domain.PriceRecord, error)
}

// DailyPriceProvider fetches daily bars for ADX computation.
type DailyPriceProvider interface {
	GetDailyHistory(ctx context.Context, contractCode string, days int) ([]domain.DailyPrice, error)
}

// COTProvider fetches the latest COT analysis for a single contract code.
type COTProvider interface {
	AnalyzeContract(ctx context.Context, contractCode string) (*domain.COTAnalysis, error)
}

// ---------------------------------------------------------------------------
// Engine
// ---------------------------------------------------------------------------

// Engine orchestrates all sub-models and returns a RegimeOverlay.
type Engine struct {
	weekly     WeeklyPriceProvider
	daily      DailyPriceProvider
	cotProvider COTProvider

	mu    sync.Mutex
	cache map[string]*cachedOverlay
}

type cachedOverlay struct {
	overlay   *RegimeOverlay
	cachedAt  time.Time
	ttl       time.Duration
}

// NewEngine creates a new regime overlay engine.
// cotProvider is optional — pass nil to skip COT scoring.
func NewEngine(weekly WeeklyPriceProvider, daily DailyPriceProvider, cot COTProvider) *Engine {
	return &Engine{
		weekly:      weekly,
		daily:       daily,
		cotProvider: cot,
		cache:       make(map[string]*cachedOverlay),
	}
}

// cacheTTL returns the appropriate cache TTL based on timeframe.
func cacheTTL(timeframe string) time.Duration {
	switch strings.ToLower(timeframe) {
	case "1h", "4h", "intraday":
		return 1 * time.Hour
	default:
		return 4 * time.Hour
	}
}

// cacheKey builds a cache key from symbol + timeframe.
func cacheKey(symbol, timeframe string) string {
	return strings.ToUpper(symbol) + ":" + strings.ToLower(timeframe)
}

// ComputeOverlay orchestrates all sub-models and returns a unified RegimeOverlay.
// symbol is a display currency symbol (e.g. "EUR", "XAU", "BTC").
// mapping is the full PriceSymbolMapping for the contract.
func (e *Engine) ComputeOverlay(ctx context.Context, mapping domain.PriceSymbolMapping, timeframe string) (*RegimeOverlay, error) {
	key := cacheKey(mapping.Currency, timeframe)

	// Check cache
	e.mu.Lock()
	if c, ok := e.cache[key]; ok && time.Since(c.cachedAt) < c.ttl {
		e.mu.Unlock()
		return c.overlay, nil
	}
	e.mu.Unlock()

	overlay := &RegimeOverlay{}

	// ---- 1. HMM (30%) ----
	hmmScore, hmmOK := e.computeHMM(ctx, overlay, mapping)

	// ---- 2. GARCH (25%) ----
	garchScore, garchOK := e.computeGARCH(ctx, overlay, mapping)

	// ---- 3. ADX (25%) ----
	adxScore, adxOK := e.computeADX(ctx, overlay, mapping)

	// ---- 4. COT (20%) ----
	cotScore, cotOK := e.computeCOT(ctx, overlay, mapping)

	// ---- 5. Weighted composite with graceful degradation ----
	totalWeight := 0.0
	weightedSum := 0.0

	if hmmOK {
		weightedSum += hmmScore * weightHMM
		totalWeight += weightHMM
	}
	if garchOK {
		weightedSum += garchScore * weightGARCH
		totalWeight += weightGARCH
	}
	if adxOK {
		weightedSum += adxScore * weightADX
		totalWeight += weightADX
	}
	if cotOK {
		weightedSum += cotScore * weightCOT
		totalWeight += weightCOT
	}

	if totalWeight == 0 {
		return nil, fmt.Errorf("all regime sub-models failed for %s", mapping.Currency)
	}

	// Re-normalise to keep score in -100..+100 range
	unified := weightedSum / totalWeight * 100
	unified = math.Max(-100, math.Min(100, unified))

	overlay.UnifiedScore = math.Round(unified*10) / 10
	overlay.WeightsUsed = totalWeight

	// ---- 6. Classify ----
	overlay.OverlayColor, overlay.Label = classifyScore(unified)
	overlay.Description = buildDescription(overlay)

	// ---- 7. Store cache ----
	ttl := cacheTTL(timeframe)
	e.mu.Lock()
	e.cache[key] = &cachedOverlay{overlay: overlay, cachedAt: time.Now(), ttl: ttl}
	e.mu.Unlock()

	return overlay, nil
}

// ---------------------------------------------------------------------------
// Sub-model helpers
// ---------------------------------------------------------------------------

// computeHMM fetches weekly prices and runs the HMM regime detector.
// Returns a score in -1..+1 and whether it succeeded.
func (e *Engine) computeHMM(ctx context.Context, out *RegimeOverlay, mapping domain.PriceSymbolMapping) (float64, bool) {
	records, err := e.weekly.FetchWeekly(ctx, mapping, 120)
	if err != nil || len(records) < 60 {
		return 0, false
	}

	result, err := pricesvc.EstimateHMMRegime(records)
	if err != nil || result == nil {
		return 0, false
	}

	out.HMMState = result.CurrentState
	// Confidence = probability of the current state
	switch result.CurrentState {
	case pricesvc.HMMRiskOn:
		out.HMMConfidence = result.StateProbabilities[0]
	case pricesvc.HMMRiskOff:
		out.HMMConfidence = result.StateProbabilities[1]
	case pricesvc.HMMCrisis:
		out.HMMConfidence = result.StateProbabilities[2]
	}
	out.HMMAvailable = true

	score := hmmStateScore(result.CurrentState, out.HMMConfidence)
	out.HMMScore = score * 100
	return score, true
}

// hmmStateScore maps HMM state + confidence to -1..+1.
func hmmStateScore(state string, confidence float64) float64 {
	base := 0.0
	switch state {
	case pricesvc.HMMRiskOn:
		base = 1.0
	case pricesvc.HMMRiskOff:
		base = 0.0
	case pricesvc.HMMCrisis:
		base = -1.0
	}
	// Scale by confidence; un-confident result contributes less signal
	clampedConf := math.Max(0.33, math.Min(1.0, confidence))
	return base * clampedConf
}

// computeGARCH fetches weekly prices and runs GARCH(1,1).
// Low vol (contracting) is positive; high vol (expanding) is negative.
func (e *Engine) computeGARCH(ctx context.Context, out *RegimeOverlay, mapping domain.PriceSymbolMapping) (float64, bool) {
	records, err := e.weekly.FetchWeekly(ctx, mapping, 80)
	if err != nil || len(records) < 30 {
		return 0, false
	}

	result, err := pricesvc.EstimateGARCH(records)
	if err != nil || result == nil || !result.Converged {
		return 0, false
	}

	out.VolRatio = result.VolRatio
	out.GARCHVolRegime = volRegimeLabel(result.VolRatio)
	out.GARCHAvailable = true

	score := garchVolScore(result.VolRatio)
	out.GARCHScore = score * 100
	return score, true
}

// garchVolScore maps VolRatio to -1..+1.
// VolRatio > 1 means above-average vol → negative (risk-off signal).
// VolRatio < 1 means below-average vol → positive.
func garchVolScore(volRatio float64) float64 {
	// Map [0.3 .. 2.5] → [+1 .. -1]
	clamped := math.Max(0.3, math.Min(2.5, volRatio))
	// Linear: at 1.0 → 0, at 0.3 → +1, at 2.5 → -1
	return -(clamped - 1.0) / 1.5
}

// computeADX fetches daily bars and computes ADX(14).
// Strong trending = positive signal; weak/ranging = neutral.
func (e *Engine) computeADX(ctx context.Context, out *RegimeOverlay, mapping domain.PriceSymbolMapping) (float64, bool) {
	if e.daily == nil {
		return 0, false
	}
	dailyBars, err := e.daily.GetDailyHistory(ctx, mapping.ContractCode, 80)
	if err != nil || len(dailyBars) < 28 {
		return 0, false
	}

	// Convert domain.DailyPrice to ta.OHLCV
	ohlcv := make([]ta.OHLCV, len(dailyBars))
	for i, b := range dailyBars {
		ohlcv[i] = ta.OHLCV{
			Date:  b.Date,
			Open:  b.Open,
			High:  b.High,
			Low:   b.Low,
			Close: b.Close,
		}
	}

	result := ta.CalcADX(ohlcv, 14)
	if result == nil {
		return 0, false
	}

	out.ADXValue = result.ADX
	out.ADXStrength = result.TrendStrength
	out.ADXAvailable = true

	score := adxScore(result.ADX, result.PlusDI, result.MinusDI)
	out.ADXScore = score * 100
	return score, true
}

// adxScore maps ADX + direction to -1..+1.
// Strong trend with +DI > -DI = bullish; strong with -DI > +DI = bearish; weak = neutral.
func adxScore(adx, plusDI, minusDI float64) float64 {
	// ADX strength: 0..25 weak, 25..50 moderate, 50+ strong
	strengthFactor := math.Min(adx/50.0, 1.0) // 0..1

	direction := 0.0
	if plusDI > minusDI {
		direction = 1.0
	} else if minusDI > plusDI {
		direction = -1.0
	}

	return direction * strengthFactor
}

// computeCOT fetches COT analysis and extracts SentimentScore.
func (e *Engine) computeCOT(ctx context.Context, out *RegimeOverlay, mapping domain.PriceSymbolMapping) (float64, bool) {
	if e.cotProvider == nil {
		return 0, false
	}

	analysis, err := e.cotProvider.AnalyzeContract(ctx, mapping.ContractCode)
	if err != nil || analysis == nil {
		return 0, false
	}

	out.COTSentiment = analysis.SentimentScore
	out.COTAvailable = true

	// SentimentScore is already -1..+1 per domain definition
	score := math.Max(-1.0, math.Min(1.0, analysis.SentimentScore))
	out.COTScore = score * 100
	return score, true
}

// ---------------------------------------------------------------------------
// Classification helpers
// ---------------------------------------------------------------------------

// classifyScore maps unified score to color emoji and label.
func classifyScore(score float64) (color, label string) {
	switch {
	case score >= 50:
		return "🟢", "BULLISH"
	case score >= 15:
		return "🟡", "MILDLY BULLISH"
	case score > -15:
		return "🟡", "NEUTRAL"
	case score > -50:
		return "🔴", "MILDLY BEARISH"
	default:
		return "🔴", "BEARISH/CRISIS"
	}
}

// buildDescription builds a compact one-line overlay header.
func buildDescription(o *RegimeOverlay) string {
	score := fmt.Sprintf("%+.0f", o.UnifiedScore)
	header := fmt.Sprintf("📊 Regime: %s %s (%s)", o.OverlayColor, o.Label, score)

	parts := []string{}
	if o.ADXAvailable {
		parts = append(parts, o.ADXStrength+" Trend")
	}
	if o.GARCHAvailable {
		switch o.GARCHVolRegime {
		case "EXPANDING":
			parts = append(parts, "High Vol")
		case "CONTRACTING":
			parts = append(parts, "Low Vol")
		default:
			parts = append(parts, "Norm Vol")
		}
	}
	if o.COTAvailable {
		switch {
		case o.COTSentiment > 0.3:
			parts = append(parts, "COT Long")
		case o.COTSentiment < -0.3:
			parts = append(parts, "COT Short")
		default:
			parts = append(parts, "COT Neutral")
		}
	}
	if o.HMMAvailable {
		parts = append(parts, "HMM:"+o.HMMState)
	}

	if len(parts) > 0 {
		header += " | " + strings.Join(parts, ", ")
	}
	return header
}
