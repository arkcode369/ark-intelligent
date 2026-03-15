package ports

import (
	"context"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
)

// ---------------------------------------------------------------------------
// WeeklyData — Aggregated input for weekly outlook generation
// ---------------------------------------------------------------------------

// WeeklyData bundles all available data for AI weekly outlook generation.
type WeeklyData struct {
	COTAnalyses      []domain.COTAnalysis     `json:"cot_analyses"`
	ConfluenceScores []domain.ConfluenceScore `json:"confluence_scores"`
	CurrencyRanking  *domain.CurrencyRanking  `json:"currency_ranking"`
}

// ---------------------------------------------------------------------------
// AIAnalyzer — Gemini AI interpretation interface
// ---------------------------------------------------------------------------

// AIAnalyzer defines the interface for AI-powered market analysis.
// Primary implementation uses Google Gemini API.
// Fallback: template-based interpretation (no AI required).
type AIAnalyzer interface {
	// AnalyzeCOT generates a narrative interpretation of COT positioning.
	// Input: latest COT analyses for all tracked contracts.
	// Output: 3-4 sentence institutional positioning narrative.
	AnalyzeCOT(ctx context.Context, analyses []domain.COTAnalysis) (string, error)


	// SynthesizeConfluence generates an actionable trading bias paragraph.
	// Input: multi-factor confluence score with all factor breakdowns.
	// Output: actionable narrative with conviction level.
	SynthesizeConfluence(ctx context.Context, score domain.ConfluenceScore) (string, error)

	// GenerateWeeklyOutlook generates a comprehensive weekly briefing.
	// Input: all available data aggregated.
	// Output: 500-800 word market outlook.
	GenerateWeeklyOutlook(ctx context.Context, data WeeklyData) (string, error)

	// AnalyzeCrossMarket generates a risk-on/risk-off regime narrative.
	// Input: COT data across Gold, USD, Bonds, Oil.
	// Output: cross-market correlation analysis.
	AnalyzeCrossMarket(ctx context.Context, cotData map[string]*domain.COTAnalysis) (string, error)

	// IsAvailable returns true if the AI service is configured and reachable.
	IsAvailable() bool
}
