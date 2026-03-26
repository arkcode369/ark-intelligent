package backtest

import (
	"fmt"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// DedupResult holds statistics comparing raw vs deduplicated signal sets.
type DedupResult struct {
	RawSignalCount   int     `json:"raw_signal_count"`
	DedupSignalCount int     `json:"dedup_signal_count"`
	OverlapRate      float64 `json:"overlap_rate"`
	DedupWinRate1W   float64 `json:"dedup_win_rate_1w"`
	DedupAvgReturn1W float64 `json:"dedup_avg_return_1w"`
}

// DeduplicateSignals removes duplicate signals that target the same contract
// in the same ISO week. For each (ContractCode, ISO-week) group, the signal
// with the highest Strength is kept; ties are broken by highest Confidence.
func DeduplicateSignals(signals []domain.PersistedSignal) []domain.PersistedSignal {
	type groupKey struct {
		ContractCode string
		ISOWeek      string
	}

	best := make(map[groupKey]domain.PersistedSignal)

	for _, s := range signals {
		y, w := s.ReportDate.ISOWeek()
		key := groupKey{
			ContractCode: s.ContractCode,
			ISOWeek:      fmt.Sprintf("%04d-W%02d", y, w),
		}

		existing, ok := best[key]
		if !ok {
			best[key] = s
			continue
		}

		// Pick higher Strength; break ties by higher Confidence.
		if s.Strength > existing.Strength ||
			(s.Strength == existing.Strength && s.Confidence > existing.Confidence) {
			best[key] = s
		}
	}

	result := make([]domain.PersistedSignal, 0, len(best))
	for _, s := range best {
		result = append(result, s)
	}
	return result
}

// ComputeDedupStats computes statistics on the raw and deduplicated signal sets,
// including overlap rate, win rate, and average return at the 1W horizon.
func ComputeDedupStats(signals []domain.PersistedSignal) *DedupResult {
	deduped := DeduplicateSignals(signals)

	rawCount := len(signals)
	dedupCount := len(deduped)

	var overlapRate float64
	if rawCount > 0 {
		overlapRate = round2(float64(rawCount-dedupCount) / float64(rawCount) * 100)
	}

	// Compute win rate and avg return at 1W for deduped set (evaluated signals only).
	var wins, eval int
	var sumReturn float64
	for _, s := range deduped {
		if s.Outcome1W == domain.OutcomeWin || s.Outcome1W == domain.OutcomeLoss {
			eval++
			sumReturn += s.Return1W
			if s.Outcome1W == domain.OutcomeWin {
				wins++
			}
		}
	}

	var winRate, avgReturn float64
	if eval > 0 {
		winRate = round2(float64(wins) / float64(eval) * 100)
		avgReturn = round4(sumReturn / float64(eval))
	}

	return &DedupResult{
		RawSignalCount:   rawCount,
		DedupSignalCount: dedupCount,
		OverlapRate:      overlapRate,
		DedupWinRate1W:   winRate,
		DedupAvgReturn1W: avgReturn,
	}
}
