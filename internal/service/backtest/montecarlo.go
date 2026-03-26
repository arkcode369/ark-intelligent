package backtest

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// MonteCarloResult holds the aggregate output of a Monte Carlo bootstrap simulation.
type MonteCarloResult struct {
	NumSimulations    int     `json:"num_simulations"`
	MedianReturn      float64 `json:"median_return"`
	P5Return          float64 `json:"p5_return"`
	P95Return         float64 `json:"p95_return"`
	MedianMaxDD       float64 `json:"median_max_dd"`
	WorstCaseMaxDD    float64 `json:"worst_case_max_dd"` // 95th percentile of drawdowns
	ProbabilityOfLoss float64 `json:"probability_of_loss"`
	MedianSharpe      float64 `json:"median_sharpe"`
}

// MonteCarloSimulator runs bootstrap simulations on historical signal returns.
type MonteCarloSimulator struct {
	signalRepo ports.SignalRepository
}

// NewMonteCarloSimulator creates a new Monte Carlo simulator.
func NewMonteCarloSimulator(signalRepo ports.SignalRepository) *MonteCarloSimulator {
	return &MonteCarloSimulator{signalRepo: signalRepo}
}

// Simulate runs numSims bootstrap simulations over evaluated signal returns.
func (mc *MonteCarloSimulator) Simulate(ctx context.Context, numSims int) (*MonteCarloResult, error) {
	signals, err := mc.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals: %w", err)
	}

	// Collect Return1W values from evaluated signals.
	var returns []float64
	for _, s := range signals {
		if s.Outcome1W == domain.OutcomeWin || s.Outcome1W == domain.OutcomeLoss {
			returns = append(returns, s.Return1W)
		}
	}

	if len(returns) < 2 {
		return nil, fmt.Errorf("insufficient evaluated signals: %d", len(returns))
	}

	n := len(returns)
	simReturns := make([]float64, numSims)
	simMaxDDs := make([]float64, numSims)
	simSharpes := make([]float64, numSims)
	lossCount := 0

	// Use a local RNG seeded from current time for reproducibility control
	// and thread safety (global rand is not safe for concurrent use).
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < numSims; i++ {
		// Bootstrap: resample n returns with replacement.
		sampled := make([]float64, n)
		for j := 0; j < n; j++ {
			sampled[j] = returns[rng.Intn(n)]
		}

		// Cumulative return (geometric compounding from percentage returns).
		equity := 1.0
		peak := 1.0
		maxDD := 0.0
		sumRet := 0.0
		sumRetSq := 0.0

		for _, r := range sampled {
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
		simReturns[i] = cumReturn
		simMaxDDs[i] = maxDD * 100

		if cumReturn < 0 {
			lossCount++
		}

		// Sharpe: mean / stddev * sqrt(52) (weekly to annualized).
		meanRet := sumRet / float64(n)
		variance := sumRetSq/float64(n) - meanRet*meanRet
		if variance > 0 {
			simSharpes[i] = (meanRet / math.Sqrt(variance)) * math.Sqrt(52)
		}
	}

	sort.Float64s(simReturns)
	sort.Float64s(simMaxDDs)
	sort.Float64s(simSharpes)

	result := &MonteCarloResult{
		NumSimulations:    numSims,
		MedianReturn:      round2(percentile(simReturns, 50)),
		P5Return:          round2(percentile(simReturns, 5)),
		P95Return:         round2(percentile(simReturns, 95)),
		MedianMaxDD:       round2(percentile(simMaxDDs, 50)),
		WorstCaseMaxDD:    round2(percentile(simMaxDDs, 95)), // 95th percentile = worst 5% of drawdowns
		ProbabilityOfLoss: round2(float64(lossCount) / float64(numSims) * 100),
		MedianSharpe:      round2(percentile(simSharpes, 50)),
	}

	return result, nil
}

// percentile returns the p-th percentile (0-100) from a sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100 * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}
