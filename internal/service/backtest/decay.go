package backtest

import (
	"context"
	"fmt"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// DecayPoint captures signal performance at a specific number of days after issuance.
type DecayPoint struct {
	Day        int     `json:"day"`
	WinRate    float64 `json:"win_rate"`
	AvgReturn  float64 `json:"avg_return"`
	SampleSize int     `json:"sample_size"`
}

// DecayResult holds the full decay curve for a single signal type.
type DecayResult struct {
	SignalType  string       `json:"signal_type"`
	Points      []DecayPoint `json:"points"`
	PeakDay     int          `json:"peak_day"`
	PeakWinRate float64      `json:"peak_win_rate"`
	HalfLifeDay int          `json:"half_life_day"`
}

// DecayAnalyzer evaluates how signal edge decays over time.
type DecayAnalyzer struct {
	signalRepo ports.SignalRepository
	dailyRepo  FlexDailyProvider
}

// NewDecayAnalyzer creates a new decay analyzer.
func NewDecayAnalyzer(signalRepo ports.SignalRepository, dailyRepo FlexDailyProvider) *DecayAnalyzer {
	return &DecayAnalyzer{signalRepo: signalRepo, dailyRepo: dailyRepo}
}

// decayDays defines the evaluation horizon checkpoints.
var decayDays = []int{1, 2, 3, 4, 5, 7, 10, 14, 21, 28}

// Analyze computes decay curves for each signal type.
func (da *DecayAnalyzer) Analyze(ctx context.Context) (map[string]*DecayResult, error) {
	signals, err := da.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals: %w", err)
	}

	// Filter to evaluated signals only (Outcome1W is WIN or LOSS).
	var evaluated []domain.PersistedSignal
	for _, s := range signals {
		if s.Outcome1W == domain.OutcomeWin || s.Outcome1W == domain.OutcomeLoss {
			evaluated = append(evaluated, s)
		}
	}

	if len(evaluated) == 0 {
		return nil, nil
	}

	// Group by signal type.
	grouped := make(map[string][]domain.PersistedSignal)
	for _, s := range evaluated {
		grouped[s.SignalType] = append(grouped[s.SignalType], s)
	}

	results := make(map[string]*DecayResult, len(grouped))
	for sigType, sigs := range grouped {
		result, err := da.analyzeGroup(ctx, sigType, sigs)
		if err != nil {
			log.Warn().Err(err).Str("signal_type", sigType).Msg("decay analysis failed for group")
			continue
		}
		results[sigType] = result
	}

	return results, nil
}

func (da *DecayAnalyzer) analyzeGroup(ctx context.Context, sigType string, sigs []domain.PersistedSignal) (*DecayResult, error) {
	result := &DecayResult{SignalType: sigType}

	for _, day := range decayDays {
		var wins, total int
		var sumReturn float64

		for _, s := range sigs {
			if s.EntryPrice == 0 {
				continue
			}

			// Look up price at report_date + day, with ±3 day tolerance for weekends/holidays.
			// Clamp 'from' to at least report_date+1 to never look at pre-signal prices.
			from := s.ReportDate.AddDate(0, 0, day-3)
			earliest := s.ReportDate.AddDate(0, 0, 1) // day after report
			if from.Before(earliest) {
				from = earliest
			}
			to := s.ReportDate.AddDate(0, 0, day+3)
			prices, err := da.dailyRepo.GetDailyRange(ctx, s.ContractCode, from, to)
			if err != nil || len(prices) == 0 {
				continue
			}

			// Pick the price closest to the target day.
			targetDate := s.ReportDate.AddDate(0, 0, day)
			closest := prices[0]
			bestDist := absDuration(prices[0].Date.Sub(targetDate))
			for _, p := range prices[1:] {
				dist := absDuration(p.Date.Sub(targetDate))
				if dist < bestDist {
					bestDist = dist
					closest = p
				}
			}
			ret := computeReturn(s.EntryPrice, closest.Close, s.Inverse)
			outcome := classifyOutcome(s.Direction, ret)

			total++
			sumReturn += ret
			if outcome == domain.OutcomeWin {
				wins++
			}
		}

		if total == 0 {
			continue
		}

		point := DecayPoint{
			Day:        day,
			WinRate:    round2(float64(wins) / float64(total) * 100),
			AvgReturn:  round4(sumReturn / float64(total)),
			SampleSize: total,
		}
		result.Points = append(result.Points, point)
	}

	// Find peak day (highest win rate).
	for _, p := range result.Points {
		if p.WinRate > result.PeakWinRate {
			result.PeakWinRate = p.WinRate
			result.PeakDay = p.Day
		}
	}

	// Find half-life day: first day where win rate drops below the half-life threshold.
	// Threshold = 50 + (peakWR - 50) / 2
	if result.PeakWinRate > 50 {
		threshold := 50 + (result.PeakWinRate-50)/2
		pastPeak := false
		for _, p := range result.Points {
			if p.Day == result.PeakDay {
				pastPeak = true
				continue
			}
			if pastPeak && p.WinRate < threshold {
				result.HalfLifeDay = p.Day
				break
			}
		}
	}

	return result, nil
}

// absDuration returns the absolute value of a time.Duration.
func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
