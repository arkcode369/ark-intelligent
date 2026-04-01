package telegram

// /briefing, /br — Daily Morning Briefing

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
)

// ---------------------------------------------------------------------------
// /briefing, /br — Daily Morning Briefing
// ---------------------------------------------------------------------------

func (h *Handler) cmdBriefing(ctx context.Context, chatID string, userID int64, args string) error {
	return h.sendBriefing(ctx, chatID, userID, 0)
}

// cbBriefing handles "briefing:" prefixed callbacks.
func (h *Handler) cbBriefing(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "briefing:")
	switch action {
	case "refresh":
		return h.sendBriefing(ctx, chatID, userID, msgID)
	default:
		return nil
	}
}

// sendBriefing aggregates all data sources and sends (or edits) a briefing message.
// msgID == 0 → send new; msgID > 0 → edit existing.
func (h *Handler) sendBriefing(ctx context.Context, chatID string, userID int64, msgID int) error {
	now := timeutil.NowWIB()
	dateStr := now.Format("20060102")

	// ------------------------------------------------------------------
	// Parallel fetch: events, COT analyses, FRED macro (all best-effort)
	// ------------------------------------------------------------------
	var (
		todayEvents []domain.NewsEvent
		analyses    []domain.COTAnalysis
		macroData   *fred.MacroData

		wg sync.WaitGroup
		mu sync.Mutex
	)

	wg.Add(3)

	go func() {
		defer wg.Done()
		evts, err := h.newsRepo.GetByDate(ctx, dateStr)
		if err == nil {
			mu.Lock()
			todayEvents = evts
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		a, err := h.cotRepo.GetAllLatestAnalyses(ctx)
		if err == nil {
			mu.Lock()
			analyses = a
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		md, err := fred.GetCachedOrFetch(ctx)
		if err == nil {
			mu.Lock()
			macroData = md
			mu.Unlock()
		}
	}()

	wg.Wait()

	// ------------------------------------------------------------------
	// Build conviction scores (best-effort, non-fatal)
	// ------------------------------------------------------------------
	convictions := h.buildBriefingConvictions(ctx, analyses, macroData)

	// ------------------------------------------------------------------
	// Format & send
	// ------------------------------------------------------------------
	html := h.fmt.FormatBriefing(now, todayEvents, convictions)
	kb := h.kb.BriefingMenu()

	if msgID > 0 {
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
	}
	_, err := h.bot.SendWithKeyboard(ctx, chatID, html, kb)
	return err
}

// buildBriefingConvictions computes conviction scores for all available COT analyses.
func (h *Handler) buildBriefingConvictions(ctx context.Context, analyses []domain.COTAnalysis, macroData *fred.MacroData) []cot.ConvictionScore {
	if len(analyses) == 0 {
		return nil
	}

	var regime fred.MacroRegime
	if macroData != nil {
		composites := fred.ComputeComposites(macroData)
		regime = fred.ClassifyMacroRegime(macroData, composites)
	}

	// Build price contexts (best-effort)
	var priceCtxs map[string]*domain.PriceContext
	if h.priceRepo != nil {
		builder := pricesvc.NewContextBuilder(h.priceRepo)
		if pcs, err := builder.BuildAll(ctx); err == nil {
			priceCtxs = pcs
		}
	}

	convictions := make([]cot.ConvictionScore, 0, len(analyses))
	for _, a := range analyses {
		surpriseSigma := 0.0
		if h.newsScheduler != nil {
			surpriseSigma = h.newsScheduler.GetSurpriseSigma(a.Contract.Currency)
		}
		var pc *domain.PriceContext
		if priceCtxs != nil {
			pc = priceCtxs[a.Contract.Code]
		}
		cs := cot.ComputeConvictionScoreV3(a, regime, surpriseSigma, "", macroData, pc)
		convictions = append(convictions, cs)
	}

	// Sort descending by absolute score (strongest conviction first)
	sort.Slice(convictions, func(i, j int) bool {
		si := convictions[i].Score
		sj := convictions[j].Score
		if si < 0 {
			si = -si
		}
		if sj < 0 {
			sj = -sj
		}
		return si > sj
	})

	return convictions
}

// ---------------------------------------------------------------------------
// Formatter method — FormatBriefing
// ---------------------------------------------------------------------------

// FormatBriefing builds the HTML for the daily briefing message (max ~15 lines).
func (f *Formatter) FormatBriefing(
	now time.Time,
	events []domain.NewsEvent,
	convictions []cot.ConvictionScore,
) string {
	var sb strings.Builder

	dateLabel := now.Format("Monday, 02 January 2006")
	timeLabel := now.Format("15:04")

	sb.WriteString("🌅 <b>ARK Daily Briefing</b>\n")
	sb.WriteString(fmt.Sprintf("<i>📅 %s · %s WIB</i>\n\n", dateLabel, timeLabel))

	// ------------------------------------------------------------------
	// Section 1: Economic Events Today (High + Medium impact only)
	// ------------------------------------------------------------------
	highMedEvents := briefingFilterEvents(events)
	sb.WriteString("📅 <b>Events Hari Ini</b>\n")
	if len(highMedEvents) > 0 {
		shown := 0
		for _, e := range highMedEvents {
			if shown >= 6 {
				if remaining := len(highMedEvents) - shown; remaining > 0 {
					sb.WriteString(fmt.Sprintf("<i>  +%d event lainnya</i>\n", remaining))
				}
				break
			}
			timeStr := e.Time
			if !e.TimeWIB.IsZero() {
				timeStr = e.TimeWIB.Format("15:04")
			}
			sb.WriteString(fmt.Sprintf("%s %s · <b>%s</b> — %s\n",
				e.FormatImpactColor(), timeStr, e.Currency, e.Event))
			shown++
		}
	} else {
		sb.WriteString("<i>Tidak ada event High/Medium hari ini.</i>\n")
	}
	sb.WriteString("\n")

	// ------------------------------------------------------------------
	// Section 2: Top COT Conviction Signals
	// ------------------------------------------------------------------
	sb.WriteString("🎯 <b>Top COT Signals</b>\n")
	if len(convictions) > 0 {
		limit := 5
		if len(convictions) < limit {
			limit = len(convictions)
		}
		for _, cs := range convictions[:limit] {
			icon := "⚪"
			switch cs.COTBias {
			case "BULLISH":
				icon = "🟢"
			case "BEARISH":
				icon = "🔴"
			}
			pct := cs.Score
			if pct < 0 {
				pct = -pct
			}
			dir := cs.Direction
			if dir == "" {
				dir = cs.COTBias
			}
			sb.WriteString(fmt.Sprintf("%s <b>%s</b>: %s (%.0f%%)\n",
				icon, cs.Currency, dir, pct))
		}
	} else {
		sb.WriteString("<i>COT data belum tersedia.</i>\n")
	}
	sb.WriteString("\n")

	// ------------------------------------------------------------------
	// Section 3: Quick Bias Summary 1-liner
	// ------------------------------------------------------------------
	if len(convictions) > 0 {
		sb.WriteString("📊 <b>Bias Summary</b>\n")
		var bulls, bears, neutrals []string
		for _, cs := range convictions {
			switch cs.COTBias {
			case "BULLISH":
				bulls = append(bulls, cs.Currency)
			case "BEARISH":
				bears = append(bears, cs.Currency)
			default:
				neutrals = append(neutrals, cs.Currency)
			}
		}
		parts := make([]string, 0, 3)
		if len(bulls) > 0 {
			parts = append(parts, fmt.Sprintf("🟢 %s", strings.Join(bulls, ", ")))
		}
		if len(bears) > 0 {
			parts = append(parts, fmt.Sprintf("🔴 %s", strings.Join(bears, ", ")))
		}
		if len(neutrals) > 0 {
			parts = append(parts, fmt.Sprintf("⚪ %s", strings.Join(neutrals, ", ")))
		}
		sb.WriteString(strings.Join(parts, " · "))
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf("<code>Updated: %s WIB</code>", timeLabel))
	return sb.String()
}

// briefingFilterEvents returns events with High or Medium impact, sorted by TimeWIB ascending.
func briefingFilterEvents(events []domain.NewsEvent) []domain.NewsEvent {
	var out []domain.NewsEvent
	for _, e := range events {
		imp := strings.ToLower(e.Impact)
		if imp == "high" || imp == "medium" {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ti, tj := out[i].TimeWIB, out[j].TimeWIB
		if ti.IsZero() && tj.IsZero() {
			return false
		}
		if ti.IsZero() {
			return false
		}
		if tj.IsZero() {
			return true
		}
		return ti.Before(tj)
	})
	return out
}

// ---------------------------------------------------------------------------
// KeyboardBuilder — BriefingMenu
// ---------------------------------------------------------------------------

// BriefingMenu returns the inline keyboard for the /briefing command.
func (kb *KeyboardBuilder) BriefingMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "🔄 Refresh", CallbackData: "briefing:refresh"},
				{Text: "📅 Calendar", CallbackData: "cmd:calendar"},
				{Text: "📊 COT", CallbackData: "cmd:cot"},
			},
			{
				{Text: "🏠 Home", CallbackData: "nav:home"},
			},
		},
	}
}
