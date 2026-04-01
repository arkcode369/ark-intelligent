package telegram

import (
	"fmt"
	"math"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/orderflow"
)

// FormatOrderFlowResult formats an OrderFlowResult as an HTML Telegram message.
// Output is kept below 3000 characters by limiting the delta bar display to 7 bars.
func (f *Formatter) FormatOrderFlowResult(r *orderflow.OrderFlowResult) string {
	var b strings.Builder

	// ── Header ───────────────────────────────────────────────────────────────
	b.WriteString(fmt.Sprintf("📊 <b>ORDER FLOW — %s %s</b>\n\n",
		r.Symbol, r.Timeframe))

	if len(r.Bars) == 0 {
		b.WriteString(r.Summary)
		return b.String()
	}

	b.WriteString(fmt.Sprintf("<i>%d bars dianalisis</i>\n\n", len(r.Bars)))

	// ── Price-Delta Divergence ────────────────────────────────────────────────
	switch r.PriceDeltaDivergence {
	case "BULLISH_DIV":
		b.WriteString("⚡ <b>DELTA:</b> BULLISH DIVERGENCE ⬆️\n")
		b.WriteString("  Price: Lower Low — Delta: Higher Low\n")
		b.WriteString("  → Buyers menyerap tekanan jual\n\n")
	case "BEARISH_DIV":
		b.WriteString("⚡ <b>DELTA:</b> BEARISH DIVERGENCE ⬇️\n")
		b.WriteString("  Price: Higher High — Delta: Lower High\n")
		b.WriteString("  → Selling tersembunyi di balik harga baru tinggi\n\n")
	default:
		b.WriteString("⚡ <b>DELTA:</b> Tidak ada divergence signifikan\n\n")
	}

	// ── Delta Bars (last 7, newest first) ─────────────────────────────────────
	displayCount := 7
	if len(r.Bars) < displayCount {
		displayCount = len(r.Bars)
	}
	b.WriteString("📊 <b>DELTA BARS (terbaru):</b>\n")
	for i := 0; i < displayCount; i++ {
		db := r.Bars[i]
		dir := "▲"
		icon := "🟢"
		if db.Delta < 0 {
			dir = "▼"
			icon = "🔴"
		}
		note := ""
		if isAbsorptionBar(i, r.BullishAbsorption) {
			note = " ← potential bullish absorption"
		} else if isAbsorptionBar(i, r.BearishAbsorption) {
			note = " ← potential bearish absorption"
		}
		b.WriteString(fmt.Sprintf("  <code>%.5f</code> %s %s %+.0f%s\n",
			db.OHLCV.Close, dir, icon, db.Delta, note))
	}
	b.WriteString("\n")

	// ── Cumulative Delta ──────────────────────────────────────────────────────
	cumIcon := "🟢"
	if r.CumDelta < 0 {
		cumIcon = "🔴"
	}
	b.WriteString(fmt.Sprintf("📈 <b>CUM. DELTA:</b> %s %+.0f  |  Trend: <b>%s</b>\n\n",
		cumIcon, r.CumDelta, r.DeltaTrend))

	// ── POC ───────────────────────────────────────────────────────────────────
	b.WriteString(fmt.Sprintf("🎯 <b>POINT OF CONTROL:</b> <code>%.5f</code>\n\n",
		r.PointOfControl))

	// ── Absorption Patterns ───────────────────────────────────────────────────
	if len(r.BullishAbsorption) > 0 || len(r.BearishAbsorption) > 0 {
		b.WriteString("🔰 <b>ABSORPTION DETECTED:</b>\n")
		for _, idx := range r.BullishAbsorption {
			if idx >= len(r.Bars) {
				continue
			}
			db := r.Bars[idx]
			b.WriteString(fmt.Sprintf("  Bar %.5f: Vol tinggi, delta %+.0f, range kecil\n",
				db.OHLCV.Close, db.Delta))
			b.WriteString("  → Potential bullish absorption — sellers running out\n")
		}
		for _, idx := range r.BearishAbsorption {
			if idx >= len(r.Bars) {
				continue
			}
			db := r.Bars[idx]
			b.WriteString(fmt.Sprintf("  Bar %.5f: Vol tinggi, delta %+.0f, range kecil\n",
				db.OHLCV.Close, db.Delta))
			b.WriteString("  → Potential bearish absorption — buyers running out\n")
		}
		b.WriteString("\n")
	}

	// ── Overall Bias ──────────────────────────────────────────────────────────
	biasEmoji := "⚪"
	switch r.Bias {
	case "BULLISH":
		biasEmoji = "🟢"
	case "BEARISH":
		biasEmoji = "🔴"
	}
	b.WriteString(fmt.Sprintf("%s <b>BIAS:</b> %s\n", biasEmoji, r.Bias))

	// ── Summary ───────────────────────────────────────────────────────────────
	b.WriteString(fmt.Sprintf("\n💡 %s\n", r.Summary))

	// ── Truncate safety net ───────────────────────────────────────────────────
	out := b.String()
	if len(out) > 3000 {
		out = out[:2990] + "\n<i>...</i>"
	}
	return out
}

// isAbsorptionBar returns true if idx appears in the indices slice.
func isAbsorptionBar(idx int, indices []int) bool {
	for _, v := range indices {
		if v == idx {
			return true
		}
	}
	return false
}

// formatDeltaSign formats a delta value with consistent sign display.
// Exported so it can be called from tests if needed.
func formatDeltaSign(delta float64) string {
	if math.IsNaN(delta) || math.IsInf(delta, 0) {
		return "N/A"
	}
	return fmt.Sprintf("%+.0f", delta)
}
