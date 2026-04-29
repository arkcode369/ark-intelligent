package telegram

// formatter_ict.go — ICT result formatting for Telegram HTML messages.

import (
	"fmt"
	"strings"

	ictsvc "github.com/arkcode369/ark-intelligent/internal/service/ict"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)

// FormatICTResult formats an ICTResult as a Telegram HTML message.
// Output is kept under 3000 characters for mobile readability.
func FormatICTResult(r *ictsvc.ICTResult) string {
	var sb strings.Builder

	// Header — uses fmtutil.AnalysisHeader for consistency.
	sb.WriteString(fmtutil.AnalysisHeader("🔷", "ICT ANALYSIS", r.Symbol, r.Timeframe))
	sb.WriteString(fmt.Sprintf("📅 %s", r.AnalyzedAt.Format("2006-01-02 15:04 UTC")))
	if r.Killzone != "" {
		sb.WriteString(fmt.Sprintf("\n⏰ <b>Killzone:</b> %s", r.Killzone))
	}
	// Silver Bullet windows
	if len(r.SilverBullets) > 0 {
		activeSB := 0
		for _, sb2 := range r.SilverBullets {
			if sb2.Active {
				activeSB++
			}
		}
		if activeSB > 0 {
			sb.WriteString(fmt.Sprintf("\n🔫 <b>Silver Bullet:</b> %d window(s) ACTIVE", activeSB))
			for _, sb2 := range r.SilverBullets {
				if sb2.Active {
					sb.WriteString(fmt.Sprintf("\n  ⚡ %s (%02d:00–%02d:00 UTC)", sb2.Window, sb2.StartUTC, sb2.EndUTC))
					if sb2.FVGIndex >= 0 && sb2.FVGIndex < len(r.FVGZones) {
						fvg := r.FVGZones[sb2.FVGIndex]
						sb.WriteString(fmt.Sprintf(" → %s FVG at %.5f–%.5f", fvg.Type, fvg.Low, fvg.High))
					}
				}
			}
		} else {
			// Show next upcoming window
			for _, sb2 := range r.SilverBullets {
				sb.WriteString(fmt.Sprintf("\n  ○ %s (%02d:00–%02d:00 UTC)", sb2.Window, sb2.StartUTC, sb2.EndUTC))
			}
		}
	}
	sb.WriteString("\n\n")

	// Market Structure — uses fmtutil.BiasIcon.
	biasEmoji := fmtutil.BiasIcon(r.Bias)
	sb.WriteString(fmt.Sprintf("📐 <b>MARKET STRUCTURE:</b> %s %s\n", biasEmoji, r.Bias))
	structCount := 0
	for i := len(r.Structure) - 1; i >= 0 && structCount < 3; i-- {
		ev := r.Structure[i]
		icon := structureIcon(ev.Type, ev.Direction)
		sb.WriteString(fmt.Sprintf("  %s %s at %.5f\n", icon, ev.Type, ev.Level))
		structCount++
	}
	sb.WriteString("\n")

	// Premium/Discount Zone
	if r.Equilibrium > 0 {
		zoneLabel := "⚖️ EQUILIBRIUM"
		if r.PremiumZone {
			zoneLabel = "🔴 PREMIUM ZONE"
		} else if r.DiscountZone {
			zoneLabel = "🟢 DISCOUNT ZONE"
		}
		sb.WriteString(fmt.Sprintf("💰 <b>PRICE ZONE:</b> %s\n", zoneLabel))
		sb.WriteString(fmt.Sprintf("  Equilibrium: %.5f | Price: %.5f\n", r.Equilibrium, r.CurrentPrice))
		if r.PremiumZone {
			sb.WriteString("  ⚠️ Premium → look for shorts / sell setups\n")
		} else if r.DiscountZone {
			sb.WriteString("  ✅ Discount → look for longs / buy setups\n")
		}
		sb.WriteString("\n")
	}

	// Order Blocks
	if len(r.OrderBlocks) > 0 {
		sb.WriteString(fmt.Sprintf("📦 <b>ORDER BLOCKS (%d)</b>\n", len(r.OrderBlocks)))
		for _, ob := range r.OrderBlocks {
			status := "valid"
			suffix := ""
			icon := fmtutil.BiasIcon(ob.Type)
			if ob.Broken {
				status = "broken → Breaker"
				suffix = " ⚡"
			}
			sb.WriteString(fmt.Sprintf("  %s %s OB: %.5f–%.5f (%s%s)\n",
				icon, ob.Type, ob.Low, ob.High, status, suffix))
		}
		sb.WriteString("\n")
	}

	// Fair Value Gaps
	if len(r.FVGZones) > 0 {
		// Show only unfilled or partially filled FVGs (most recent 4).
		shown := 0
		fvgLines := make([]string, 0, 4)
		for i := len(r.FVGZones) - 1; i >= 0 && shown < 4; i-- {
			z := r.FVGZones[i]
			fillStr := fmt.Sprintf("%.0f%% filled", z.FillPct)
			if z.Filled {
				fillStr = "100% filled ✓"
			}
			arrow := fmtutil.DirectionIcon(z.Type)
			fvgLines = append(fvgLines, fmt.Sprintf("  %s %s FVG: %.5f–%.5f (%s)",
				arrow, z.Type, z.Low, z.High, fillStr))
			shown++
		}
		sb.WriteString(fmt.Sprintf("⬜ <b>FAIR VALUE GAPS (%d)</b>\n", len(r.FVGZones)))
		// Reverse so newest is first.
		for i := len(fvgLines) - 1; i >= 0; i-- {
			sb.WriteString(fvgLines[i] + "\n")
		}
		sb.WriteString("\n")
	}

	// Liquidity Sweeps
	if len(r.Sweeps) > 0 {
		sb.WriteString(fmt.Sprintf("💧 <b>LIQUIDITY SWEEPS (%d)</b>\n", len(r.Sweeps)))
		for _, s := range r.Sweeps {
			revStr := ""
			if s.Reversed {
				revStr = " → Reversed"
				if s.Type == "SWEEP_LOW" {
					revStr += " 📈 BULLISH"
				} else {
					revStr += " 📉 BEARISH"
				}
			}
			icon := "🔺"
			if s.Type == "SWEEP_LOW" {
				icon = "🔻"
			}
			sb.WriteString(fmt.Sprintf("  %s %s at %.5f%s\n", icon, s.Type, s.Level, revStr))
		}
		sb.WriteString("\n")
	}

	// Liquidity Pools (equal highs/lows clusters)
	if len(r.LiquidityLevels) > 0 {
		sb.WriteString(fmt.Sprintf("🎯 <b>LIQUIDITY POOLS (%d)</b>\n", len(r.LiquidityLevels)))
		for _, ll := range r.LiquidityLevels {
			statusIcon := "✅"
			if ll.Swept {
				statusIcon = "🧹"
			}
			dirIcon := "⬆️"
			if ll.Type == "SELL_SIDE" {
				dirIcon = "⬇️"
			}
			sb.WriteString(fmt.Sprintf("  %s %s %s: %.5f (%d touches) %s\n",
				statusIcon, dirIcon, ll.Type, ll.Price, ll.Count, map[bool]string{true: "SWEPT", false: "ACTIVE"}[ll.Swept]))
		}
		sb.WriteString("\n")
	}

	// Market Maker Models
	if len(r.MarketMakerModels) > 0 {
		sb.WriteString("🏦 <b>MARKET MAKER MODELS</b>\n")
		for _, mm := range r.MarketMakerModels {
			dirIcon := "📊"
			if mm.Direction == "BULLISH" {
				dirIcon = "📈"
			} else if mm.Direction == "BEARISH" {
				dirIcon = "📉"
			}
			sb.WriteString(fmt.Sprintf("  %s <b>%s</b>: %s %s (%.5f–%.5f)\n",
				dirIcon, mm.Model, mm.Phase, mm.Direction, mm.RangeLow, mm.RangeHigh))
		}
		sb.WriteString("\n")
	}

	// Judas Swings
	if len(r.JudasSwings) > 0 {
		sb.WriteString("🗡️ <b>JUDAS SWING</b>\n")
		for _, js := range r.JudasSwings {
			dirIcon := "📈"
			if js.Direction == "BEARISH" {
				dirIcon = "📉"
			}
			statusIcon := "✅"
			if !js.ReversalOK {
				statusIcon = "⏳"
			}
			sb.WriteString(fmt.Sprintf("  %s %s: %s %s (trap %.5f–%.5f, open %.5f)\n",
				statusIcon, js.Session, dirIcon, js.Direction, js.TrapLow, js.TrapHigh, js.OpenPrice))
		}
		sb.WriteString("\n")
	}

	// Power of 3 (AMD) phases
	if len(r.AMD) > 0 {
		sb.WriteString("🔄 <b>POWER OF 3 (AMD)</b>\n")
		for _, amd := range r.AMD {
			phaseIcon := map[string]string{
				"ACCUMULATION":  "📥",
				"MANIPULATION":  "🎭",
				"DISTRIBUTION": "📤",
				"UNKNOWN":       "❓",
			}[amd.Phase]
			dirIcon := ""
			if amd.Direction == "BULLISH" {
				dirIcon = " 📈"
			} else if amd.Direction == "BEARISH" {
				dirIcon = " 📉"
			}
			tfLabel := ""
			if amd.TF != "SESSION" {
				tfLabel = fmt.Sprintf("[%s] ", amd.TF)
			}
			sb.WriteString(fmt.Sprintf("  %s %s%s: %s%s (%.5f–%.5f)\n",
				phaseIcon, tfLabel, amd.Session, amd.Phase, dirIcon, amd.RangeLow, amd.RangeHigh))
		}
		sb.WriteString("\n")
	}

	// Killzone Boxes with pivots
	if len(r.KillzoneBoxes) > 0 {
		sb.WriteString("🕐 <b>KILLZONE BOXES</b>\n")
		for _, kz := range r.KillzoneBoxes {
			mitigated := ""
			if kz.Mitigated {
				mitigated = " ✗ mitigated"
			}
			sb.WriteString(fmt.Sprintf("  □ %s (%02d–%02d UTC): H %.5f L %.5f%s\n",
				kz.Name, kz.StartUTC, kz.EndUTC, kz.High, kz.Low, mitigated))
		}
		sb.WriteString("\n")
	}

	// DWM Pivots
	if len(r.DWMPivots) > 0 {
		sb.WriteString("📏 <b>DWM PIVOTS</b>\n")
		for _, p := range r.DWMPivots {
			broken := ""
			if p.Broken {
				broken = " ✗ broken"
			}
			sb.WriteString(fmt.Sprintf("  — %s: %.5f%s\n", p.Type, p.Level, broken))
		}
		sb.WriteString("\n")
	}

	// Relevant Anchors
	if r.RelevantHigh != nil || r.RelevantLow != nil {
		sb.WriteString("⚓ <b>RELEVANT ANCHORS</b>\n")
		if r.RelevantHigh != nil {
			sb.WriteString(fmt.Sprintf("  ⬆️ HIGH: %.5f (%.1fx ATR)\n", r.RelevantHigh.Level, r.RelevantHigh.ATRMultiple))
		}
		if r.RelevantLow != nil {
			sb.WriteString(fmt.Sprintf("  ⬇️ LOW: %.5f (%.1fx ATR)\n", r.RelevantLow.Level, r.RelevantLow.ATRMultiple))
		}
		sb.WriteString("\n")
	}

	// Optimal Trade Entry zones
	if len(r.OTE) > 0 {
		sb.WriteString(fmt.Sprintf("🎯 <b>OPTIMAL TRADE ENTRY (%d)</b>\n", len(r.OTE)))
		for _, ote := range r.OTE {
			icon := "🟢"
			if ote.Direction == "BEARISH" {
				icon = "🔴"
			}
			sb.WriteString(fmt.Sprintf("  %s %s OTE: %.5f–%.5f (mid %.5f)\n",
				icon, ote.Direction, ote.Low, ote.High, ote.Midpoint))
			// Check if current price is inside the OTE zone
			if r.CurrentPrice >= ote.Low && r.CurrentPrice <= ote.High {
				sb.WriteString("  ⚡ <b>PRICE IS IN OTE ZONE NOW!</b>\n")
			}
		}
		sb.WriteString("\n")
	}

	// Summary
	sb.WriteString(fmt.Sprintf("🎯 <b>SUMMARY:</b> %s\n", r.Summary))

	return truncateMsg(sb.String())
}

// structureIcon returns a status icon for a structure event.
func structureIcon(typ, direction string) string {
	switch {
	case typ == "CHOCH" && direction == "BULLISH":
		return "⚠️ "
	case typ == "CHOCH" && direction == "BEARISH":
		return "⚠️ "
	case typ == "BOS" && direction == "BULLISH":
		return "✅"
	case typ == "BOS" && direction == "BEARISH":
		return "❌"
	default:
		return "•"
	}
}
