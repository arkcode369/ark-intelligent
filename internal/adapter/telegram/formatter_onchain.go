package telegram

import (
	"fmt"
	"math"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/onchain"
)

// formatOnChainReport builds the Telegram HTML output for the /onchain command.
func formatOnChainReport(report *onchain.OnChainReport) string {
	if report == nil || !report.Available || len(report.Assets) == 0 {
		return "⛓ <b>On-Chain Metrics</b>\n\n⚠️ Data not available. Try again later."
	}

	var sb strings.Builder
	sb.WriteString("⛓ <b>On-Chain Exchange Flow Report</b>\n")
	sb.WriteString(fmt.Sprintf("📅 %s\n\n", report.FetchedAt.UTC().Format("2006-01-02 15:04 UTC")))

	// Render each asset in a consistent order.
	assetOrder := []string{"btc", "eth"}
	for _, asset := range assetOrder {
		s, ok := report.Assets[asset]
		if !ok || !s.Available {
			continue
		}
		sb.WriteString(formatAssetOnChain(s))
		sb.WriteString("\n")
	}

	sb.WriteString("💡 <i>Negative net flow = coins leaving exchanges (accumulation)\nPositive net flow = coins entering exchanges (sell pressure)</i>\n")
	sb.WriteString("📊 Data: CoinMetrics Community API")

	return sb.String()
}

func formatAssetOnChain(s *onchain.AssetOnChainSummary) string {
	var sb strings.Builder

	assetEmoji := "₿"
	if s.Asset == "eth" {
		assetEmoji = "Ξ"
	}

	trendEmoji := "➖"
	switch s.FlowTrend {
	case "ACCUMULATION":
		trendEmoji = "🟢 Accumulation"
	case "DISTRIBUTION":
		trendEmoji = "🔴 Distribution"
	case "NEUTRAL":
		trendEmoji = "⚪ Neutral"
	}

	sb.WriteString(fmt.Sprintf("%s <b>%s Exchange Flows</b>\n", assetEmoji, strings.ToUpper(s.Asset)))
	sb.WriteString(fmt.Sprintf("  Trend: %s\n", trendEmoji))
	sb.WriteString(fmt.Sprintf("  Net Flow 7D: %s\n", formatFlowValue(s.NetFlow7D, s.Asset)))
	sb.WriteString(fmt.Sprintf("  Net Flow 30D: %s\n", formatFlowValue(s.NetFlow30D, s.Asset)))

	if s.ConsecutiveOutflow > 0 {
		sb.WriteString(fmt.Sprintf("  🔄 %d consecutive outflow days\n", s.ConsecutiveOutflow))
	}
	if s.LargeInflowSpike {
		sb.WriteString("  ⚠️ Large inflow spike detected (>2x avg)\n")
	}

	if s.ActiveAddresses > 0 {
		sb.WriteString(fmt.Sprintf("  👥 Active Addresses: %s", formatCompactNumber(float64(s.ActiveAddresses))))
		if s.ActiveAddrChange7D != 0 {
			arrow := "📈"
			if s.ActiveAddrChange7D < 0 {
				arrow = "📉"
			}
			sb.WriteString(fmt.Sprintf(" (%s %.1f%%)", arrow, s.ActiveAddrChange7D))
		}
		sb.WriteString("\n")
	}

	if s.TxCount > 0 {
		sb.WriteString(fmt.Sprintf("  📝 Transactions: %s\n", formatCompactNumber(float64(s.TxCount))))
	}

	return sb.String()
}

func formatFlowValue(val float64, asset string) string {
	sign := "+"
	if val < 0 {
		sign = ""
	}
	absVal := math.Abs(val)

	unit := strings.ToUpper(asset)
	if absVal >= 1000 {
		return fmt.Sprintf("%s%.1fK %s", sign, val/1000, unit)
	}
	return fmt.Sprintf("%s%.1f %s", sign, val, unit)
}

func formatCompactNumber(n float64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", n/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", n/1_000)
	default:
		return fmt.Sprintf("%.0f", n)
	}
}
