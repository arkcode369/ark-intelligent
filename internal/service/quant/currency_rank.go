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

// CurrencyRanker computes a composite strength score for each major currency
// by aggregating multiple fundamental dimensions:
//   - Interest rate trajectory (from rate events)
//   - Inflation trajectory (CPI/PPI data)
//   - Growth trajectory (GDP, PMI data)
//   - Employment trajectory (NFP, unemployment)
//   - COT positioning score
//   - Economic surprise index
//
// Each dimension is scored 0-100, then weighted into a composite.
// Currencies are ranked strongest-to-weakest for pair selection.
type CurrencyRanker struct {
	eventRepo ports.EventRepository
	cotRepo   ports.COTRepository
}

// NewCurrencyRanker creates a currency ranker.
func NewCurrencyRanker(
	eventRepo ports.EventRepository,
	cotRepo ports.COTRepository,
) *CurrencyRanker {
	return &CurrencyRanker{
		eventRepo: eventRepo,
		cotRepo:   cotRepo,
	}
}

// RankAll computes strength scores for all 8 major currencies and returns
// a sorted ranking (strongest first).
func (cr *CurrencyRanker) RankAll(ctx context.Context) (*domain.CurrencyRanking, error) {
	currencies := []string{"USD", "EUR", "GBP", "JPY", "AUD", "NZD", "CAD", "CHF"}
	now := timeutil.NowWIB()

	var scores []domain.CurrencyScore

	for _, ccy := range currencies {
		score, err := cr.computeScore(ctx, ccy)
		if err != nil {
			log.Printf("[ranker] warn: %s: %v", ccy, err)
			continue
		}
		scores = append(scores, *score)
	}

	// Sort by composite score descending
	sortScores(scores)

	// Assign ranks
	rankedCurrencies := make([]domain.RankedCurrency, len(scores))
	for i, s := range scores {
		rankedCurrencies[i] = domain.RankedCurrency{
			Rank:  i + 1,
			Score: s,
		}
	}

	ranking := &domain.CurrencyRanking{
		Rankings:  rankedCurrencies,
		Timestamp: now,
	}

	log.Printf("[ranker] ranked %d currencies", len(scores))
	return ranking, nil
}

// AnalyzePair computes the strength differential between two currencies.
func (cr *CurrencyRanker) AnalyzePair(ctx context.Context, base, quote string) (*domain.PairAnalysis, error) {
	baseScore, err := cr.computeScore(ctx, base)
	if err != nil {
		return nil, fmt.Errorf("compute %s: %w", base, err)
	}

	quoteScore, err := cr.computeScore(ctx, quote)
	if err != nil {
		return nil, fmt.Errorf("compute %s: %w", quote, err)
	}

	diff := baseScore.CompositeScore - quoteScore.CompositeScore

	direction := "NEUTRAL"
	switch {
	case diff > 15:
		direction = "STRONG_BUY"
	case diff > 5:
		direction = "BUY"
	case diff < -15:
		direction = "STRONG_SELL"
	case diff < -5:
		direction = "SELL"
	}

	// Strength magnitude (0-100)
	strength := mathutil.Clamp(mathAbs(diff)*2, 0, 100)

	return &domain.PairAnalysis{
		Base:              domain.CurrencyCode(base),
		Quote:             domain.CurrencyCode(quote),
		ScoreDifferential: diff,
		Direction:         direction,
		Strength:          strength,
		BaseScore:         *baseScore,
		QuoteScore:        *quoteScore,
	}, nil
}

// computeScore calculates the composite strength score for a single currency.
func (cr *CurrencyRanker) computeScore(ctx context.Context, currency string) (*domain.CurrencyScore, error) {
	score := &domain.CurrencyScore{
		Code: domain.CurrencyCode(currency),
	}

	// Dimensions 1-4 & 6 are now neutral (50) due to removal of calendar data
	score.InterestRateScore = 50
	score.InflationScore = 50
	score.GDPScore = 50
	score.EmploymentScore = 50
	score.SurpriseScore = 50

	// Dimension 5: COT Score (15% weight) - STILL FUNCTIONAL
	score.COTScore = cr.computeCOTDimension(ctx, currency)

	// Weighted composite
	score.CompositeScore = score.InterestRateScore*0.20 +
		score.InflationScore*0.15 +
		score.GDPScore*0.20 +
		score.EmploymentScore*0.15 +
		score.COTScore*0.15 +
		score.SurpriseScore*0.15

	score.CompositeScore = mathutil.Clamp(score.CompositeScore, 0, 100)

	return score, nil
}

// --- Dimension Calculations ---


// computeCOTDimension uses COT Index as positioning score.
func (cr *CurrencyRanker) computeCOTDimension(ctx context.Context, currency string) float64 {
	contractCode := domain.CurrencyToContract(currency)
	if contractCode == "" {
		return 50
	}

	analysis, err := cr.cotRepo.GetLatestAnalysis(ctx, contractCode)
	if err != nil || analysis == nil {
		return 50
	}

	// COT Index directly maps to 0-100
	return analysis.COTIndex
}



// --- Formatting ---

// FormatRanking creates a Telegram-formatted currency ranking display.
func FormatRanking(ranking *domain.CurrencyRanking) string {
	if ranking == nil || len(ranking.Rankings) == 0 {
		return "No ranking data available."
	}

	var b strings.Builder
	b.WriteString("=== CURRENCY STRENGTH RANKING ===\n\n")

	for _, s := range ranking.Rankings {
		bar := strengthBar(s.Score.CompositeScore)
		b.WriteString(fmt.Sprintf("%d. %s  %s  %s\n",
			s.Rank, s.Score.Code,
			fmtutil.FmtNum(s.Score.CompositeScore, 1),
			bar))
	}

	// Show strongest/weakest pair suggestion
	if len(ranking.Rankings) >= 2 {
		strongest := ranking.Rankings[0]
		weakest := ranking.Rankings[len(ranking.Rankings)-1]
		b.WriteString(fmt.Sprintf("\nTop pair: %s/%s (diff: %s)\n",
			strongest.Score.Code, weakest.Score.Code,
			fmtutil.FmtNum(strongest.Score.CompositeScore-weakest.Score.CompositeScore, 1)))
	}

	return b.String()
}

// FormatPairAnalysis formats a pair analysis.
func FormatPairAnalysis(pa *domain.PairAnalysis) string {
	if pa == nil {
		return "No pair data."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("=== %s/%s ANALYSIS ===\n", pa.Base, pa.Quote))
	b.WriteString(fmt.Sprintf("Direction: %s | Strength: %s\n\n",
		pa.Direction, fmtutil.FmtNum(pa.Strength, 1)))

	// Base breakdown
	b.WriteString(fmt.Sprintf("%s Score: %s\n", pa.Base, fmtutil.FmtNum(pa.BaseScore.CompositeScore, 1)))
	b.WriteString(fmt.Sprintf("  Rate: %s | CPI: %s | GDP: %s\n",
		fmtutil.FmtNum(pa.BaseScore.InterestRateScore, 0),
		fmtutil.FmtNum(pa.BaseScore.InflationScore, 0),
		fmtutil.FmtNum(pa.BaseScore.GDPScore, 0)))
	b.WriteString(fmt.Sprintf("  Jobs: %s | COT: %s | Surprise: %s\n",
		fmtutil.FmtNum(pa.BaseScore.EmploymentScore, 0),
		fmtutil.FmtNum(pa.BaseScore.COTScore, 0),
		fmtutil.FmtNum(pa.BaseScore.SurpriseScore, 0)))

	// Quote breakdown
	b.WriteString(fmt.Sprintf("\n%s Score: %s\n", pa.Quote, fmtutil.FmtNum(pa.QuoteScore.CompositeScore, 1)))
	b.WriteString(fmt.Sprintf("  Rate: %s | CPI: %s | GDP: %s\n",
		fmtutil.FmtNum(pa.QuoteScore.InterestRateScore, 0),
		fmtutil.FmtNum(pa.QuoteScore.InflationScore, 0),
		fmtutil.FmtNum(pa.QuoteScore.GDPScore, 0)))
	b.WriteString(fmt.Sprintf("  Jobs: %s | COT: %s | Surprise: %s\n",
		fmtutil.FmtNum(pa.QuoteScore.EmploymentScore, 0),
		fmtutil.FmtNum(pa.QuoteScore.COTScore, 0),
		fmtutil.FmtNum(pa.QuoteScore.SurpriseScore, 0)))

	b.WriteString(fmt.Sprintf("\nDifferential: %s", fmtutil.FmtNumSigned(pa.ScoreDifferential, 1)))

	return b.String()
}

// --- helpers ---

func strengthBar(score float64) string {
	blocks := int(score / 10)
	if blocks > 10 {
		blocks = 10
	}
	if blocks < 0 {
		blocks = 0
	}
	return "[" + strings.Repeat("#", blocks) + strings.Repeat(" ", 10-blocks) + "]"
}

func sortScores(scores []domain.CurrencyScore) {
	// Insertion sort by CompositeScore descending
	for i := 1; i < len(scores); i++ {
		for j := i; j > 0 && scores[j].CompositeScore > scores[j-1].CompositeScore; j-- {
			scores[j], scores[j-1] = scores[j-1], scores[j]
		}
	}
}

func isNaN(v float64) bool {
	return v != v // NaN != NaN
}

// math.Abs import helper to avoid ambiguity
var mathAbs = func(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
