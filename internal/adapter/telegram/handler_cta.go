package telegram

// handler_cta.go — /cta command: Classical Technical Analysis dashboard
//   /cta [SYMBOL] [TIMEFRAME]  — TA dashboard with chart + inline keyboard
//
// This file contains only orchestration logic (command dispatch + callback routing).
// Chart generation: chart_cta.go
// Output formatting: formatter_cta.go

import (
	"context"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ---------------------------------------------------------------------------
// CTAServices — dependencies for the /cta command
// ---------------------------------------------------------------------------

// CTAServices holds the services required for the CTA command.
type CTAServices struct {
	TAEngine       *ta.Engine
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore
	PriceMapping   []domain.PriceSymbolMapping
}

// ---------------------------------------------------------------------------
// ctaState — cached computation results
// ---------------------------------------------------------------------------

type ctaState struct {
	symbol     string
	currency   string
	daily      *ta.FullResult
	h4         *ta.FullResult
	h1         *ta.FullResult
	m15        *ta.FullResult
	m30        *ta.FullResult
	h6         *ta.FullResult
	h12        *ta.FullResult
	weekly     *ta.FullResult
	mtf        *ta.MTFResult
	bars       map[string][]ta.OHLCV // timeframe -> bars
	chartData  map[string][]byte     // timeframe -> PNG bytes (lazy-generated)
	computedAt time.Time
}

var ctaStateTTL = config.CTAStateTTL

// ctaStateCache stores per-chat CTA state with TTL.
type ctaStateCache struct {
	mu    sync.Mutex
	store map[string]*ctaState // chatID -> state
}

func newCTAStateCache() *ctaStateCache {
	return &ctaStateCache{
		store: make(map[string]*ctaState),
	}
}

func (c *ctaStateCache) get(chatID string) *ctaState {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.store[chatID]
	if !ok || time.Since(s.computedAt) > ctaStateTTL {
		return nil
	}
	return s
}

func (c *ctaStateCache) set(chatID string, s *ctaState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Opportunistic cleanup
	if len(c.store) > 50 {
		now := time.Now()
		for k, v := range c.store {
			if now.Sub(v.computedAt) > ctaStateTTL*2 {
				delete(c.store, k)
			}
		}
	}
	c.store[chatID] = s
}

// ---------------------------------------------------------------------------
// Handler wiring
// ---------------------------------------------------------------------------

// WithCTA injects CTAServices into the handler and registers CTA commands.
func (h *Handler) WithCTA(c *CTAServices) *Handler {
	h.cta = c
	if c != nil {
		h.ctaCache = newCTAStateCache()
		h.registerCTACommands()
	}
	return h
}

// registerCTACommands wires the CTA commands into the bot.
func (h *Handler) registerCTACommands() {
	h.bot.RegisterCommand("/cta", h.cmdCTA)
	h.bot.RegisterCallback("cta:", h.handleCTACallback)
}

// ---------------------------------------------------------------------------
// /cta — Main CTA Command
// ---------------------------------------------------------------------------

func (h *Handler) cmdCTA(ctx context.Context, chatID string, _ int64, args string) error {
	if h.cta == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ CTA Engine not configured.")
		return err
	}

	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))
	if len(parts) == 0 {
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			`📈 <b>CTA — Classical Technical Analysis</b>

Multi-timeframe TA dashboard dengan 6 tools:

📊 <b>Chart</b> — Candlestick + indikator per TF (15m-daily)
🏯 <b>Ichimoku</b> — Cloud, Tenkan/Kijun, signal
📐 <b>Fibonacci</b> — Swing levels + Golden Zone
🕯 <b>Patterns</b> — Candlestick pattern detection
⚡ <b>Confluence</b> — Multi-indicator agreement score
📱 <b>Multi-TF</b> — Alignment semua timeframe
🎯 <b>Zones</b> — Entry/SL/TP otomatis

Pilih aset:`, h.kb.CTASymbolMenu())
		return err
	}

	symbol := parts[0]

	mapping := h.resolveCTAMapping(symbol)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"❌ Symbol <code>%s</code> tidak ditemukan.\nContoh: <code>/cta EUR</code>, <code>/cta XAU</code>, <code>/cta BTC</code>",
			html.EscapeString(symbol),
		))
		return err
	}

	loadingID, _ := h.bot.SendLoading(ctx, chatID, fmt.Sprintf("⚡ Computing TA for <b>%s</b>... ⏳", html.EscapeString(mapping.Currency)))

	state, err := h.computeCTAState(ctx, mapping)
	if err != nil {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		h.sendUserError(ctx, chatID, err, "cta")
		return nil
	}

	h.ctaCache.set(chatID, state)

	chartPNG, chartErr := h.generateCTAChart(state, "daily")
	if chartErr != nil {
		log.Warn().Err(chartErr).Str("symbol", symbol).Msg("chart generation failed")
	}

	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}

	summary := formatCTASummary(state)
	kb := h.kb.CTAMenu()

	if chartPNG != nil && len(chartPNG) > 0 {
		state.chartData["daily"] = chartPNG
		shortCaption := fmt.Sprintf("⚡ <b>CTA: %s</b> — Daily", html.EscapeString(mapping.Currency))
		_, photoErr := h.bot.SendPhotoWithKeyboard(ctx, chatID, chartPNG, shortCaption, kb)
		if photoErr != nil {
			log.Warn().Err(photoErr).Msg("send CTA photo failed, falling back to text")
			_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, summary, kb)
			return err
		}
		_, err = h.bot.SendHTML(ctx, chatID, summary)
		return err
	}

	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, summary, kb)
	return err
}

// ---------------------------------------------------------------------------
// Callback Handler
// ---------------------------------------------------------------------------

func (h *Handler) handleCTACallback(ctx context.Context, chatID string, msgID int, _ int64, data string) error {
	action := strings.TrimPrefix(data, "cta:")

	if strings.HasPrefix(action, "sym:") {
		sym := strings.TrimPrefix(action, "sym:")
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		return h.cmdCTA(ctx, chatID, 0, sym)
	}

	state := h.ctaCache.get(chatID)
	if state == nil {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		_, err := h.bot.SendHTML(ctx, chatID, "⏳ Data expired. Gunakan /cta untuk refresh.")
		return err
	}

	switch {
	case action == "back":
		return h.ctaShowSummaryChart(ctx, chatID, msgID, state, "daily")

	case action == "refresh":
		mapping := h.resolveCTAMapping(state.currency)
		if mapping == nil {
			return h.bot.EditMessage(ctx, chatID, msgID, "❌ Symbol not found.")
		}
		newState, err := h.computeCTAState(ctx, mapping)
		if err != nil {
			h.editUserError(ctx, chatID, msgID, err, "cta")
			return nil
		}
		h.ctaCache.set(chatID, newState)
		return h.ctaShowSummaryChart(ctx, chatID, msgID, newState, "daily")

	case strings.HasPrefix(action, "tf:"):
		tf := strings.TrimPrefix(action, "tf:")
		return h.ctaShowTimeframe(ctx, chatID, msgID, state, tf)

	case action == "ichi":
		txt := formatCTAIchimoku(state)
		kb := h.kb.CTADetailMenu()
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		chartPNG, chartErr := h.generateCTADetailChart(ctx, state, "daily", "ichimoku")
		if chartErr == nil && len(chartPNG) > 0 {
			shortCaption := fmt.Sprintf("🏯 Ichimoku Cloud — %s", html.EscapeString(state.symbol))
			_, _ = h.bot.SendPhoto(ctx, chatID, chartPNG, shortCaption)
		}
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
		return err

	case action == "fib":
		txt := formatCTAFibonacci(state)
		kb := h.kb.CTADetailMenu()
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		chartPNG, chartErr := h.generateCTADetailChart(ctx, state, "daily", "fibonacci")
		if chartErr == nil && len(chartPNG) > 0 {
			shortCaption := fmt.Sprintf("📐 Fibonacci — %s", html.EscapeString(state.symbol))
			_, _ = h.bot.SendPhoto(ctx, chatID, chartPNG, shortCaption)
		}
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
		return err

	case action == "patterns":
		txt := formatCTAPatterns(state)
		kb := h.kb.CTADetailMenu()
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
		return err

	case action == "confluence":
		txt := formatCTAConfluence(state)
		kb := h.kb.CTADetailMenu()
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
		return err

	case action == "mtf":
		txt := formatCTAMTF(state)
		kb := h.kb.CTADetailMenu()
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
		return err

	case action == "zones":
		txt := formatCTAZones(state)
		kb := h.kb.CTADetailMenu()
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		chartPNG, chartErr := h.generateCTADetailChart(ctx, state, "daily", "zones")
		if chartErr == nil && len(chartPNG) > 0 {
			shortCaption := fmt.Sprintf("🎯 Trade Setup — %s", html.EscapeString(state.symbol))
			_, _ = h.bot.SendPhoto(ctx, chatID, chartPNG, shortCaption)
		}
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
		return err
	}

	return nil
}

// ctaShowSummaryChart deletes old message and sends new photo (can't edit text→photo).
func (h *Handler) ctaShowSummaryChart(ctx context.Context, chatID string, msgID int, state *ctaState, tf string) error {
	chartPNG, err := h.getCTAChart(state, tf)
	if err != nil || len(chartPNG) == 0 {
		summary := formatCTASummary(state)
		kb := h.kb.CTAMenu()
		return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, summary, kb)
	}

	_ = h.bot.DeleteMessage(ctx, chatID, msgID)
	summary := formatCTASummary(state)
	kb := h.kb.CTAMenu()
	shortCaption := fmt.Sprintf("⚡ <b>CTA: %s</b> — Daily", html.EscapeString(state.symbol))
	_, photoErr := h.bot.SendPhotoWithKeyboard(ctx, chatID, chartPNG, shortCaption, kb)
	if photoErr != nil {
		_, sendErr := h.bot.SendWithKeyboardChunked(ctx, chatID, summary, kb)
		return sendErr
	}
	_, sendErr := h.bot.SendHTML(ctx, chatID, summary)
	return sendErr
}

// ctaShowTimeframe shows a specific timeframe detail + chart.
func (h *Handler) ctaShowTimeframe(ctx context.Context, chatID string, msgID int, state *ctaState, tf string) error {
	result := h.getCTAResult(state, tf)
	if result == nil {
		return h.bot.EditMessage(ctx, chatID, msgID, fmt.Sprintf("⚠️ Data %s tidak tersedia.", tf))
	}

	chartPNG, _ := h.getCTAChart(state, tf)
	txt := formatCTATimeframeDetail(state, tf, result)
	kb := h.kb.CTATimeframeMenu()

	if chartPNG != nil && len(chartPNG) > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		shortCaption := fmt.Sprintf("⚡ <b>CTA: %s</b> — %s", html.EscapeString(state.symbol), strings.ToUpper(tf))
		_, photoErr := h.bot.SendPhotoWithKeyboard(ctx, chatID, chartPNG, shortCaption, kb)
		if photoErr != nil {
			_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
			return err
		}
		_, err := h.bot.SendHTML(ctx, chatID, txt)
		return err
	}

	return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, txt, kb)
}

// ---------------------------------------------------------------------------
// Symbol Resolution
// ---------------------------------------------------------------------------

func (h *Handler) resolveCTAMapping(symbol string) *domain.PriceSymbolMapping {
	return domain.FindPriceMappingByCurrency(strings.ToUpper(symbol))
}
