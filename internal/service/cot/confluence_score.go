// Package cot provides Confluence Score v2 — institutional-grade positioning score.
// Formula: FRED regime (30%) + COT positioning (35%) + Calendar surprise (20%) + Financial stress (15%)
package cot

import (
	"fmt"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

// ConfluenceScoreV2 computes the institutional-grade confluence score for a currency.
//
// Components:
//   - COT positioning   (35%) — based on SentimentScore (-100..+100)
//   - Calendar surprise (20%) — based on recent sigma surprise for this currency
//   - Financial stress  (15%) — based on FRED NFCI (negative = loose = bullish risk)
//   - FRED regime       (30%) — composite from yield curve, PCE, NFCI, initial claims
//
// Returns a score in [-100, +100]. Positive = bullish bias, negative = bearish bias.
func ConfluenceScoreV2(
	analysis domain.COTAnalysis,
	macroData *fred.MacroData,
	surpriseSigma float64, // recent sigma surprise for this currency (positive = hawkish)
) float64 {
	// 1. COT component (35%)
	cotScore := mathutil.Clamp(analysis.SentimentScore, -100, 100)

	// 2. Calendar surprise component (20%)
	// Scale: 1 sigma → +20 points; 5 sigma → capped at ±100
	surpriseScore := mathutil.Clamp(surpriseSigma*20, -100, 100)

	// 3. Financial stress component (15%)
	// NFCI negative = loose = bullish; positive = tight = bearish
	stressScore := 0.0
	if macroData != nil {
		stressScore = mathutil.Clamp(-macroData.NFCI*50, -100, 100)
	}

	// 4. FRED regime component (30%)
	// Points: yield curve steepening (+30), disinflationary (+30),
	// loose conditions (+20), strong labor (+20) → raw 0..100 normalized to -100..+100
	fredRaw := 0.0
	if macroData != nil {
		if macroData.YieldSpread > 0 {
			fredRaw += 30
		}
		if macroData.CorePCE > 0 && macroData.CorePCE < 2.5 {
			fredRaw += 30
		}
		if macroData.NFCI < 0 {
			fredRaw += 20
		}
		if macroData.InitialClaims > 0 && macroData.InitialClaims < 250_000 {
			fredRaw += 20
		}
		// Normalize: 0..100 raw → -100..+100 (50 = neutral)
		fredScore := mathutil.Clamp(fredRaw-50, -100, 100)

		// GDP growth factor: positive growth → bullish risk bias
		if macroData.GDPGrowth != 0 {
			gdpFactor := 0.0
			switch {
			case macroData.GDPGrowth > 3.0:
				gdpFactor = 10
			case macroData.GDPGrowth > 1.5:
				gdpFactor = 5
			case macroData.GDPGrowth > 0:
				gdpFactor = 0 // low positive growth → neutral (not bearish)
			case macroData.GDPGrowth < 0:
				gdpFactor = -15
			}
			// Blend GDP into the FRED component (reduce FRED weight slightly, add GDP)
			fredScore += gdpFactor * 0.3
		}

		total := cotScore*0.35 + surpriseScore*0.20 + stressScore*0.15 + fredScore*0.30
		return mathutil.Clamp(total, -100, 100)
	}

	// Without FRED data: weight COT (60%) + surprise (40%), stress is unavailable
	total := cotScore*0.60 + surpriseScore*0.40
	return mathutil.Clamp(total, -100, 100)
}

// ---------------------------------------------------------------------------
// Gap B — FRED Macro Regime as per-currency COT bias multiplier
// ---------------------------------------------------------------------------

// FREDRegimeMultiplier returns a per-currency score adjustment (-20 to +20) based on
// the FRED macro regime. This allows the regime to act as a mathematical multiplier
// on COT signals rather than only influencing them at the AI prompt level.
//
// Regime × Currency matrix:
//
//	                   USD   EUR/GBP/CHF   AUD/NZD/CAD   JPY/XAU
//	INFLATIONARY      +15       -10           -10           +5
//	DISINFLATIONARY    -5       +10           +10            0
//	STRESS            -10         0           -20          +20
//	RECESSION         -15         0           -20          +20
//	STAGFLATION         0        -5           -15          +15
//	GOLDILOCKS         -5        +5           +15           -5
//	default             0         0             0            0
func FREDRegimeMultiplier(currency string, regime fred.MacroRegime) float64 {
	type row struct {
		usd, eurGbpChf, audNzdCad, jpyXau float64
	}

	matrix := map[string]row{
		"INFLATIONARY":   {+15, -10, -10, +5},
		"DISINFLATIONARY": {-5, +10, +10, 0},
		"STRESS":         {-10, 0, -20, +20},
		"RECESSION":      {-15, 0, -20, +20},
		"STAGFLATION":    {0, -5, -15, +15},
		"GOLDILOCKS":     {-5, +5, +15, -5},
	}

	r, ok := matrix[regime.Name]
	if !ok {
		return 0
	}

	switch currency {
	case "USD":
		return r.usd
	case "EUR", "GBP", "CHF":
		return r.eurGbpChf
	case "AUD", "NZD", "CAD":
		return r.audNzdCad
	case "JPY", "XAU":
		return r.jpyXau
	default:
		return 0
	}
}

// ComputeRegimeAdjustedScore computes a sentiment score that incorporates the
// FRED macro regime multiplier for the given currency.
//
// Formula: adjusted = Clamp(SentimentScore + FREDRegimeMultiplier(currency, regime), -100, 100)
//
// This is the mathematical link between FRED regime and COT score (Gap B).
func ComputeRegimeAdjustedScore(analysis domain.COTAnalysis, regime fred.MacroRegime) float64 {
	multiplier := FREDRegimeMultiplier(analysis.Contract.Currency, regime)
	return mathutil.Clamp(analysis.SentimentScore+multiplier, -100, 100)
}

// ---------------------------------------------------------------------------
// Gap D — Conviction Score across all 3 sources
// ---------------------------------------------------------------------------

// ConvictionScore represents the unified cross-source conviction for a currency,
// combining COT positioning, FRED macro regime, and calendar surprise data into
// a single actionable score.
type ConvictionScore struct {
	Currency     string  // e.g. "EUR"
	Score        float64 // 0-100
	Direction    string  // "LONG", "SHORT", "NEUTRAL"
	COTBias      string  // e.g. "BULLISH"
	FREDRegime   string  // e.g. "DISINFLATIONARY"
	CalendarBias string  // e.g. "ECB hawkish"
	Label        string  // e.g. "HIGH CONVICTION LONG"
	Version      int     // 2 or 3 (which formula was used)
}

// ComputeConvictionScore generates a unified 0-100 conviction score from all 3 data sources:
// COT positioning, FRED macro regime, and economic calendar surprise.
//
// Algorithm:
//  1. Compute base score via ConfluenceScoreV2 → range -100..+100
//  2. Apply per-currency FREDRegimeMultiplier → adjusted range -100..+100
//  3. Normalize to 0-100: conviction = (adjusted + 100) / 2
//  4. Classify direction: >55 = LONG, <45 = SHORT, else = NEUTRAL
//  5. Classify label: >75 = HIGH CONVICTION, 60-75 = MODERATE, else = LOW
func ComputeConvictionScore(
	analysis domain.COTAnalysis,
	regime fred.MacroRegime,
	surpriseSigma float64,
	calendarNote string,
	macroData *fred.MacroData,
) ConvictionScore {
	// 1. Base score from all 3 sources (-100..+100)
	baseScore := ConfluenceScoreV2(analysis, macroData, surpriseSigma)

	// 2. Apply FRED regime multiplier
	multiplier := FREDRegimeMultiplier(analysis.Contract.Currency, regime)
	adjusted := mathutil.Clamp(baseScore+multiplier, -100, 100)

	// 3. Normalize to 0-100
	conviction := (adjusted + 100) / 2

	return buildConvictionResult(analysis, conviction, regime.Name, calendarNote, 2)
}

// ---------------------------------------------------------------------------
// V3 — 5-component Confluence Score with Price Data
// ---------------------------------------------------------------------------

// ConfluenceScoreV3 computes a 5-component institutional-grade confluence score.
//
// Components:
//   - COT positioning   (25%) — based on SentimentScore (-100..+100)
//   - Calendar surprise  (15%) — based on recent sigma surprise
//   - Financial stress   (10%) — based on FRED NFCI
//   - FRED regime        (20%) — composite macro conditions
//   - Price momentum     (30%) — MA alignment, trend, price-COT concordance
//
// Returns a score in [-100, +100]. Positive = bullish bias, negative = bearish bias.
func ConfluenceScoreV3(
	analysis domain.COTAnalysis,
	macroData *fred.MacroData,
	surpriseSigma float64,
	priceContext *domain.PriceContext,
) float64 {
	// 1. COT component (25%)
	cotScore := mathutil.Clamp(analysis.SentimentScore, -100, 100)

	// 2. Calendar surprise component (15%)
	surpriseScore := mathutil.Clamp(surpriseSigma*20, -100, 100)

	// 3. Financial stress component (10%)
	stressScore := 0.0
	if macroData != nil {
		stressScore = mathutil.Clamp(-macroData.NFCI*50, -100, 100)
	}

	// 4. FRED regime component (20%) — same logic as V2
	fredScore := 0.0
	if macroData != nil {
		fredRaw := 0.0
		if macroData.YieldSpread > 0 {
			fredRaw += 30
		}
		if macroData.CorePCE > 0 && macroData.CorePCE < 2.5 {
			fredRaw += 30
		}
		if macroData.NFCI < 0 {
			fredRaw += 20
		}
		if macroData.InitialClaims > 0 && macroData.InitialClaims < 250_000 {
			fredRaw += 20
		}
		fredScore = mathutil.Clamp(fredRaw-50, -100, 100)

		if macroData.GDPGrowth != 0 {
			gdpFactor := 0.0
			switch {
			case macroData.GDPGrowth > 3.0:
				gdpFactor = 10
			case macroData.GDPGrowth > 1.5:
				gdpFactor = 5
			case macroData.GDPGrowth > 0:
				gdpFactor = 0
			case macroData.GDPGrowth < 0:
				gdpFactor = -15
			}
			fredScore += gdpFactor * 0.3
		}
	}

	// 5. Price momentum component (30%)
	priceScore := 0.0
	if priceContext != nil {
		// MA alignment: above both MAs = bullish, below both = bearish
		maScore := 0.0
		if priceContext.AboveMA4W {
			maScore += 25
		} else {
			maScore -= 25
		}
		if priceContext.AboveMA13W {
			maScore += 25
		} else {
			maScore -= 25
		}

		// Momentum from weekly/monthly changes
		momentumScore := mathutil.Clamp(priceContext.WeeklyChgPct*10, -25, 25) +
			mathutil.Clamp(priceContext.MonthlyChgPct*5, -25, 25)

		priceScore = mathutil.Clamp(maScore+momentumScore, -100, 100)

		// Price-COT concordance bonus: if price and COT agree, boost; if they disagree, dampen
		cotBullish := analysis.SentimentScore > 20
		cotBearish := analysis.SentimentScore < -20
		priceBullish := priceContext.Trend4W == "UP"
		priceBearish := priceContext.Trend4W == "DOWN"

		if (cotBullish && priceBullish) || (cotBearish && priceBearish) {
			// Agreement bonus — boost price component by 20%
			priceScore *= 1.2
		} else if (cotBullish && priceBearish) || (cotBearish && priceBullish) {
			// Disagreement — dampen price component by 30%
			priceScore *= 0.7
		}
		priceScore = mathutil.Clamp(priceScore, -100, 100)
	}

	// Weighted combination
	if priceContext != nil && macroData != nil {
		// Full 5-component
		total := cotScore*0.25 + surpriseScore*0.15 + stressScore*0.10 + fredScore*0.20 + priceScore*0.30
		return mathutil.Clamp(total, -100, 100)
	} else if priceContext != nil {
		// No FRED: COT 35% + Surprise 20% + Price 45%
		total := cotScore*0.35 + surpriseScore*0.20 + priceScore*0.45
		return mathutil.Clamp(total, -100, 100)
	} else if macroData != nil {
		// No price: fall back to V2 weights
		return ConfluenceScoreV2(analysis, macroData, surpriseSigma)
	}

	// Neither: COT 60% + Surprise 40%
	total := cotScore*0.60 + surpriseScore*0.40
	return mathutil.Clamp(total, -100, 100)
}

// ComputeConvictionScoreV3 generates a unified 0-100 conviction score using
// the 5-component V3 formula that includes price momentum data.
func ComputeConvictionScoreV3(
	analysis domain.COTAnalysis,
	regime fred.MacroRegime,
	surpriseSigma float64,
	calendarNote string,
	macroData *fred.MacroData,
	priceContext *domain.PriceContext,
) ConvictionScore {
	baseScore := ConfluenceScoreV3(analysis, macroData, surpriseSigma, priceContext)

	multiplier := FREDRegimeMultiplier(analysis.Contract.Currency, regime)
	adjusted := mathutil.Clamp(baseScore+multiplier, -100, 100)
	conviction := (adjusted + 100) / 2

	return buildConvictionResult(analysis, conviction, regime.Name, calendarNote, 3)
}

// buildConvictionResult creates a ConvictionScore from a normalized 0-100 conviction value.
func buildConvictionResult(analysis domain.COTAnalysis, conviction float64, regimeName, calendarNote string, version int) ConvictionScore {
	direction := "NEUTRAL"
	switch {
	case conviction > 55:
		direction = "LONG"
	case conviction < 45:
		direction = "SHORT"
	}

	cotBias := "NEUTRAL"
	switch {
	case analysis.SentimentScore > 30:
		cotBias = "BULLISH"
	case analysis.SentimentScore < -30:
		cotBias = "BEARISH"
	}

	var label string
	switch {
	case conviction > 75:
		label = fmt.Sprintf("HIGH CONVICTION %s", direction)
	case conviction > 60:
		label = fmt.Sprintf("MODERATE %s", direction)
	default:
		label = fmt.Sprintf("LOW %s", direction)
	}

	return ConvictionScore{
		Currency:     analysis.Contract.Currency,
		Score:        conviction,
		Direction:    direction,
		COTBias:      cotBias,
		FREDRegime:   regimeName,
		CalendarBias: calendarNote,
		Label:        label,
		Version:      version,
	}
}
