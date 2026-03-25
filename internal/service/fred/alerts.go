package fred

import "fmt"

// AlertType represents the category of a FRED-triggered macro alert.
type AlertType string

const (
	AlertYieldUninvert  AlertType = "YIELD_UNINVERT"    // 2Y-10Y spread: negative → positive
	AlertYieldInvert    AlertType = "YIELD_INVERT"      // 2Y-10Y spread: positive → negative
	Alert3MUninvert     AlertType = "3M_YIELD_UNINVERT" // 3M-10Y spread: negative → positive
	Alert3MInvert       AlertType = "3M_YIELD_INVERT"   // 3M-10Y spread: positive → negative
	AlertNFCIStress     AlertType = "NFCI_STRESS"       // NFCI crosses above 0.5
	AlertNFCILoose      AlertType = "NFCI_LOOSE"        // NFCI crosses below -0.3
	AlertSahmTrigger    AlertType = "SAHM_TRIGGER"      // Sahm Rule crosses 0.5 (recession signal)
	AlertSahmClear      AlertType = "SAHM_CLEAR"        // Sahm Rule drops below 0.3
	AlertFedBalExpand   AlertType = "FED_BAL_EXPAND"    // Fed balance sheet expanding (QE signal)
	AlertFedBalContract AlertType = "FED_BAL_CONTRACT"  // Fed balance sheet contracting (QT)
	AlertVIXSpike       AlertType = "VIX_SPIKE"        // VIX crosses above 30
	AlertVIXCalm        AlertType = "VIX_CALM"         // VIX drops below 15
	AlertNFPNegative    AlertType = "NFP_NEGATIVE"     // NFP MoM change turns negative
)

// MacroAlert represents a single triggered macro regime change event.
type MacroAlert struct {
	Type        AlertType
	Title       string
	Description string
	Severity    string // "HIGH", "MEDIUM", "LOW"
	Value       float64
	Previous    float64
}

// CheckAlerts compares current MacroData against previous snapshot to detect regime changes.
// Returns all alerts that should be broadcast to users.
// Returns nil if either argument is nil (safe for initial startup).
func CheckAlerts(current, previous *MacroData) []MacroAlert {
	if current == nil || previous == nil {
		return nil
	}

	var alerts []MacroAlert

	// --- 1. Yield Curve 2Y-10Y inversion / uninversion ---
	if previous.YieldSpread < 0 && current.YieldSpread >= 0 {
		alerts = append(alerts, MacroAlert{
			Type: AlertYieldUninvert,
			Title: "🟢 Yield Curve UN-INVERTED (2Y-10Y)",
			Description: fmt.Sprintf(
				"2Y-10Y spread turned positive: %.2f%% (was %.2f%%). "+
					"Historically precedes risk-on rotation, but also signals the early recession phase. "+
					"Watch for USD weakness and gold strength.",
				current.YieldSpread, previous.YieldSpread),
			Severity: "HIGH",
			Value:    current.YieldSpread,
			Previous: previous.YieldSpread,
		})
	} else if previous.YieldSpread >= 0 && current.YieldSpread < 0 {
		alerts = append(alerts, MacroAlert{
			Type: AlertYieldInvert,
			Title: "🔴 Yield Curve INVERTED (2Y-10Y)",
			Description: fmt.Sprintf(
				"2Y-10Y spread went negative: %.2f%% (was %.2f%%). "+
					"Classic recession leading indicator with ~12-24 month lag. "+
					"USD may strengthen initially; monitor for Fed pivot signals.",
				current.YieldSpread, previous.YieldSpread),
			Severity: "HIGH",
			Value:    current.YieldSpread,
			Previous: previous.YieldSpread,
		})
	}

	// --- 2. Yield Curve 3M-10Y (stronger recession predictor per Fed research) ---
	if current.Spread3M10Y != 0 && previous.Spread3M10Y != 0 {
		if previous.Spread3M10Y < 0 && current.Spread3M10Y >= 0 {
			alerts = append(alerts, MacroAlert{
				Type: Alert3MUninvert,
				Title: "🟡 3M-10Y Spread UN-INVERTED",
				Description: fmt.Sprintf(
					"3M-10Y spread turned positive: %.2f%%. "+
						"Per NY Fed model, this uninversion often precedes recession within 6-18 months. "+
						"Heightened vigilance recommended.",
					current.Spread3M10Y),
				Severity: "HIGH",
				Value:    current.Spread3M10Y,
				Previous: previous.Spread3M10Y,
			})
		} else if previous.Spread3M10Y >= 0 && current.Spread3M10Y < 0 {
			alerts = append(alerts, MacroAlert{
				Type: Alert3MInvert,
				Title: "🔴 3M-10Y Spread INVERTED",
				Description: fmt.Sprintf(
					"3M-10Y spread turned negative: %.2f%%. "+
						"This is the NY Fed's preferred recession predictor — accuracy historically >85%%. "+
						"Defensive positioning recommended.",
					current.Spread3M10Y),
				Severity: "HIGH",
				Value:    current.Spread3M10Y,
				Previous: previous.Spread3M10Y,
			})
		}
	}

	// --- 3. NFCI financial conditions threshold crossings ---
	if previous.NFCI < 0.5 && current.NFCI >= 0.5 {
		alerts = append(alerts, MacroAlert{
			Type: AlertNFCIStress,
			Title: "🔴 NFCI: Financial Conditions TIGHT",
			Description: fmt.Sprintf(
				"NFCI crossed 0.5 threshold: %.3f (was %.3f). "+
					"Risk-off environment — reduce exposure to high-beta FX and EM. "+
					"Safe havens (JPY, CHF, Gold) favored.",
				current.NFCI, previous.NFCI),
			Severity: "HIGH",
			Value:    current.NFCI,
			Previous: previous.NFCI,
		})
	} else if previous.NFCI >= -0.3 && current.NFCI < -0.3 {
		alerts = append(alerts, MacroAlert{
			Type: AlertNFCILoose,
			Title: "🟢 NFCI: Financial Conditions LOOSE",
			Description: fmt.Sprintf(
				"NFCI dropped below -0.3: %.3f (was %.3f). "+
					"Risk-on conditions improving — AUD, NZD, CAD and risk FX favored.",
				current.NFCI, previous.NFCI),
			Severity: "MEDIUM",
			Value:    current.NFCI,
			Previous: previous.NFCI,
		})
	}

	// --- 4. Sahm Rule recession indicator ---
	if previous.SahmRule < 0.5 && current.SahmRule >= 0.5 {
		alerts = append(alerts, MacroAlert{
			Type: AlertSahmTrigger,
			Title: "🚨 SAHM RULE TRIGGERED — Recession Signal!",
			Description: fmt.Sprintf(
				"Sahm Rule indicator crossed 0.5: %.2f (was %.2f). "+
					"This indicator has triggered before every US recession since 1970 with zero false positives. "+
					"Defensive positioning STRONGLY recommended. Gold, JPY, CHF.",
				current.SahmRule, previous.SahmRule),
			Severity: "HIGH",
			Value:    current.SahmRule,
			Previous: previous.SahmRule,
		})
	} else if previous.SahmRule >= 0.5 && current.SahmRule < 0.3 {
		alerts = append(alerts, MacroAlert{
			Type: AlertSahmClear,
			Title: "🟢 SAHM RULE CLEARED",
			Description: fmt.Sprintf(
				"Sahm Rule dropped below 0.3: %.2f (was %.2f). "+
					"Recession risk receding — risk appetite may gradually recover. "+
					"Monitor for confirmation before adding risk.",
				current.SahmRule, previous.SahmRule),
			Severity: "MEDIUM",
			Value:    current.SahmRule,
			Previous: previous.SahmRule,
		})
	}

	// --- 5. Fed Balance Sheet direction (QE vs QT signal) ---
	// Use $200B as a significant threshold for weekly WALCL changes
	if current.FedBalSheet > 0 && previous.FedBalSheet > 0 {
		balChange := current.FedBalSheet - previous.FedBalSheet
		prevDir := previous.FedBalSheetTrend.Direction
		if prevDir != "UP" && balChange > 200 {
			alerts = append(alerts, MacroAlert{
				Type: AlertFedBalExpand,
				Title: "🟢 Fed Balance Sheet EXPANDING (QE Signal)",
				Description: fmt.Sprintf(
					"Fed total assets increased by $%.0fB to $%.0fB. "+
						"Potential liquidity injection — bullish for gold, risk FX, and equities.",
					balChange, current.FedBalSheet),
				Severity: "MEDIUM",
				Value:    current.FedBalSheet,
				Previous: previous.FedBalSheet,
			})
		} else if prevDir != "DOWN" && balChange < -200 {
			alerts = append(alerts, MacroAlert{
				Type: AlertFedBalContract,
				Title: "🔴 Fed Balance Sheet CONTRACTING (QT Active)",
				Description: fmt.Sprintf(
					"Fed total assets decreased by $%.0fB to $%.0fB. "+
						"Quantitative tightening accelerating — USD supportive, risk assets under pressure.",
					-balChange, current.FedBalSheet),
				Severity: "MEDIUM",
				Value:    current.FedBalSheet,
				Previous: previous.FedBalSheet,
			})
		}
	}

	// --- 6. VIX spike / calm ---
	if current.VIX > 0 && previous.VIX > 0 {
		if previous.VIX < 30 && current.VIX >= 30 {
			alerts = append(alerts, MacroAlert{
				Type:  AlertVIXSpike,
				Title: "🔴 VIX SPIKE — Risk-Off Mode",
				Description: fmt.Sprintf(
					"VIX crossed 30: %.1f (was %.1f). "+
						"Market fear elevated — JPY/CHF/Gold favored, risk FX under pressure. "+
						"Historically, VIX >30 correlates with USDJPY downside.",
					current.VIX, previous.VIX),
				Severity: "HIGH",
				Value:    current.VIX,
				Previous: previous.VIX,
			})
		} else if previous.VIX >= 15 && current.VIX < 15 {
			alerts = append(alerts, MacroAlert{
				Type:  AlertVIXCalm,
				Title: "🟢 VIX CALM — Risk Appetite Returning",
				Description: fmt.Sprintf(
					"VIX dropped below 15: %.1f (was %.1f). "+
						"Low volatility environment — risk-on FX (AUD, NZD, CAD) may benefit.",
					current.VIX, previous.VIX),
				Severity: "MEDIUM",
				Value:    current.VIX,
				Previous: previous.VIX,
			})
		}
	}

	// --- 7. NFP negative ---
	if current.NFPChange != 0 && previous.NFPChange != 0 {
		if previous.NFPChange > 0 && current.NFPChange < 0 {
			alerts = append(alerts, MacroAlert{
				Type:  AlertNFPNegative,
				Title: "🚨 NFP NEGATIVE — Job Losses!",
				Description: fmt.Sprintf(
					"Nonfarm Payrolls turned negative: %.0fK (was +%.0fK). "+
						"Actual job losses are extremely rare and signal severe economic deterioration. "+
						"Fed dovish pivot likely — USD bearish, Gold bullish.",
					current.NFPChange, previous.NFPChange),
				Severity: "HIGH",
				Value:    current.NFPChange,
				Previous: previous.NFPChange,
			})
		}
	}

	return alerts
}

// FormatMacroAlert formats a MacroAlert as Telegram HTML for broadcast.
func FormatMacroAlert(alert MacroAlert) string {
	severityIcon := "ℹ️"
	switch alert.Severity {
	case "HIGH":
		severityIcon = "🚨"
	case "MEDIUM":
		severityIcon = "⚠️"
	}

	return fmt.Sprintf(
		"%s\n\n<i>%s</i>\n\n<code>Current: %.3f | Previous: %.3f</code>\n\n%s <i>Severity: %s</i>\n<i>Source: St. Louis FRED | </i><code>/macro</code><i> for full dashboard</i>",
		alert.Title,
		alert.Description,
		alert.Value,
		alert.Previous,
		severityIcon,
		alert.Severity,
	)
}
