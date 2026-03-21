package price

import (
	"context"
	"fmt"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// RiskContextBuilder computes VIX + S&P 500 risk sentiment context from stored prices.
type RiskContextBuilder struct {
	priceRepo ports.PriceRepository
}

// NewRiskContextBuilder creates a new risk context builder.
func NewRiskContextBuilder(priceRepo ports.PriceRepository) *RiskContextBuilder {
	return &RiskContextBuilder{priceRepo: priceRepo}
}

// Build computes the current risk context from stored VIX and SPX price records.
// Returns nil (not an error) if price data is not yet available — caller should treat
// nil RiskContext as "no adjustment" (multiplier = 1.0).
func (rb *RiskContextBuilder) Build(ctx context.Context) (*domain.RiskContext, error) {
	vixRecords, err := rb.priceRepo.GetHistory(ctx, "risk_VIX", 4)
	if err != nil || len(vixRecords) == 0 {
		return nil, fmt.Errorf("VIX price data unavailable: %w", err)
	}

	spxRecords, err := rb.priceRepo.GetHistory(ctx, "risk_SPX", 5)
	if err != nil || len(spxRecords) == 0 {
		return nil, fmt.Errorf("SPX price data unavailable: %w", err)
	}

	rc := &domain.RiskContext{}

	// --- VIX ---
	rc.VIXLevel = vixRecords[0].Close

	// 4-week VIX average
	var sumVIX float64
	for _, r := range vixRecords {
		sumVIX += r.Close
	}
	rc.VIX4WAvg = roundN(sumVIX/float64(len(vixRecords)), 2)

	// VIX trend: compare current vs 4W avg
	if rc.VIXLevel > rc.VIX4WAvg*1.05 {
		rc.VIXTrend = "RISING"
	} else if rc.VIXLevel < rc.VIX4WAvg*0.95 {
		rc.VIXTrend = "FALLING"
	} else {
		rc.VIXTrend = "STABLE"
	}

	rc.Regime = domain.ClassifyRiskRegime(rc.VIXLevel)

	// --- SPX ---
	rc.SPXWeeklyChg = 0
	if len(spxRecords) >= 2 && spxRecords[1].Close > 0 {
		rc.SPXWeeklyChg = roundN((spxRecords[0].Close-spxRecords[1].Close)/spxRecords[1].Close*100, 4)
	}

	rc.SPXMonthlyChg = 0
	if len(spxRecords) >= 5 && spxRecords[4].Close > 0 {
		rc.SPXMonthlyChg = roundN((spxRecords[0].Close-spxRecords[4].Close)/spxRecords[4].Close*100, 4)
	}

	// SPX above 4W MA
	if len(spxRecords) >= 4 {
		var sumSPX float64
		for i := 0; i < 4; i++ {
			sumSPX += spxRecords[i].Close
		}
		ma4w := sumSPX / 4
		rc.SPXAboveMA4W = spxRecords[0].Close > ma4w
	}

	return rc, nil
}
