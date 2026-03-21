package backtest

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("backtest")

// Evaluator fills in outcome fields on persisted signals by looking up
// future prices at +1W, +2W, and +4W from the signal's report date.
type Evaluator struct {
	signalRepo ports.SignalRepository
	priceRepo  ports.PriceRepository
}

// NewEvaluator creates a new signal outcome evaluator.
func NewEvaluator(signalRepo ports.SignalRepository, priceRepo ports.PriceRepository) *Evaluator {
	return &Evaluator{
		signalRepo: signalRepo,
		priceRepo:  priceRepo,
	}
}

// EvaluatePending finds all signals that need outcome evaluation and fills
// in price/return/outcome fields. Returns the number of signals evaluated.
func (e *Evaluator) EvaluatePending(ctx context.Context) (int, error) {
	pending, err := e.signalRepo.GetPendingSignals(ctx)
	if err != nil {
		return 0, fmt.Errorf("get pending signals: %w", err)
	}

	if len(pending) == 0 {
		return 0, nil
	}

	log.Info().Int("pending", len(pending)).Msg("Evaluating pending signals")

	evaluated := 0
	for i := range pending {
		updated, err := e.evaluateSignal(ctx, &pending[i])
		if err != nil {
			log.Warn().Err(err).
				Str("contract", pending[i].ContractCode).
				Str("type", pending[i].SignalType).
				Msg("Failed to evaluate signal")
			continue
		}
		if !updated {
			continue
		}

		if err := e.signalRepo.UpdateSignal(ctx, pending[i]); err != nil {
			log.Warn().Err(err).Msg("Failed to update evaluated signal")
			continue
		}
		evaluated++
	}

	log.Info().Int("evaluated", evaluated).Int("pending", len(pending)).Msg("Signal evaluation complete")
	return evaluated, nil
}

// evaluateSignal looks up future prices and fills outcome fields.
// Returns true if any field was updated.
func (e *Evaluator) evaluateSignal(ctx context.Context, sig *domain.PersistedSignal) (bool, error) {
	if sig.EntryPrice == 0 {
		return false, nil // Cannot evaluate without entry price
	}

	now := time.Now()
	updated := false

	// Evaluate 1-week outcome
	if (sig.Outcome1W == "" || sig.Outcome1W == domain.OutcomePending) &&
		now.Sub(sig.ReportDate) >= 7*24*time.Hour {
		targetDate := sig.ReportDate.AddDate(0, 0, 7)
		price, err := e.priceRepo.GetPriceAt(ctx, sig.ContractCode, targetDate)
		if err != nil {
			return false, fmt.Errorf("get price at +1W: %w", err)
		}
		if price != nil && price.Close > 0 {
			sig.Price1W = price.Close
			sig.Return1W = computeReturn(sig.EntryPrice, price.Close, sig.Inverse)
			sig.Outcome1W = classifyOutcome(sig.Direction, sig.Return1W)
			updated = true
		}
	}

	// Evaluate 2-week outcome
	if (sig.Outcome2W == "" || sig.Outcome2W == domain.OutcomePending) &&
		now.Sub(sig.ReportDate) >= 14*24*time.Hour {
		targetDate := sig.ReportDate.AddDate(0, 0, 14)
		price, err := e.priceRepo.GetPriceAt(ctx, sig.ContractCode, targetDate)
		if err != nil {
			return false, fmt.Errorf("get price at +2W: %w", err)
		}
		if price != nil && price.Close > 0 {
			sig.Price2W = price.Close
			sig.Return2W = computeReturn(sig.EntryPrice, price.Close, sig.Inverse)
			sig.Outcome2W = classifyOutcome(sig.Direction, sig.Return2W)
			updated = true
		}
	}

	// Evaluate 4-week outcome
	if (sig.Outcome4W == "" || sig.Outcome4W == domain.OutcomePending) &&
		now.Sub(sig.ReportDate) >= 28*24*time.Hour {
		targetDate := sig.ReportDate.AddDate(0, 0, 28)
		price, err := e.priceRepo.GetPriceAt(ctx, sig.ContractCode, targetDate)
		if err != nil {
			return false, fmt.Errorf("get price at +4W: %w", err)
		}
		if price != nil && price.Close > 0 {
			sig.Price4W = price.Close
			sig.Return4W = computeReturn(sig.EntryPrice, price.Close, sig.Inverse)
			sig.Outcome4W = classifyOutcome(sig.Direction, sig.Return4W)
			updated = true
		}
	}

	if updated {
		sig.EvaluatedAt = now
	}

	return updated, nil
}

// computeReturn calculates the percentage return from entry to exit price.
// For inverse pairs (USD/JPY, USD/CHF, USD/CAD, DXY), a price increase
// means the base currency (USD) strengthened, which is bearish for the
// foreign currency — so the return is negated.
func computeReturn(entryPrice, exitPrice float64, inverse bool) float64 {
	if entryPrice == 0 {
		return 0
	}
	ret := ((exitPrice - entryPrice) / entryPrice) * 100
	if inverse {
		ret = -ret
	}
	// Round to 4 decimal places
	return math.Round(ret*10000) / 10000
}

// classifyOutcome determines WIN or LOSS based on direction and return.
// A BULLISH signal wins if return > 0, BEARISH wins if return < 0.
func classifyOutcome(direction string, returnPct float64) string {
	switch direction {
	case "BULLISH":
		if returnPct > 0 {
			return domain.OutcomeWin
		}
		return domain.OutcomeLoss
	case "BEARISH":
		if returnPct < 0 {
			return domain.OutcomeWin
		}
		return domain.OutcomeLoss
	default:
		return domain.OutcomePending
	}
}
