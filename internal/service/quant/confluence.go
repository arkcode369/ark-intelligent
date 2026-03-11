package quant

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
	"github.com/arkcode369/ff-calendar-bot/pkg/fmtutil"
	"github.com/arkcode369/ff-calendar-bot/pkg/mathutil"
	"github.com/arkcode369/ff-calendar-bot/pkg/timeutil"
)

// ConfluenceScorer combines 6 independent factors into a single directional
// score (0-100) for each currency pair. Higher = more bullish for the pair.
//
// Factors and weights:
//   1. COT Positioning      (25%) - Net speculator positioning vs history
//   2. Economic Surprise    (20%) - How data is beating/missing expectations
//   3. Rate Trajectory      (20%) - Interest rate expectations direction
//   4. Revision Momentum    (15%) - Are data releases being revised up or down
//   5. Crowd Sentiment      (10%) - Contrarian signal from retail positioning
//   6. Event Risk Premium   (10%) - Upcoming high-impact event density
type ConfluenceScorer struct {
	eventRepo    ports.EventRepository
	cotRepo      ports.COTRepository
	surpriseRepo ports.SurpriseRepository
}

// NewConfluenceScorer creates a confluence scorer with all dependencies.
func NewConfluenceScorer(
	eventRepo ports.EventRepository,
	cotRepo ports.COTRepository,
	surpriseRepo ports.SurpriseRepository,
) *ConfluenceScorer {
	return &ConfluenceScorer{
		eventRepo:    eventRepo,
		cotRepo:      cotRepo,
		surpriseRepo: surpriseRepo,
	}
}

// currencyToContract maps a currency code to a COT contract code
// using the DefaultCOTContracts list.
func currencyToContract(currency string) string {
	for _, c := range domain.DefaultCOTContracts {
		if c.Currency == currency {
			return c.Code
		}
	}
	return ""
}

// ComputeForPair calculates the confluence score for a currency pair.
func (cs *ConfluenceScorer) ComputeForPair(ctx context.Context, base, quote string) (*domain.ConfluenceScore, error) {
	score := &domain.ConfluenceScore{
		CurrencyPair:  base + quote,
		BaseCurrency:  base,
		QuoteCurrency: quote,
		Timestamp:     timeutil.NowWIB(), // FIX: was UpdatedAt
	}

	// Compute each factor
	factors := []domain.ConfluenceFactor{
		cs.computeCOTFactor(ctx, base, quote),
		cs.computeSurpriseFactor(ctx, base, quote),
		cs.computeRateFactor(ctx, base, quote),
		cs.computeRevisionFactor(ctx, base, quote),
		cs.computeCrowdFactor(ctx, base, quote),
		cs.computeEventRiskFactor(ctx, base, quote),
	}

	score.Factors = factors

	// Calculate total weighted score
	totalScore := 0.0
	for _, f := range factors {
		totalScore += f.WeightedScore
	}
	score.TotalScore = mathutil.Clamp(totalScore, 0, 100)

	// FIX: Use Bias (ConfluenceBias type) instead of Direction (string)
	switch {
	case score.TotalScore >= 65:
		score.Bias = domain.BiasBullish
	case score.TotalScore <= 35:
		score.Bias = domain.BiasBearish
	default:
		score.Bias = domain.BiasNeutral
	}

	// FIX: Use AgreementPct instead of Confidence
	score.AgreementPct = computeConfidence(factors)

	// Compute strongest/weakest factors
	if len(factors) > 0 {
		strongest := factors[0]
		weakest := factors[0]
		for _, f := range factors[1:] {
			if f.WeightedScore > strongest.WeightedScore {
				strongest = f
			}
			if f.WeightedScore < weakest.WeightedScore {
				weakest = f
			}
		}
		score.StrongestFactor = string(strongest.Name)
		score.WeakestFactor = string(weakest.Name)
	}

	// Count aligned factors
	aligned := 0
	for _, f := range factors {
		if (score.TotalScore >= 50 && f.RawScore >= 50) || (score.TotalScore < 50 && f.RawScore < 50) {
			aligned++
		}
	}
	score.FactorsAligned = aligned

	// Save
	if err := cs.surpriseRepo.SaveConfluence(ctx, *score); err != nil {
		log.Printf("[confluence] warn: save: %v", err)
	}

	return score, nil
}

// ComputeAllMajorPairs calculates confluence for all major pairs.
func (cs *ConfluenceScorer) ComputeAllMajorPairs(ctx context.Context) ([]domain.ConfluenceScore, error) {
	pairs := []struct{ Base, Quote string }{
		{"EUR", "USD"}, {"GBP", "USD"}, {"USD", "JPY"},
		{"AUD", "USD"}, {"NZD", "USD"}, {"USD", "CAD"},
		{"USD", "CHF"}, {"EUR", "GBP"}, {"EUR", "JPY"},
		{"GBP", "JPY"}, {"AUD", "JPY"}, {"EUR", "AUD"},
	}

	var results []domain.ConfluenceScore
	for _, p := range pairs {
		score, err := cs.ComputeForPair(ctx, p.Base, p.Quote)
		if err != nil {
			log.Printf("[confluence] warn: %s%s: %v", p.Base, p.Quote, err)
			continue
		}
		results = append(results, *score)
	}

	log.Printf("[confluence] computed %d pair scores", len(results))
	return results, nil
}

// --- Factor computations ---

// Factor 1: COT Positioning (25%)
func (cs *ConfluenceScorer) computeCOTFactor(ctx context.Context, base, quote string) domain.ConfluenceFactor {
	f := domain.ConfluenceFactor{
		Name:   domain.FactorName("COT Positioning"), // FIX: FactorName type
		Weight: 0.25,
	}

	baseContract := currencyToContract(base)   // FIX: was domain.CurrencyToContract
	quoteContract := currencyToContract(quote)

	baseScore := 50.0
	quoteScore := 50.0

	if baseContract != "" {
		if analysis, err := cs.cotRepo.GetLatestAnalysis(ctx, baseContract); err == nil && analysis != nil {
			baseScore = analysis.COTIndex
		}
	}
	if quoteContract != "" {
		if analysis, err := cs.cotRepo.GetLatestAnalysis(ctx, quoteContract); err == nil && analysis != nil {
			quoteScore = analysis.COTIndex
		}
	}

	f.RawScore = mathutil.Clamp(50+(baseScore-quoteScore)/2, 0, 100)
	f.WeightedScore = f.RawScore * f.Weight
	f.Signal = classifyFactorSignal(f.RawScore)

	return f
}

// Factor 2: Economic Surprise (20%)
func (cs *ConfluenceScorer) computeSurpriseFactor(ctx context.Context, base, quote string) domain.ConfluenceFactor {
	f := domain.ConfluenceFactor{
		Name:   domain.FactorName("Economic Surprise"),
		Weight: 0.20,
	}

	// FIX: GetSurpriseIndex takes 2 args (ctx, currency), not 3
	baseIdx, _ := cs.surpriseRepo.GetSurpriseIndex(ctx, base)
	quoteIdx, _ := cs.surpriseRepo.GetSurpriseIndex(ctx, quote)

	baseSurprise := 0.0
	quoteSurprise := 0.0
	if baseIdx != nil {
		baseSurprise = baseIdx.RollingScore
	}
	if quoteIdx != nil {
		quoteSurprise = quoteIdx.RollingScore
	}

	diff := baseSurprise - quoteSurprise
	f.RawScore = mathutil.Clamp(50+diff*2, 0, 100)
	f.WeightedScore = f.RawScore * f.Weight
	f.Signal = classifyFactorSignal(f.RawScore)

	return f
}

// Factor 3: Interest Rate Trajectory (20%)
func (cs *ConfluenceScorer) computeRateFactor(ctx context.Context, base, quote string) domain.ConfluenceFactor {
	f := domain.ConfluenceFactor{
		Name:   domain.FactorName("Rate Trajectory"),
		Weight: 0.20,
	}

	now := timeutil.NowWIB()
	start := now.AddDate(0, 0, -30)

	events, err := cs.eventRepo.GetEventsByDateRange(ctx, start, now)
	if err != nil {
		f.RawScore = 50
		f.WeightedScore = f.RawScore * f.Weight
		f.Signal = "NEUTRAL"
		return f
	}

	rateKeywords := []string{"rate", "interest", "monetary", "policy", "fed fund", "bank rate", "cash rate"}

	baseRateScore := computeRateScore(events, base, rateKeywords)
	quoteRateScore := computeRateScore(events, quote, rateKeywords)

	f.RawScore = mathutil.Clamp(50+(baseRateScore-quoteRateScore)*10, 0, 100)
	f.WeightedScore = f.RawScore * f.Weight
	f.Signal = classifyFactorSignal(f.RawScore)

	return f
}

// Factor 4: Revision Momentum (15%)
func (cs *ConfluenceScorer) computeRevisionFactor(ctx context.Context, base, quote string) domain.ConfluenceFactor {
	f := domain.ConfluenceFactor{
		Name:   domain.FactorName("Revision Momentum"),
		Weight: 0.15,
	}

	baseRevs, _ := cs.eventRepo.GetRevisions(ctx, base, 30)
	quoteRevs, _ := cs.eventRepo.GetRevisions(ctx, quote, 30)

	baseRevScore := revisionDirectionScore(baseRevs)
	quoteRevScore := revisionDirectionScore(quoteRevs)

	f.RawScore = mathutil.Clamp(50+(baseRevScore-quoteRevScore)*25, 0, 100)
	f.WeightedScore = f.RawScore * f.Weight
	f.Signal = classifyFactorSignal(f.RawScore)

	return f
}

// Factor 5: Crowd Sentiment (10%) - Contrarian
func (cs *ConfluenceScorer) computeCrowdFactor(ctx context.Context, base, quote string) domain.ConfluenceFactor {
	f := domain.ConfluenceFactor{
		Name:   domain.FactorName("Crowd Sentiment"),
		Weight: 0.10,
	}

	baseContract := currencyToContract(base)   // FIX: was domain.CurrencyToContract
	quoteContract := currencyToContract(quote)

	baseCrowd := 50.0
	quoteCrowd := 50.0

	if baseContract != "" {
		if analysis, err := cs.cotRepo.GetLatestAnalysis(ctx, baseContract); err == nil && analysis != nil {
			baseCrowd = 100 - analysis.CrowdingIndex
		}
	}
	if quoteContract != "" {
		if analysis, err := cs.cotRepo.GetLatestAnalysis(ctx, quoteContract); err == nil && analysis != nil {
			quoteCrowd = 100 - analysis.CrowdingIndex
		}
	}

	f.RawScore = mathutil.Clamp(50+(baseCrowd-quoteCrowd)/2, 0, 100)
	f.WeightedScore = f.RawScore * f.Weight
	f.Signal = classifyFactorSignal(f.RawScore)

	return f
}

// Factor 6: Event Risk Premium (10%)
func (cs *ConfluenceScorer) computeEventRiskFactor(ctx context.Context, base, quote string) domain.ConfluenceFactor {
	f := domain.ConfluenceFactor{
		Name:   domain.FactorName("Event Risk Premium"),
		Weight: 0.10,
	}

	now := timeutil.NowWIB()
	end := now.AddDate(0, 0, 7)

	events, err := cs.eventRepo.GetEventsByDateRange(ctx, now, end)
	if err != nil {
		f.RawScore = 50
		f.WeightedScore = f.RawScore * f.Weight
		f.Signal = "NEUTRAL"
		return f
	}

	baseHighCount := 0
	quoteHighCount := 0
	for _, ev := range events {
		if ev.Impact != domain.ImpactHigh {
			continue
		}
		if ev.Currency == base {
			baseHighCount++
		}
		if ev.Currency == quote {
			quoteHighCount++
		}
	}

	riskDiff := float64(quoteHighCount - baseHighCount)
	f.RawScore = mathutil.Clamp(50+riskDiff*5, 0, 100)
	f.WeightedScore = f.RawScore * f.Weight
	f.Signal = classifyFactorSignal(f.RawScore)

	return f
}

// --- helpers ---

func classifyFactorSignal(rawScore float64) string {
	switch {
	case rawScore >= 70:
		return "STRONG_BULLISH"
	case rawScore >= 55:
		return "BULLISH"
	case rawScore <= 30:
		return "STRONG_BEARISH"
	case rawScore <= 45:
		return "BEARISH"
	default:
		return "NEUTRAL"
	}
}

func computeConfidence(factors []domain.ConfluenceFactor) float64 {
	bullish := 0
	bearish := 0
	for _, f := range factors {
		if strings.Contains(f.Signal, "BULLISH") {
			bullish++
		} else if strings.Contains(f.Signal, "BEARISH") {
			bearish++
		}
	}

	total := len(factors)
	if total == 0 {
		return 50
	}

	maxAgree := bullish
	if bearish > maxAgree {
		maxAgree = bearish
	}

	return float64(maxAgree) / float64(total) * 100
}

func computeRateScore(events []domain.FFEvent, currency string, keywords []string) float64 {
	score := 0.0
	count := 0

	for _, ev := range events {
		if ev.Currency != currency || ev.Actual == "" {
			continue
		}

		titleLower := strings.ToLower(ev.Title)
		isRate := false
		for _, kw := range keywords {
			if strings.Contains(titleLower, kw) {
				isRate = true
				break
			}
		}
		if !isRate {
			continue
		}

		actual := parseNumericValue(ev.Actual)
		previous := parseNumericValue(ev.Previous)
		if actual > previous {
			score += 1
		} else if actual < previous {
			score -= 1
		}
		count++
	}

	if count == 0 {
		return 0
	}
	return score / float64(count)
}

func revisionDirectionScore(revisions []domain.EventRevision) float64 {
	if len(revisions) == 0 {
		return 0
	}

	upward := 0
	downward := 0
	for _, rev := range revisions {
		switch rev.Direction {
		case "upward":
			upward++
		case "downward":
			downward++
		}
	}

	total := upward + downward
	if total == 0 {
		return 0
	}
	return float64(upward-downward) / float64(total)
}

// FormatConfluenceScore creates a Telegram-formatted confluence display.
func FormatConfluenceScore(score *domain.ConfluenceScore) string {
	if score == nil {
		return "No confluence data."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("=== %s CONFLUENCE ===\n", score.CurrencyPair))
	// FIX: Use Bias instead of Direction, AgreementPct instead of Confidence
	b.WriteString(fmt.Sprintf("Score: %s/100 | %s | Agreement: %.0f%%\n\n",
		fmtutil.FmtNum(score.TotalScore, 1), string(score.Bias), score.AgreementPct))

	for _, f := range score.Factors {
		// FIX: COTIndexBar takes 2 args (score, width)
		bar := fmtutil.COTIndexBar(f.RawScore, 10)
		b.WriteString(fmt.Sprintf("  %s (%.0f%%): %s %s [%s]\n",
			string(f.Name), f.Weight*100,
			fmtutil.FmtNum(f.RawScore, 1), bar, f.Signal))
	}

	return b.String()
}
