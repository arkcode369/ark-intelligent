package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
)

// cmdBacktest handles /backtest [contract|all|signals]
func (h *Handler) cmdBacktest(ctx context.Context, chatID string, userID int64, args string) error {
	if h.signalRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Backtest data not available yet. Signal tracking is being initialized.")
		return err
	}

	calc := backtestsvc.NewStatsCalculator(h.signalRepo)
	args = strings.TrimSpace(strings.ToUpper(args))

	switch {
	case args == "" || args == "ALL":
		return h.backtestAll(ctx, chatID, calc)
	case args == "SIGNALS" || args == "TYPES":
		return h.backtestBySignalType(ctx, chatID, calc)
	default:
		return h.backtestByContract(ctx, chatID, calc, args)
	}
}

func (h *Handler) backtestAll(ctx context.Context, chatID string, calc *backtestsvc.StatsCalculator) error {
	stats, err := calc.ComputeAll(ctx)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Error: %s", err))
		return sendErr
	}

	if stats.TotalSignals == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "No signal data available yet. Signals are generated on each COT release.")
		return err
	}

	html := h.fmt.FormatBacktestStats(stats)
	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}

func (h *Handler) backtestBySignalType(ctx context.Context, chatID string, calc *backtestsvc.StatsCalculator) error {
	statsMap, err := calc.ComputeAllBySignalType(ctx)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Error: %s", err))
		return sendErr
	}

	if len(statsMap) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "No signal data available yet.")
		return err
	}

	html := h.fmt.FormatBacktestSummary(statsMap, "Signal Type")
	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}

func (h *Handler) backtestByContract(ctx context.Context, chatID string, calc *backtestsvc.StatsCalculator, currency string) error {
	// Resolve currency to contract code
	mapping := domain.FindPriceMappingByCurrency(currency)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Unknown currency: %s\n\nUsage: /backtest [all|signals|EUR|GBP|...]", currency))
		return err
	}

	stats, err := calc.ComputeByContract(ctx, mapping.ContractCode)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Error: %s", err))
		return sendErr
	}

	if stats.TotalSignals == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("No signal data for %s yet.", currency))
		return err
	}

	stats.GroupLabel = currency
	html := h.fmt.FormatBacktestStats(stats)
	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}

// cmdAccuracy handles /accuracy — quick one-line accuracy summary
func (h *Handler) cmdAccuracy(ctx context.Context, chatID string, userID int64, args string) error {
	if h.signalRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Backtest data not available yet.")
		return err
	}

	calc := backtestsvc.NewStatsCalculator(h.signalRepo)
	stats, err := calc.ComputeAll(ctx)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Error: %s", err))
		return sendErr
	}

	if stats.Evaluated == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "No evaluated signals yet. Outcomes are calculated after price data becomes available.")
		return err
	}

	html := fmt.Sprintf(
		"\xF0\x9F\x8E\xAF <b>Signal Accuracy</b>\n\n"+
			"<code>Signals  :</code> %d total, %d evaluated\n"+
			"<code>Win Rate :</code> 1W %.1f%% | 2W %.1f%% | 4W %.1f%%\n"+
			"<code>Best     :</code> %s at %.1f%%\n"+
			"<code>Avg Conf :</code> %.0f%% (calibration error: %.1f%%)\n\n"+
			"<i>Use /backtest for detailed breakdown</i>",
		stats.TotalSignals, stats.Evaluated,
		stats.WinRate1W, stats.WinRate2W, stats.WinRate4W,
		stats.BestPeriod, stats.BestWinRate,
		stats.AvgConfidence, stats.CalibrationError,
	)

	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}
