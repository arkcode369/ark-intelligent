package telegram

import (
	"context"

	fred "github.com/arkcode369/ark-intelligent/internal/service/fred"
)

// cmdCarry handles the /carry command — shows carry trade monitor with
// ranked pairs and unwind detection.
func (h *Handler) cmdCarry(ctx context.Context, chatID string, _ int64, _ string) error {
	loadingID, _ := h.bot.SendLoading(ctx, chatID, "💰 Mengambil data carry trades... ⏳")

	monitor := fred.GetCarryMonitor()
	result, err := monitor.FetchCarryDashboard(ctx)
	if err != nil {
		errMsg := "❌ <b>Carry monitor error</b>\n\n" +
			"<code>" + err.Error() + "</code>\n\n" +
			"<i>FRED API may be temporarily unavailable.</i>"
		if loadingID > 0 {
			_ = h.bot.EditMessage(ctx, chatID, loadingID, errMsg)
		} else {
			_, _ = h.bot.SendHTML(ctx, chatID, errMsg)
		}
		return nil
	}

	text := h.fmt.FormatCarryMonitor(result)
	if loadingID > 0 {
		_ = h.bot.EditMessage(ctx, chatID, loadingID, text)
	} else {
		_, _ = h.bot.SendHTML(ctx, chatID, text)
	}
	return nil
}
