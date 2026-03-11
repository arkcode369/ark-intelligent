package ai

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
)

// Interpreter orchestrates AI-powered narrative generation for all analysis types.
// It implements the ports.AIAnalyzer interface, bridging the quantitative
// engines with natural language interpretation via Gemini.
type Interpreter struct {
	gemini    *GeminiClient
	eventRepo ports.EventRepository
	cotRepo   ports.COTRepository
}

// NewInterpreter creates an AI interpreter.
func NewInterpreter(gemini *GeminiClient, eventRepo ports.EventRepository, cotRepo ports.COTRepository) *Interpreter {
	return &Interpreter{
		gemini:    gemini,
		eventRepo: eventRepo,
		cotRepo:   cotRepo,
	}
}

// Ensure Interpreter implements ports.AIAnalyzer at compile time.
var _ ports.AIAnalyzer = (*Interpreter)(nil)

// AnalyzeCOT generates a natural language interpretation of COT positioning data.
func (ip *Interpreter) AnalyzeCOT(ctx context.Context, analyses []domain.COTAnalysis) (string, error) {
	if len(analyses) == 0 {
		return "No COT data available for analysis.", nil
	}

	prompt := BuildCOTAnalysisPrompt(analyses)

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt, prompt)
	if err != nil {
		log.Printf("[ai] COT analysis failed: %v", err)
		return ip.fallbackCOTSummary(analyses), nil
	}

	return formatResponse("COT ANALYSIS", result), nil
}

// PredictEventImpact generates an AI interpretation of an upcoming event.
func (ip *Interpreter) PredictEventImpact(ctx context.Context, event domain.FFEvent, history []domain.FFEventDetail) (string, error) {
	prompt := BuildEventImpactPrompt(event, history)

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt, prompt)
	if err != nil {
		log.Printf("[ai] event impact failed: %v", err)
		return ip.fallbackEventSummary(event, history), nil
	}

	return formatResponse(fmt.Sprintf("%s %s IMPACT", event.Currency, event.Title), result), nil
}

// SynthesizeConfluence generates an AI interpretation of confluence scoring.
func (ip *Interpreter) SynthesizeConfluence(ctx context.Context, score domain.ConfluenceScore) (string, error) {
	prompt := BuildConfluencePrompt(score)

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt, prompt)
	if err != nil {
		log.Printf("[ai] confluence failed: %v", err)
		return fmt.Sprintf("%s Confluence: %.1f/100 (%s) - AI interpretation unavailable",
			score.CurrencyPair, score.TotalScore, score.Direction), nil
	}

	return formatResponse(fmt.Sprintf("%s CONFLUENCE", score.CurrencyPair), result), nil
}

// GenerateWeeklyOutlook creates a comprehensive weekly market outlook.
func (ip *Interpreter) GenerateWeeklyOutlook(ctx context.Context, data ports.WeeklyData) (string, error) {
	// Convert ports.WeeklyData to our internal WeeklyOutlookData
	outlookData := WeeklyOutlookData{
		COTAnalyses:      data.COTAnalyses,
		HighImpactEvents: data.HighImpactEvents,
		SurpriseIndices:  data.SurpriseIndices,
		Rankings:         data.Rankings,
		Confluences:      data.Confluences,
	}

	prompt := BuildWeeklyOutlookPrompt(outlookData)

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt, prompt)
	if err != nil {
		log.Printf("[ai] weekly outlook failed: %v", err)
		return ip.fallbackWeeklyOutlook(outlookData), nil
	}

	return formatResponse("WEEKLY OUTLOOK", result), nil
}

// AnalyzeCrossMarket generates cross-market positioning interpretation.
func (ip *Interpreter) AnalyzeCrossMarket(ctx context.Context, cotData map[string]*domain.COTAnalysis) (string, error) {
	prompt := BuildCrossMarketPrompt(cotData)

	result, err := ip.gemini.GenerateWithSystem(ctx, SystemPrompt, prompt)
	if err != nil {
		log.Printf("[ai] cross-market failed: %v", err)
		return "Cross-market analysis unavailable.", nil
	}

	return formatResponse("CROSS-MARKET ANALYSIS", result), nil
}

// --- Batch Operations ---

// GenerateAllInsights runs all AI analyses and returns combined output.
// Used for the weekly digest Telegram message.
func (ip *Interpreter) GenerateAllInsights(ctx context.Context, data WeeklyOutlookData) (map[string]string, error) {
	results := make(map[string]string)

	// 1. COT Analysis
	if len(data.COTAnalyses) > 0 {
		cotResult, err := ip.AnalyzeCOT(ctx, data.COTAnalyses)
		if err != nil {
			log.Printf("[ai] batch COT: %v", err)
		} else {
			results["cot"] = cotResult
		}
		throttle()
	}

	// 2. Weekly Outlook
	weeklyData := ports.WeeklyData{
		COTAnalyses:      data.COTAnalyses,
		HighImpactEvents: data.HighImpactEvents,
		SurpriseIndices:  data.SurpriseIndices,
		Rankings:         data.Rankings,
		Confluences:      data.Confluences,
	}
	weeklyResult, err := ip.GenerateWeeklyOutlook(ctx, weeklyData)
	if err != nil {
		log.Printf("[ai] batch weekly: %v", err)
	} else {
		results["weekly"] = weeklyResult
	}
	throttle()

	// 3. Cross-Market
	if len(data.COTAnalyses) > 1 {
		cotMap := make(map[string]*domain.COTAnalysis)
		for i := range data.COTAnalyses {
			a := data.COTAnalyses[i]
			cotMap[a.ContractCode] = &a
		}
		crossResult, err := ip.AnalyzeCrossMarket(ctx, cotMap)
		if err != nil {
			log.Printf("[ai] batch cross-market: %v", err)
		} else {
			results["cross_market"] = crossResult
		}
		throttle()
	}

	// 4. Top 3 event impacts
	eventCount := 0
	for _, ev := range data.HighImpactEvents {
		if eventCount >= 3 {
			break
		}
		if ev.Impact != domain.ImpactHigh {
			continue
		}

		history, _ := ip.eventRepo.GetEventHistory(ctx, ev.Title, ev.Currency, 12)
		eventResult, err := ip.PredictEventImpact(ctx, ev, history)
		if err != nil {
			log.Printf("[ai] batch event %s: %v", ev.Title, err)
			continue
		}
		results[fmt.Sprintf("event_%s_%s", ev.Currency, ev.Title)] = eventResult
		eventCount++
		throttle()
	}

	log.Printf("[ai] generated %d insights", len(results))
	return results, nil
}

// --- Fallback summaries (when Gemini is unavailable) ---

func (ip *Interpreter) fallbackCOTSummary(analyses []domain.COTAnalysis) string {
	var b strings.Builder
	b.WriteString("=== COT ANALYSIS (Auto-generated) ===\n\n")

	for _, a := range analyses {
		bias := "NEUTRAL"
		if a.SentimentScore > 20 {
			bias = "BULLISH"
		} else if a.SentimentScore < -20 {
			bias = "BEARISH"
		}

		b.WriteString(fmt.Sprintf("%s: %s\n", a.Currency, bias))
		b.WriteString(fmt.Sprintf("  Spec COT Index: %.0f | Comm: %s\n",
			a.COTIndex, a.CommercialSignal))

		if a.DivergenceFlag {
			b.WriteString("  [!] Spec/Commercial DIVERGENCE detected\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (ip *Interpreter) fallbackEventSummary(event domain.FFEvent, history []domain.FFEventDetail) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("=== %s %s ===\n", event.Currency, event.Title))
	b.WriteString(fmt.Sprintf("Time: %s WIB\n", event.DateTime.Format("Mon 15:04")))
	b.WriteString(fmt.Sprintf("Forecast: %s | Previous: %s\n", event.Forecast, event.Previous))

	if len(history) >= 3 {
		beats := 0
		misses := 0
		for _, h := range history[:min(6, len(history))] {
			actual := parseNumericFallback(h.Actual)
			forecast := parseNumericFallback(h.Forecast)
			if actual > forecast {
				beats++
			} else if actual < forecast {
				misses++
			}
		}
		b.WriteString(fmt.Sprintf("\nRecent track: %d beats, %d misses in last %d\n",
			beats, misses, min(6, len(history))))
	}

	return b.String()
}

func (ip *Interpreter) fallbackWeeklyOutlook(data WeeklyOutlookData) string {
	var b strings.Builder
	b.WriteString("=== WEEKLY OUTLOOK (Auto-generated) ===\n\n")

	// Rankings
	if data.Rankings != nil && len(data.Rankings.Rankings) > 0 {
		b.WriteString("Currency Strength:\n")
		for _, r := range data.Rankings.Rankings {
			b.WriteString(fmt.Sprintf("  %d. %s (%.1f)\n", r.Rank, r.Code, r.CompositeScore))
		}
		b.WriteString("\n")
	}

	// Key events
	if len(data.HighImpactEvents) > 0 {
		b.WriteString("Key Events:\n")
		count := 0
		for _, ev := range data.HighImpactEvents {
			if ev.Impact != domain.ImpactHigh || count >= 5 {
				continue
			}
			b.WriteString(fmt.Sprintf("  %s %s: %s\n",
				ev.DateTime.Format("Mon 15:04"), ev.Currency, ev.Title))
			count++
		}
	}

	b.WriteString("\nNote: AI interpretation unavailable. Showing raw data summary.")
	return b.String()
}

// --- helpers ---

func formatResponse(header, body string) string {
	return fmt.Sprintf("=== %s ===\n\n%s", header, strings.TrimSpace(body))
}

func throttle() {
	// Rate limit Gemini calls: 1 second between requests
	time.Sleep(1 * time.Second)
}

func parseNumericFallback(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimRight(s, "%KkMmBb")
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
