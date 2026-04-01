package telegram

// handler_orderflow.go — /orderflow command handler.
// Implements estimated delta and order flow analysis from OHLCV data.
// Uses tick-rule volume approximation for forex; suitable for any asset with OHLCV data.

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/orderflow"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ---------------------------------------------------------------------------
// OrderFlowServices — injected dependencies
// ---------------------------------------------------------------------------

// OrderFlowServices holds dependencies for the /orderflow command.
type OrderFlowServices struct {
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore // may be nil
	Engine         *orderflow.Engine
}

// ---------------------------------------------------------------------------
// WithOrderFlow wires the order flow handler into the main Handler.
// ---------------------------------------------------------------------------

// WithOrderFlow registers the /orderflow command on the handler.
func (h *Handler) WithOrderFlow(svc OrderFlowServices) {
	h.orderflow = &svc
	h.bot.RegisterCommand("/orderflow", h.cmdOrderFlow)
}

// ---------------------------------------------------------------------------
// /orderflow command
// ---------------------------------------------------------------------------

// cmdOrderFlow handles /orderflow [SYMBOL] [TIMEFRAME]
// Examples:
//   /orderflow EURUSD
//   /orderflow XAUUSD H4
//   /orderflow BTCUSD daily
func (h *Handler) cmdOrderFlow(ctx context.Context, chatID string, userID int64, args string) error {
	if h.orderflow == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚠️ Order Flow engine tidak tersedia.")
		return err
	}

	parts := strings.Fields(strings.TrimSpace(strings.ToUpper(args)))
	if len(parts) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, `📊 <b>Order Flow Analysis</b>

Gunakan: <code>/orderflow [SYMBOL] [TIMEFRAME]</code>

Contoh:
  <code>/orderflow EURUSD</code>
  <code>/orderflow XAUUSD H4</code>
  <code>/orderflow BTCUSD daily</code>

Timeframe: <code>daily</code>, <code>4h</code>, <code>1h</code>

Analisis delta estimasi, divergence harga-delta, POC, dan absorption.`)
		return err
	}

	currency := parts[0]
	timeframe := "daily"
	if len(parts) > 1 {
		switch strings.ToLower(parts[1]) {
		case "h4", "4hour", "4h":
			timeframe = "4h"
		case "h1", "1hour", "1h":
			timeframe = "1h"
		case "d", "d1", "daily":
			timeframe = "daily"
		default:
			timeframe = strings.ToLower(parts[1])
		}
	}

	mapping := domain.FindPriceMappingByCurrency(currency)
	if mapping == nil || mapping.RiskOnly {
		_, err := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("❌ Symbol tidak dikenal: <code>%s</code>", html.EscapeString(currency)))
		return err
	}

	msgID, _ := h.bot.SendLoading(ctx, chatID,
		fmt.Sprintf("⏳ Menganalisis Order Flow <b>%s</b> (%s)...",
			html.EscapeString(mapping.Currency), strings.ToUpper(timeframe)))

	bars, err := h.fetchOrderFlowBars(ctx, mapping, timeframe)
	if err != nil || len(bars) == 0 {
		errMsg := fmt.Sprintf("❌ Gagal mengambil data harga untuk <b>%s</b>: %s",
			html.EscapeString(mapping.Currency), html.EscapeString(fmt.Sprintf("%v", err)))
		if msgID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		}
		_, sendErr := h.bot.SendHTML(ctx, chatID, errMsg)
		return sendErr
	}

	result := h.orderflow.Engine.Analyze(mapping.Currency, strings.ToUpper(timeframe), bars)
	output := h.fmt.FormatOrderFlowResult(result)

	if msgID > 0 {
		return h.bot.EditMessage(ctx, chatID, msgID, output)
	}
	_, sendErr := h.bot.SendHTML(ctx, chatID, output)
	return sendErr
}

// ---------------------------------------------------------------------------
// fetchOrderFlowBars — fetch OHLCV bars for order flow analysis
// ---------------------------------------------------------------------------

func (h *Handler) fetchOrderFlowBars(ctx context.Context, mapping *domain.PriceSymbolMapping, timeframe string) ([]ta.OHLCV, error) {
	code := mapping.ContractCode

	switch timeframe {
	case "4h", "1h":
		if h.orderflow.IntradayRepo == nil {
			return nil, fmt.Errorf("intraday data tidak tersedia")
		}
		intradayBars, err := h.orderflow.IntradayRepo.GetHistory(ctx, code, timeframe, 50)
		if err != nil {
			return nil, fmt.Errorf("fetch intraday bars: %w", err)
		}
		return ta.IntradayBarsToOHLCV(intradayBars), nil

	default: // "daily"
		dailyRecords, err := h.orderflow.DailyPriceRepo.GetDailyHistory(ctx, code, 50)
		if err != nil {
			return nil, fmt.Errorf("fetch daily bars: %w", err)
		}
		return ta.DailyPricesToOHLCV(dailyRecords), nil
	}
}
