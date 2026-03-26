package backtest

import (
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// Risk of Ruin — institutional risk management metric
// ---------------------------------------------------------------------------

// RiskOfRuinResult holds risk-of-ruin analysis output.
type RiskOfRuinResult struct {
	WinRate          float64 `json:"win_rate"`           // Win rate used (0-100%)
	AvgWin           float64 `json:"avg_win"`            // Average winning return (%)
	AvgLoss          float64 `json:"avg_loss"`           // Average losing return (%, negative)
	KellyFraction    float64 `json:"kelly_fraction"`     // Full Kelly fraction
	RuinProb10Pct    float64 `json:"ruin_prob_10pct"`    // Probability of 10% drawdown
	RuinProb25Pct    float64 `json:"ruin_prob_25pct"`    // Probability of 25% drawdown
	RuinProb50Pct    float64 `json:"ruin_prob_50pct"`    // Probability of 50% drawdown
	SafePositionSize float64 `json:"safe_position_size"` // Max size for <5% ruin at 25% level
}

// ComputeRiskOfRuin calculates the classic risk-of-ruin probability.
//
// Formula: RoR = (q/p)^units where:
//   - p = winRate / 100, q = 1 - p
//   - edge = (p * avgWin - q * |avgLoss|) / avgWin
//   - units = ruinLevelPct / |avgLoss|
//
// If edge <= 0, ruin is certain (returns 1.0).
func ComputeRiskOfRuin(winRate, avgWin, avgLoss float64, ruinLevelPct float64) float64 {
	if avgWin <= 0 || avgLoss >= 0 || ruinLevelPct <= 0 {
		return 1.0
	}

	p := winRate / 100.0
	q := 1.0 - p

	if p <= 0 || p >= 1 {
		if p <= 0 {
			return 1.0
		}
		return 0.0
	}

	absLoss := math.Abs(avgLoss)
	edge := (p*avgWin - q*absLoss) / avgWin
	if edge <= 0 {
		return 1.0
	}

	units := ruinLevelPct / absLoss
	ror := math.Pow(q/p, units)
	if ror > 1.0 {
		return 1.0
	}
	return ror
}

// ComputeRiskOfRuinFromSignals computes full risk-of-ruin analysis from
// evaluated signals. Uses 1W outcomes to derive win rate, average win/loss,
// then calculates ruin probabilities at 10%, 25%, and 50% drawdown levels.
func ComputeRiskOfRuinFromSignals(signals []domain.PersistedSignal) *RiskOfRuinResult {
	var wins, losses int
	var sumWinReturn, sumLossReturn float64

	for i := range signals {
		s := &signals[i]
		if s.Outcome1W == "" || s.Outcome1W == domain.OutcomePending || s.Outcome1W == domain.OutcomeExpired {
			continue
		}
		if s.Outcome1W == domain.OutcomeWin {
			wins++
			sumWinReturn += math.Abs(s.Return1W)
		} else if s.Outcome1W == domain.OutcomeLoss {
			losses++
			sumLossReturn += math.Abs(s.Return1W)
		}
	}

	total := wins + losses
	if total == 0 || wins == 0 || losses == 0 {
		return &RiskOfRuinResult{
			RuinProb10Pct: 1.0,
			RuinProb25Pct: 1.0,
			RuinProb50Pct: 1.0,
		}
	}

	winRate := float64(wins) / float64(total) * 100.0
	avgWin := sumWinReturn / float64(wins)
	avgLoss := -(sumLossReturn / float64(losses)) // negative by convention

	// Kelly fraction: f* = p - q / (avgWin / |avgLoss|)
	p := winRate / 100.0
	q := 1.0 - p
	winLossRatio := avgWin / math.Abs(avgLoss)
	kelly := p - q/winLossRatio

	result := &RiskOfRuinResult{
		WinRate:       round2(winRate),
		AvgWin:        round4(avgWin),
		AvgLoss:       round4(avgLoss),
		KellyFraction: round4(kelly),
		RuinProb10Pct: round4(ComputeRiskOfRuin(winRate, avgWin, avgLoss, 10.0)),
		RuinProb25Pct: round4(ComputeRiskOfRuin(winRate, avgWin, avgLoss, 25.0)),
		RuinProb50Pct: round4(ComputeRiskOfRuin(winRate, avgWin, avgLoss, 50.0)),
	}

	// Binary search for safe position size: max Kelly fraction where
	// ruin probability at 25% drawdown level stays below 5%.
	result.SafePositionSize = round4(findSafePositionSize(winRate, avgWin, avgLoss))

	return result
}

// findSafePositionSize uses binary search to find the maximum position size
// (as a fraction of Kelly) where the probability of a 25% drawdown is < 5%.
func findSafePositionSize(winRate, avgWin, avgLoss float64) float64 {
	lo, hi := 0.0, 1.0

	for iter := 0; iter < 100; iter++ {
		mid := (lo + hi) / 2.0
		// Scale avg win/loss by position fraction to simulate sizing.
		scaledWin := avgWin * mid
		scaledLoss := avgLoss * mid
		ror := ComputeRiskOfRuin(winRate, scaledWin, scaledLoss, 25.0)
		if ror < 0.05 {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}
