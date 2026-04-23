package telegram

// handler_vpbt.go — /vpbt command: Volume Profile Backtest dashboard
//   /vpbt [SYMBOL] [TIMEFRAME] [MODE] [GRADE]  — run volume profile backtest with chart + inline keyboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/ports"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
	vpbt "github.com/arkcode369/ark-intelligent/internal/service/vpbt"
)

// ---------------------------------------------------------------------------
// VPBTServices — dependencies for the /vpbt command
// ---------------------------------------------------------------------------

// VPBTServices holds the services required for the Volume Profile Backtest command.
type VPBTServices struct {
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore
}

// ---------------------------------------------------------------------------
// Handler wiring
// ---------------------------------------------------------------------------

// VPBTState holds the current backtest state for a chat.
type VPBTState struct {
	Symbol    string
	Timeframe string
	Mode      string
	Grade     string
	UpdatedAt time.Time
}

// vpbtStateCache caches VPBT state per chat.
type vpbtStateCache struct {
	mu    sync.Mutex
	store map[string]*VPBTState
}

func newVPBTStateCache() *vpbtStateCache {
	return &vpbtStateCache{store: make(map[string]*VPBTState)}
}

var vpbtStateTTL = 60 * time.Minute

func (c *vpbtStateCache) get(chatID string) *VPBTState {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.store[chatID]
	if !ok || time.Since(s.UpdatedAt) > vpbtStateTTL {
		return nil
	}
	return s
}

func (c *vpbtStateCache) set(chatID string, s *VPBTState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[chatID] = s
}

// WithVPBT injects VPBTServices into the handler and registers VPBT commands.
func (h *Handler) WithVPBT(v *VPBTServices) *Handler {
	h.vpbt = v
	if v != nil {
		h.vpbtCache = newVPBTStateCache()
		h.registerVPBTCommands()
	}
	return h
}

// registerVPBTCommands wires the VPBT commands into the bot.
func (h *Handler) registerVPBTCommands() {
	h.bot.RegisterCommand("/vpbt", h.cmdVPBT)
	h.bot.RegisterCallback("vpbt:", h.handleVPBTCallback)
}

// ---------------------------------------------------------------------------
// /vpbt — Main Volume Profile Backtest Command
// ---------------------------------------------------------------------------

func (h *Handler) cmdVPBT(ctx context.Context, chatID string, _ int64, args string) error {
	if h.vpbt == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ Volume Profile Backtest Engine not configured.")
		return err
	}

	// Parse args: [SYMBOL] [TIMEFRAME] [MODE] [GRADE]
	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))

	if len(parts) == 0 {
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			`📊 <b>Volume Profile Backtest — Multi-Mode Analysis</b>

Backtest strategi volume profile dengan 10 mode analisis:

📊 <b>7 Timeframe:</b> 15m, 30m, 1h, 4h, 6h, 12h, daily
🎯 <b>10 VP Modes:</b> profile, vahval, hvn, lvn, session, shape, composite, vwap, confluence, full
⭐ <b>Grade Filter:</b> A (best), B, C (all trades)
📋 <b>Detail Trades:</b> Entry/exit/PnL setiap trade
📈 <b>Metrics:</b> Win rate, Sharpe, drawdown, profit factor

Pilih aset:`, h.vpbtSymbolMenu())
		return err
	}

	symbol := parts[0]
	timeframe := "daily"
	mode := "profile"
	grade := "C"
	if len(parts) > 1 {
		timeframe = normalizeTimeframe(parts[1])
	}
	if len(parts) > 2 {
		mode = normalizeVPMode(parts[2])
	}
	if len(parts) > 3 {
		g := parts[3]
		if g == "A" || g == "B" || g == "C" {
			grade = g
		}
	}

	// Save state
	h.vpbtCache.set(chatID, &VPBTState{
		Symbol:    symbol,
		Timeframe: timeframe,
		Mode:      mode,
		Grade:     grade,
		UpdatedAt: time.Now(),
	})

	return h.runVPBacktest(ctx, chatID, symbol, timeframe, mode, grade, 0)
}

// ---------------------------------------------------------------------------
// Callback Handler
// ---------------------------------------------------------------------------

func (h *Handler) handleVPBTCallback(ctx context.Context, chatID string, msgID int, _ int64, data string) error {
	action := strings.TrimPrefix(data, "vpbt:")

	// Symbol selection from VPBTSymbolMenu (before any other processing)
	if strings.HasPrefix(action, "sym:") {
		sym := strings.TrimPrefix(action, "sym:")
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		return h.cmdVPBT(ctx, chatID, 0, sym)
	}

	// Get current state (or use defaults)
	state := h.vpbtCache.get(chatID)
	symbol := "EUR"
	timeframe := "daily"
	mode := "profile"
	grade := "C"
	if state != nil {
		symbol = state.Symbol
		timeframe = state.Timeframe
		mode = state.Mode
		grade = state.Grade
	}

	switch {
	case action == "daily":
		timeframe = "daily"
	case action == "12h":
		timeframe = "12h"
	case action == "6h":
		timeframe = "6h"
	case action == "4h":
		timeframe = "4h"
	case action == "1h":
		timeframe = "1h"
	case action == "30m":
		timeframe = "30m"
	case action == "15m":
		timeframe = "15m"
	case action == "modeProfile":
		mode = "profile"
	case action == "modeVahval":
		mode = "vahval"
	case action == "modeHvn":
		mode = "hvn"
	case action == "modeLvn":
		mode = "lvn"
	case action == "modeSession":
		mode = "session"
	case action == "modeShape":
		mode = "shape"
	case action == "modeComposite":
		mode = "composite"
	case action == "modeVwap":
		mode = "vwap"
	case action == "modeConfluence":
		mode = "confluence"
	case action == "modeFull":
		mode = "full"
	case action == "gradeA":
		grade = "A"
	case action == "gradeB":
		grade = "B"
	case action == "gradeC":
		grade = "C"
	case action == "refresh":
		// refresh uses current state
	case action == "trades":
		return h.showVPBTTrades(ctx, chatID, msgID, symbol, timeframe, mode, grade)
	default:
		return nil
	}

	// Update state
	h.vpbtCache.set(chatID, &VPBTState{
		Symbol:    symbol,
		Timeframe: timeframe,
		Mode:      mode,
		Grade:     grade,
		UpdatedAt: time.Now(),
	})

	// Delete old message and send new one
	_ = h.bot.DeleteMessage(ctx, chatID, msgID)
	return h.runVPBacktest(ctx, chatID, symbol, timeframe, mode, grade, 0)
}

// ---------------------------------------------------------------------------
// Core backtest execution
// ---------------------------------------------------------------------------

func (h *Handler) runVPBacktest(ctx context.Context, chatID string, symbol, timeframe, mode, grade string, editMsgID int) error {
	// Resolve symbol to contract code
	mapping := h.resolveCTAMapping(symbol)
	if mapping == nil {
		msg := fmt.Sprintf("❌ Symbol <code>%s</code> tidak ditemukan.\nContoh: <code>/vpbt EUR</code>, <code>/vpbt XAU</code>",
			html.EscapeString(symbol))
		if editMsgID > 0 {
			return h.bot.EditMessage(ctx, chatID, editMsgID, msg)
		}
		_, err := h.bot.SendHTML(ctx, chatID, msg)
		return err
	}

	if h.vpbt.DailyPriceRepo == nil {
		h.sendUserError(ctx, chatID, fmt.Errorf("daily price data not configured"), "vpbt")
		return nil
	}

	// Send loading
	loadingID, _ := h.bot.SendLoading(ctx, chatID, fmt.Sprintf(
		"⏳ Menjalankan volume profile backtest <b>%s</b> (%s, %s, Grade ≥ %s)...\n<i>Ini bisa memakan waktu 15-40 detik.</i>",
		html.EscapeString(mapping.Currency), timeframe, mode, grade,
	))

	// Fetch bars
	var bars []ta.OHLCV
	code := mapping.ContractCode

	switch timeframe {
	case "daily":
		dailyRecords, err := h.vpbt.DailyPriceRepo.GetDailyHistory(ctx, code, 500)
		if err != nil || len(dailyRecords) < 50 {
			if loadingID > 0 {
				_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
			}
			cnt := 0
			if dailyRecords != nil {
				cnt = len(dailyRecords)
			}
			_, err2 := h.bot.SendHTML(ctx, chatID,
				fmt.Sprintf("❌ Data daily tidak cukup untuk %s (%d bars, minimal 65).", mapping.Currency, cnt))
			return err2
		}
		bars = ta.DailyPricesToOHLCV(dailyRecords)
	case "12h", "6h", "4h", "1h", "30m", "15m":
		if h.vpbt.IntradayRepo == nil {
			if loadingID > 0 {
				_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
			}
			h.sendUserError(ctx, chatID, fmt.Errorf("intraday data repository not configured"), "vpbt")
			return nil
		}
		// Determine bar count based on timeframe granularity
		count := 600
		switch timeframe {
		case "15m":
			count = 5000 // ~52 days of 15m bars
		case "30m":
			count = 2500
		case "1h":
			count = 1200
		case "4h":
			count = 600
		case "6h":
			count = 400
		case "12h":
			count = 300
		}
		intradayBars, err := h.vpbt.IntradayRepo.GetHistory(ctx, code, timeframe, count)
		if err != nil || len(intradayBars) < 50 {
			if loadingID > 0 {
				_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
			}
			cnt := 0
			if intradayBars != nil {
				cnt = len(intradayBars)
			}
			_, err2 := h.bot.SendHTML(ctx, chatID,
				fmt.Sprintf("❌ Data %s tidak cukup untuk %s (%d bars, minimal 65).", timeframe, mapping.Currency, cnt))
			return err2
		}
		bars = ta.IntradayBarsToOHLCV(intradayBars)
	default:
		// fallback to daily
		timeframe = "daily"
		dailyRecords, err := h.vpbt.DailyPriceRepo.GetDailyHistory(ctx, code, 500)
		if err != nil || len(dailyRecords) < 50 {
			if loadingID > 0 {
				_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
			}
			_, err2 := h.bot.SendHTML(ctx, chatID,
				fmt.Sprintf("❌ Data daily tidak cukup untuk %s.", mapping.Currency))
			return err2
		}
		bars = ta.DailyPricesToOHLCV(dailyRecords)
	}

	// Run VP backtest using Python engine with real data
	vpResult, err := vpbt.RunVPBacktest(ctx, mapping.Currency, timeframe, mode, grade, bars)
	if err != nil {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		log.Error().Err(err).Str("symbol", mapping.Currency).Str("timeframe", timeframe).Str("mode", mode).Msg("vp backtest failed")

		// User-friendly error messages
		errMsg := err.Error()
		userMsg := "❌ Backtest gagal. "
		if strings.Contains(errMsg, "timeout") {
			userMsg += "Data terlalu besar untuk timeframe ini. Coba timeframe yang lebih longgar (daily/4h) atau mode yang lebih sederhana."
		} else if strings.Contains(errMsg, "not enough data") || strings.Contains(errMsg, "Insufficient") {
			userMsg += "Data tidak cukup untuk backtest. Minimal 100 bars diperlukan."
		} else if strings.Contains(errMsg, "volume") {
			userMsg += "Volume data tidak tersedia, menggunakan proxy range-based."
		} else {
			userMsg += fmt.Sprintf("Error: %v", err)
		}

		_, err2 := h.bot.SendHTML(ctx, chatID, userMsg)
		return err2
	}

	// Generate chart from Python result
	chartPNG, chartErr := h.generateVPChartFromVPResult(ctx, vpResult, mapping.Currency, timeframe, mode)
	if chartErr != nil {
		log.Error().Err(chartErr).Str("symbol", symbol).Str("timeframe", timeframe).Str("mode", mode).Msg("vp chart generation failed, falling back to text")
	}

	// Delete loading
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}

	// Format result
	summary := formatVPBTResult(vpResult)
	kb := h.vpbtMenu()

	// Send chart + caption with keyboard
	if chartPNG != nil && len(chartPNG) > 0 {
		_, err := h.bot.SendPhotoWithKeyboard(ctx, chatID, chartPNG, summary, kb)
		if err != nil {
			return err
		}
		return nil
	}

	// Chart unavailable: prepend notification so user knows chart exists but failed
	if chartErr != nil {
		summary = "📊 <i>Chart sementara tidak tersedia. Menampilkan analisis teks.</i>\n\n" + summary
	}
	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, summary, kb)
	return err
}

// ---------------------------------------------------------------------------
// Trade detail view
// ---------------------------------------------------------------------------

func (h *Handler) showVPBTTrades(ctx context.Context, chatID string, msgID int, symbol, timeframe, mode, grade string) error {
	// Re-run backtest to get trades (lightweight enough since data is cached)
	mapping := h.resolveCTAMapping(symbol)
	if mapping == nil {
		return h.bot.EditMessage(ctx, chatID, msgID, "❌ Symbol not found.")
	}

	if h.vpbt.DailyPriceRepo == nil {
		return h.bot.EditMessage(ctx, chatID, msgID, "❌ Daily price data not configured.")
	}

	code := mapping.ContractCode
	var bars []ta.OHLCV

	switch timeframe {
	case "daily":
		records, err := h.vpbt.DailyPriceRepo.GetDailyHistory(ctx, code, 500)
		if err != nil || len(records) < 50 {
			return h.bot.EditMessage(ctx, chatID, msgID, "❌ Insufficient data.")
		}
		bars = ta.DailyPricesToOHLCV(records)
	case "12h", "6h", "4h", "1h", "30m", "15m":
		if h.vpbt.IntradayRepo == nil {
			return h.bot.EditMessage(ctx, chatID, msgID, "❌ Intraday not available.")
		}
		count := 600
		switch timeframe {
		case "15m":
			count = 5000
		case "30m":
			count = 2500
		case "1h":
			count = 1200
		case "4h":
			count = 600
		case "6h":
			count = 400
		case "12h":
			count = 300
		}
		intBars, err := h.vpbt.IntradayRepo.GetHistory(ctx, code, timeframe, count)
		if err != nil || len(intBars) < 50 {
			return h.bot.EditMessage(ctx, chatID, msgID, "❌ Insufficient data.")
		}
		bars = ta.IntradayBarsToOHLCV(intBars)
	default:
		return h.bot.EditMessage(ctx, chatID, msgID, "❌ Invalid timeframe.")
	}

	params := ta.DefaultBacktestParams()
	params.Symbol = mapping.Currency
	params.Timeframe = timeframe
	params.MinGrade = grade

	vpResult, err := vpbt.RunVPBacktest(ctx, mapping.Currency, timeframe, mode, grade, bars)
	if err != nil || vpResult == nil || len(vpResult.Result.Trades) == 0 {
		return h.bot.EditMessage(ctx, chatID, msgID, "❌ Tidak ada trade yang dihasilkan atau error: "+err.Error())
	}
	result := convertVPResultToBacktestResult(vpResult, mapping.Currency, timeframe)

	// Format last 10 trades
	txt := formatTradeList(result, mapping.Currency, timeframe)
	kb := h.vpbtMenu()

	// Delete old and send new (might be too long for edit)
	_ = h.bot.DeleteMessage(ctx, chatID, msgID)
	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
	return err
}

// ---------------------------------------------------------------------------
// Chart generation
// ---------------------------------------------------------------------------

// vpChartInput is the JSON structure for the vpbt chart Python script.
type vpChartInput struct {
	EquityCurve []float64     `json:"equity_curve"`
	TradeDates  []string      `json:"trade_dates"`
	TradePnL    []float64     `json:"trade_pnl"`
	Drawdown    []float64     `json:"drawdown"`
	Symbol      string        `json:"symbol"`
	Timeframe   string        `json:"timeframe"`
	Mode        string        `json:"mode"`
	Params      vpChartParams `json:"params"`
	VPLevels    []vpLevel     `json:"vp_levels"`
}

type vpChartParams struct {
	StartEquity float64 `json:"start_equity"`
	TotalTrades int     `json:"total_trades"`
	WinRate     float64 `json:"win_rate"`
	TotalReturn float64 `json:"total_return"`
	MaxDD       float64 `json:"max_dd"`
	Sharpe      float64 `json:"sharpe"`
	PF          float64 `json:"pf"`
	Mode        string  `json:"mode"`
}

type vpLevel struct {
	Price  float64 `json:"price"`
	Volume float64 `json:"volume"`
	Type   string  `json:"type"` // "hvn", "lvn", "poc", "vah", "val"
}

func (h *Handler) generateVPChart(ctx context.Context, result *ta.BacktestResult, symbol, timeframe, mode string) (pngData []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in generateVPChart: %v", r)
			log.Error().Interface("panic", r).Str("symbol", symbol).Str("timeframe", timeframe).Str("mode", mode).Msg("recovered panic in generateVPChart")
		}
	}()
	if result == nil {
		return nil, fmt.Errorf("no backtest result")
	}

	// If no trades, create a flat equity line with start equity
	equityCurve := result.EquityCurve
	if len(equityCurve) < 2 {
		equityCurve = []float64{result.Params.StartEquity, result.Params.StartEquity}
	}

	// Build trade dates and PnL arrays (one entry per trade / equity point)
	tradeDates := make([]string, len(result.Trades))
	tradePnL := make([]float64, len(result.Trades))
	for i, t := range result.Trades {
		tradeDates[i] = t.ExitDate.Format("2006-01-02")
		tradePnL[i] = t.PnLPercent
	}

	// Compute drawdown from equity curve
	drawdown := make([]float64, len(equityCurve))
	peak := equityCurve[0]
	for i, eq := range equityCurve {
		if eq > peak {
			peak = eq
		}
		if peak > 0 {
			drawdown[i] = (eq - peak) / peak * 100.0
		}
	}

	// Build VP levels (placeholder - would come from backtest metadata in real implementation)
	vpLevels := []vpLevel{
		{Price: 1.0850, Volume: 12500, Type: "poc"},
		{Price: 1.0880, Volume: 8500, Type: "vah"},
		{Price: 1.0820, Volume: 9200, Type: "val"},
		{Price: 1.0900, Volume: 6800, Type: "hvn"},
		{Price: 1.0800, Volume: 5400, Type: "lvn"},
	}

	input := vpChartInput{
		EquityCurve: equityCurve,
		TradeDates:  tradeDates,
		TradePnL:    tradePnL,
		Drawdown:    drawdown,
		Symbol:      symbol,
		Timeframe:   timeframe,
		Mode:        mode,
		Params: vpChartParams{
			StartEquity: result.Params.StartEquity,
			TotalTrades: result.TotalTrades,
			WinRate:     result.WinRate,
			TotalReturn: result.TotalPnLPercent,
			MaxDD:       -result.MaxDrawdown,
			Sharpe:      result.SharpeRatio,
			PF:          result.ProfitFactor,
			Mode:        mode,
		},
		VPLevels: vpLevels,
	}

	// Sanitize NaN/Inf for JSON marshaling
	sanitizeVPFloat := func(v float64) float64 {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0
		}
		return v
	}
	for i := range input.EquityCurve {
		input.EquityCurve[i] = sanitizeVPFloat(input.EquityCurve[i])
	}
	for i := range input.TradePnL {
		input.TradePnL[i] = sanitizeVPFloat(input.TradePnL[i])
	}
	for i := range input.Drawdown {
		input.Drawdown[i] = sanitizeVPFloat(input.Drawdown[i])
	}
	for i := range input.VPLevels {
		input.VPLevels[i].Price = sanitizeVPFloat(input.VPLevels[i].Price)
		input.VPLevels[i].Volume = sanitizeVPFloat(input.VPLevels[i].Volume)
	}
	input.Params.WinRate = sanitizeVPFloat(input.Params.WinRate)
	input.Params.TotalReturn = sanitizeVPFloat(input.Params.TotalReturn)
	input.Params.MaxDD = sanitizeVPFloat(input.Params.MaxDD)
	input.Params.Sharpe = sanitizeVPFloat(input.Params.Sharpe)
	input.Params.PF = sanitizeVPFloat(input.Params.PF)

	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal chart input: %w", err)
	}

	// Write temp files
	tmpDir := os.TempDir()
	inputPath := filepath.Join(tmpDir, fmt.Sprintf("vpbt_input_%d.json", time.Now().UnixNano()))
	outputPath := filepath.Join(tmpDir, fmt.Sprintf("vpbt_output_%d.png", time.Now().UnixNano()))

	if err := os.WriteFile(inputPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("write chart input: %w", err)
	}
	defer os.Remove(inputPath)
	defer os.Remove(outputPath) // ensure PNG is cleaned up on all return paths

	scriptPath, findErr := findVPChartScript()
	if findErr != nil {
		return nil, findErr
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Error().Err(err).Str("stderr", stderr.String()).Msg("vp chart renderer failed")
		return nil, fmt.Errorf("vp chart renderer failed: %w", err)
	}

	pngData, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		return nil, fmt.Errorf("read chart output: %w", readErr)
	}

	return pngData, nil
}

// convertVPResultToBacktestResult converts VPBacktestResult to ta.BacktestResult
func convertVPResultToBacktestResult(vpResult *vpbt.VPBacktestResult, symbol, timeframe string) *ta.BacktestResult {
	result := &ta.BacktestResult{
		Params: ta.BacktestParams{
			Symbol:    symbol,
			Timeframe: timeframe,
		},
		Trades:          make([]ta.TradeRecord, len(vpResult.Result.Trades)),
		TotalTrades:     vpResult.Result.TotalTrades,
		WinRate:         vpResult.Result.WinRate * 100,
		TotalPnLPercent: vpResult.Result.TotalPnL,
		MaxDrawdown:     vpResult.Result.MaxDrawdown,
		SharpeRatio:     vpResult.Result.SharpeRatio,
		ProfitFactor:    vpResult.Result.ProfitFactor,
		AvgWin:          vpResult.Result.AvgWin,
		AvgLoss:         vpResult.Result.AvgLoss,
		ExpectedValue:   vpResult.Result.ExpectedValue,
		EquityCurve:     vpResult.Result.EquityCurve,
	}

	for i, t := range vpResult.Result.Trades {
		result.Trades[i] = ta.TradeRecord{
			EntryPrice: t.EntryPrice,
			ExitPrice:  t.ExitPrice,
			Direction:  strings.ToUpper(t.Direction),
			PnLPercent: t.PnL,
			ExitReason: t.Reason,
			Grade:      t.Grade,
		}
	}

	return result
}

// generateVPChartFromVPResult generates chart directly from VPBacktestResult
func (h *Handler) generateVPChartFromVPResult(ctx context.Context, vpResult *vpbt.VPBacktestResult, symbol, timeframe, mode string) (pngData []byte, err error) {
	if vpResult == nil || len(vpResult.Result.EquityCurve) == 0 {
		return nil, fmt.Errorf("no data for chart")
	}

	// Build chart input from VP result
	input := vpChartInput{
		EquityCurve: vpResult.Result.EquityCurve,
		TradeDates:  make([]string, len(vpResult.Result.Trades)),
		TradePnL:    make([]float64, len(vpResult.Result.Trades)),
		Drawdown:    vpResult.Result.Drawdown,
		Symbol:      symbol,
		Timeframe:   timeframe,
		Mode:        mode,
		Params: vpChartParams{
			StartEquity: 10000,
			TotalTrades: vpResult.Result.TotalTrades,
			WinRate:     vpResult.Result.WinRate * 100,
			TotalReturn: vpResult.Result.TotalPnL,
			MaxDD:       vpResult.Result.MaxDrawdown,
			Sharpe:      vpResult.Result.SharpeRatio,
			PF:          vpResult.Result.ProfitFactor,
			Mode:        mode,
		},
		VPLevels: make([]vpLevel, 0),
	}

	// Convert trades to chart format
	for i, t := range vpResult.Result.Trades {
		if i < len(input.TradeDates) {
			input.TradeDates[i] = t.EntryTime
		}
		if i < len(input.TradePnL) {
			input.TradePnL[i] = t.PnL
		}
	}

	// Add VP levels
	if vpResult.Result.VPLLevels.Prices != nil {
		for i, price := range vpResult.Result.VPLLevels.Prices {
			if i < len(vpResult.Result.VPLLevels.Volumes) {
				input.VPLevels = append(input.VPLevels, vpLevel{
					Price:  price,
					Volume: vpResult.Result.VPLLevels.Volumes[i],
					Type:   "hvn", // Default, could be more sophisticated
				})
			}
		}
	}

	// Add POC, VAH, VAL as special levels
	if vpResult.Result.VPLLevels.POC > 0 {
		input.VPLevels = append(input.VPLevels, vpLevel{Price: vpResult.Result.VPLLevels.POC, Volume: 0, Type: "poc"})
	}
	if vpResult.Result.VPLLevels.VAH > 0 {
		input.VPLevels = append(input.VPLevels, vpLevel{Price: vpResult.Result.VPLLevels.VAH, Volume: 0, Type: "vah"})
	}
	if vpResult.Result.VPLLevels.VAL > 0 {
		input.VPLevels = append(input.VPLevels, vpLevel{Price: vpResult.Result.VPLLevels.VAL, Volume: 0, Type: "val"})
	}

	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal chart input: %w", err)
	}

	// Write temp files
	tmpDir := os.TempDir()
	inputPath := filepath.Join(tmpDir, fmt.Sprintf("vpbt_chart_input_%d.json", time.Now().UnixNano()))
	outputPath := filepath.Join(tmpDir, fmt.Sprintf("vpbt_chart_output_%d.png", time.Now().UnixNano()))

	if err := os.WriteFile(inputPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("write chart input: %w", err)
	}
	defer os.Remove(inputPath)
	defer os.Remove(outputPath)

	scriptPath, findErr := findVPChartScript()
	if findErr != nil {
		return nil, findErr
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Error().Err(err).Str("stderr", stderr.String()).Msg("vp chart renderer failed")
		return nil, fmt.Errorf("vp chart renderer failed: %w", err)
	}

	pngData, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		return nil, fmt.Errorf("read chart output: %w", readErr)
	}

	return pngData, nil
}

// findVPChartScript locates the vpbt_chart.py script.
func findVPChartScript() (string, error) {
	candidates := []string{
		"scripts/vpbt_chart.py",
		"../scripts/vpbt_chart.py",
	}
	if d := os.Getenv("SCRIPTS_DIR"); d != "" {
		candidates = append([]string{filepath.Join(d, "vpbt_chart.py")}, candidates...)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		rel := filepath.Join(execDir, "scripts", "vpbt_chart.py")
		if _, err := os.Stat(rel); err == nil {
			return rel, nil
		}
		rel = filepath.Join(execDir, "..", "scripts", "vpbt_chart.py")
		if _, err := os.Stat(rel); err == nil {
			abs, _ := filepath.Abs(rel)
			return abs, nil
		}
	}
	return "", fmt.Errorf("vpbt_chart.py not found (searched: %v)", candidates)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// formatVPBTResult formats a VPBacktestResult as a Telegram HTML message.
func formatVPBTResult(r *vpbt.VPBacktestResult) string {
	if !r.Success {
		return "❌ <b>Backtest Failed</b>"
	}

	var sb strings.Builder
	rslt := r.Result

	sb.WriteString(fmt.Sprintf("📊 <b>Volume Profile Backtest</b>\n\n"))
	sb.WriteString(fmt.Sprintf("<b>Total Trades:</b> %d\n", rslt.TotalTrades))
	sb.WriteString(fmt.Sprintf("<b>Win Rate:</b> %.1f%%\n", rslt.WinRate*100))
	sb.WriteString(fmt.Sprintf("<b>Profit Factor:</b> %.2f\n", rslt.ProfitFactor))
	sb.WriteString(fmt.Sprintf("<b>Sharpe Ratio:</b> %.2f\n", rslt.SharpeRatio))
	sb.WriteString(fmt.Sprintf("<b>Max Drawdown:</b> %.1f%%\n", rslt.MaxDrawdown*100))
	sb.WriteString(fmt.Sprintf("<b>Total P&L:</b> %.2f\n", rslt.TotalPnL))
	sb.WriteString(fmt.Sprintf("<b>Win/Loss:</b> %d/%d\n", rslt.WinCount, rslt.LossCount))

	if r.TextOutput != "" {
		sb.WriteString(fmt.Sprintf("\n<i>%s</i>", html.EscapeString(r.TextOutput)))
	}

	return sb.String()
}

// normalizeVPMode normalizes user input to a recognized VP mode.
func normalizeVPMode(mode string) string {
	switch strings.ToLower(mode) {
	case "profile", "p":
		return "profile"
	case "vahval", "va":
		return "vahval"
	case "hvn", "h":
		return "hvn"
	case "lvn", "l":
		return "lvn"
	case "session", "s":
		return "session"
	case "shape", "sh":
		return "shape"
	case "composite", "c":
		return "composite"
	case "vwap", "v":
		return "vwap"
	case "confluence", "cf":
		return "confluence"
	case "full", "f":
		return "full"
	default:
		return "profile"
	}
}

// ---------------------------------------------------------------------------
// Keyboard builders for /vpbt
// ---------------------------------------------------------------------------

// vpbtSymbolMenu returns the symbol selection keyboard for /vpbt.
func (h *Handler) vpbtSymbolMenu() ports.InlineKeyboard {
	// All available symbols from price data (consistent with your data sources)
	symbols := []string{
		// Forex Majors
		"EUR", "GBP", "JPY", "CHF", "AUD", "CAD", "NZD", "USD",
		// Commodities
		"XAU", "XAG", "COPPER", "OIL", "ULSD", "RBOB",
		// Bonds & Indices
		"BOND", "BOND30", "BOND5", "BOND2", "SPX500", "NDX", "DJI", "RUT",
		// Crypto
		"BTC", "ETH",
		// Cross pairs (synthetic)
		"XAUEUR", "XAUGBP", "XAGEUR", "XAGGBP",
	}
	// Grid layout: 4 columns for better UX
	rows := make([][]ports.InlineButton, 0)
	for i := 0; i < len(symbols); i += 4 {
		end := i + 4
		if end > len(symbols) {
			end = len(symbols)
		}
		row := make([]ports.InlineButton, 0, 4)
		for j := i; j < end; j++ {
			row = append(row, ports.InlineButton{
				Text:         fmt.Sprintf("📈 %s", symbols[j]),
				CallbackData: fmt.Sprintf("vpbt:sym:%s", symbols[j]),
			})
		}
		rows = append(rows, row)
	}
	return ports.InlineKeyboard{Rows: rows}
}

// vpbtMenu returns the main menu keyboard for /vpbt results.
func (h *Handler) vpbtMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "🔄 Refresh", CallbackData: "vpbt:refresh"},
				{Text: "📋 Detail Trades", CallbackData: "vpbt:trades"},
			},
			{
				{Text: "📊 Daily", CallbackData: "vpbt:daily"},
				{Text: "📊 12h", CallbackData: "vpbt:12h"},
				{Text: "📊 6h", CallbackData: "vpbt:6h"},
			},
			{
				{Text: "📊 4h", CallbackData: "vpbt:4h"},
				{Text: "📊 1h", CallbackData: "vpbt:1h"},
				{Text: "📊 30m", CallbackData: "vpbt:30m"},
				{Text: "📊 15m", CallbackData: "vpbt:15m"},
			},
			{
				{Text: "🎯 Profile", CallbackData: "vpbt:modeProfile"},
				{Text: "🎯 VAH/VAL", CallbackData: "vpbt:modeVahval"},
				{Text: "🎯 HVN", CallbackData: "vpbt:modeHvn"},
				{Text: "🎯 LVN", CallbackData: "vpbt:modeLvn"},
			},
			{
				{Text: "🎯 Session", CallbackData: "vpbt:modeSession"},
				{Text: "🎯 Shape", CallbackData: "vpbt:modeShape"},
				{Text: "🎯 Composite", CallbackData: "vpbt:modeComposite"},
				{Text: "🎯 VWAP", CallbackData: "vpbt:modeVwap"},
			},
			{
				{Text: "🎯 Confluence", CallbackData: "vpbt:modeConfluence"},
				{Text: "🎯 Full", CallbackData: "vpbt:modeFull"},
			},
			{
				{Text: "⭐ Grade A", CallbackData: "vpbt:gradeA"},
				{Text: "⭐⭐ Grade B", CallbackData: "vpbt:gradeB"},
				{Text: "⭐⭐⭐ Grade C", CallbackData: "vpbt:gradeC"},
			},
		},
	}
}
