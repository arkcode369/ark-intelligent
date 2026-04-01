package telegram

import (
	"context"

	"github.com/arkcode369/ark-intelligent/internal/service/onchain"
)

// registerOnChainCommands wires the /onchain command.
func (h *Handler) registerOnChainCommands() {
	h.bot.RegisterCommand("/onchain", h.cmdOnChain)
}

// cmdOnChain handles the /onchain command — shows BTC + ETH exchange flow data
// from CoinMetrics community API.
func (h *Handler) cmdOnChain(ctx context.Context, chatID string, _ int64, _ string) error {
	h.bot.SendTyping(ctx, chatID)

	report := onchain.GetCachedOrFetch(ctx)

	txt := formatOnChainReport(report)
	_, err := h.bot.SendHTML(ctx, chatID, txt)
	return err
}
