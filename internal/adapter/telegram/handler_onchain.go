package telegram

import (
	"context"

	"github.com/arkcode369/ark-intelligent/internal/service/onchain"
)

// cmdOnChain handles the /onchain command — shows BTC network health (Blockchain.com)
// and BTC + ETH exchange flow data (CoinMetrics).
func (h *Handler) cmdOnChain(ctx context.Context, chatID string, _ int64, _ string) error {
	loadingID, _ := h.bot.SendLoading(ctx, chatID, "⛓️ Mengambil data on-chain... ⏳")

	report := onchain.GetCachedOrFetch(ctx)
	btcHealth := onchain.GetBTCHealth(ctx)

	txt := formatOnChainReport(report, btcHealth)
	if loadingID > 0 {
		_ = h.bot.EditMessage(ctx, chatID, loadingID, txt)
	} else {
		_, _ = h.bot.SendHTML(ctx, chatID, txt)
	}
	return nil
}
