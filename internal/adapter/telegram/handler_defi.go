package telegram

import (
	"context"

	"github.com/arkcode369/ark-intelligent/internal/service/defi"
)

// registerDeFiCommands wires the /defi command.
func (h *Handler) registerDeFiCommands() {
	h.bot.RegisterCommand("/defi", h.cmdDeFi)
}

// cmdDeFi handles /defi — shows DeFi health dashboard with TVL, DEX volume, and stablecoin supply.
func (h *Handler) cmdDeFi(ctx context.Context, chatID string, _ int64, _ string) error {
	loadingID, _ := h.bot.SendLoading(ctx, chatID, "🌾 Mengambil data DeFi dashboard... ⏳")

	report := defi.GetCachedOrFetch(ctx)

	txt := formatDeFiReport(report)
	if loadingID > 0 {
		_ = h.bot.EditMessage(ctx, chatID, loadingID, txt)
	} else {
		_, _ = h.bot.SendHTML(ctx, chatID, txt)
	}
	return nil
}
