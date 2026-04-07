package telegram

// handler_bis.go — /bis command: BIS Statistics Dashboard.
// Displays central bank policy rates (WS_CBPOL) and credit-to-GDP gaps
// (WS_CREDIT_GAP) alongside REER data (WS_EER) from the BIS free API.

import (
	"context"

	"github.com/arkcode369/ark-intelligent/internal/service/bis"
)

// cmdBIS handles /bis — BIS Statistics dashboard (policy rates, credit gaps, REER).
func (h *Handler) cmdBIS(ctx context.Context, chatID string, _ int64, _ string) error {
	loadingID, _ := h.bot.SendLoading(ctx, chatID, "🏦 Mengambil data BIS policy rates... ⏳")

	policyCh := make(chan *bis.PolicyRateSuite, 1)
	creditCh := make(chan *bis.CreditGapReport, 1)
	reerCh := make(chan *bis.BISData, 1)

	go func() { policyCh <- bis.GetPolicyRates(ctx) }()
	go func() { creditCh <- bis.GetCreditGaps(ctx) }()
	go func() {
		reer, _ := bis.GetCachedOrFetch(ctx)
		reerCh <- reer
	}()

	policy := <-policyCh
	creditGap := <-creditCh
	reer := <-reerCh

	txt := formatBISDashboard(reer, policy, creditGap)
	if loadingID > 0 {
		_ = h.bot.EditMessage(ctx, chatID, int(loadingID), txt)
		return nil
	}
	_, err := h.bot.SendHTML(ctx, chatID, txt)
	return err
}
