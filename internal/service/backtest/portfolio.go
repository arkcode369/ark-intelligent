package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// PortfolioResult holds portfolio-level performance metrics computed from
// equal-weighted weekly signal returns.
type PortfolioResult struct {
	TotalWeeks        int       `json:"total_weeks"`
	ActiveWeeks       int       `json:"active_weeks"`
	AvgSignalsPerWeek float64   `json:"avg_signals_per_week"`
	WeeklyReturns     []float64 `json:"weekly_returns"`
	CumulativeReturn  float64   `json:"cumulative_return"`
	PortfolioSharpe   float64   `json:"portfolio_sharpe"`
	PortfolioMaxDD    float64   `json:"portfolio_max_dd"`
	CalmarRatio       float64   `json:"calmar_ratio"`
}

// PortfolioAnalyzer computes portfolio-level analytics from signal history.
type PortfolioAnalyzer struct {
	signalRepo ports.SignalRepository
}

// NewPortfolioAnalyzer creates a new portfolio analyzer.
func NewPortfolioAnalyzer(signalRepo ports.SignalRepository) *PortfolioAnalyzer {
	return &PortfolioAnalyzer{signalRepo: signalRepo}
}

// weekBucket accumulates signals within a single ISO week.
type weekBucket struct {
	key       string
	sumReturn float64
	count     int
}

// Analyze computes portfolio-level metrics from equal-weighted weekly returns.
func (pa *PortfolioAnalyzer) Analyze(ctx context.Context) (*PortfolioResult, error) {
	signals, err := pa.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals: %w", err)
	}

	// Group signals by ISO week.
	weekMap := make(map[string]*weekBucket)
	for _, s := range signals {
		if s.Outcome1W != domain.OutcomeWin && s.Outcome1W != domain.OutcomeLoss {
			continue
		}

		y, w := s.ReportDate.ISOWeek()
		key := fmt.Sprintf("%04d-W%02d", y, w)

		bucket, ok := weekMap[key]
		if !ok {
			bucket = &weekBucket{key: key}
			weekMap[key] = bucket
		}
		bucket.sumReturn += s.Return1W
		bucket.count++
	}

	if len(weekMap) == 0 {
		return &PortfolioResult{}, nil
	}

	// Sort weeks chronologically.
	weeks := make([]string, 0, len(weekMap))
	for k := range weekMap {
		weeks = append(weeks, k)
	}
	sort.Strings(weeks)

	// Compute equal-weighted weekly returns.
	weeklyReturns := make([]float64, 0, len(weeks))
	totalSignals := 0
	for _, k := range weeks {
		b := weekMap[k]
		weeklyReturns = append(weeklyReturns, b.sumReturn/float64(b.count))
		totalSignals += b.count
	}

	// Cumulative return (geometric).
	equity := 1.0
	peak := 1.0
	maxDD := 0.0
	sumRet := 0.0
	sumRetSq := 0.0

	for _, r := range weeklyReturns {
		equity *= 1 + r/100
		if equity > peak {
			peak = equity
		}
		dd := (peak - equity) / peak
		if dd > maxDD {
			maxDD = dd
		}
		sumRet += r
		sumRetSq += r * r
	}

	cumReturn := (equity - 1) * 100
	n := float64(len(weeklyReturns))

	// Sharpe ratio: annualized from weekly.
	meanRet := sumRet / n
	variance := sumRetSq/n - meanRet*meanRet
	sharpe := 0.0
	if variance > 0 {
		sharpe = (meanRet / math.Sqrt(variance)) * math.Sqrt(52)
	}

	// Calmar ratio: annualized return / max drawdown (both in %).
	// meanRet is in percentage points (e.g., 0.5 = 0.5%), so annualReturn is too.
	// maxDD is a decimal fraction (0 to 1), so convert to % for consistent units.
	annualReturn := meanRet * 52
	calmar := 0.0
	if maxDD > 0 {
		calmar = annualReturn / (maxDD * 100)
	}

	// Total weeks span: from first to last week in the dataset.
	totalWeeks := len(weeks)

	return &PortfolioResult{
		TotalWeeks:        totalWeeks,
		ActiveWeeks:       len(weeklyReturns),
		AvgSignalsPerWeek: round2(float64(totalSignals) / n),
		WeeklyReturns:     weeklyReturns,
		CumulativeReturn:  round2(cumReturn),
		PortfolioSharpe:   round2(sharpe),
		PortfolioMaxDD:    round2(maxDD * 100),
		CalmarRatio:       round2(calmar),
	}, nil
}
