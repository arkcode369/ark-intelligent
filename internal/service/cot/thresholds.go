// Package cot — centralized COT threshold constants.
// These are used across analyzer, confluence scoring, price divergence, and formatters.
package cot

// Directional bias thresholds (COT Index 0-100).
// > COTBullishThreshold = bullish bias, < COTBearishThreshold = bearish bias.
const (
	COTBullishThreshold = 60
	COTBearishThreshold = 40
)

// Extreme positioning thresholds (COT Index 0-100).
// These indicate crowded/extreme positions with reversal risk.
const (
	COTExtremeLong  = 75
	COTExtremeShort = 25
)
