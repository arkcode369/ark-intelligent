package orderflow

import (
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// pointOfControl returns the price level (OHLCV Close) with the highest aggregated volume
// across all bars. Volume is attributed to the close price of each bar, which approximates
// the price level where most activity occurred.
func pointOfControl(bars []ta.OHLCV) float64 {
	if len(bars) == 0 {
		return 0
	}

	// Bucket volume by price rounded to 4 significant decimal places.
	// We use a map[float64]float64 for simplicity.
	buckets := make(map[float64]float64, len(bars))
	for _, b := range bars {
		// Round to midpoint of High-Low range for cleaner bucketing.
		mid := (b.High + b.Low) / 2
		// Round to 5 decimal places (forex precision).
		rounded := roundPrice(mid, b.High-b.Low)
		buckets[rounded] += b.Volume
	}

	var maxVol float64
	var poc float64
	for price, vol := range buckets {
		if vol > maxVol {
			maxVol = vol
			poc = price
		}
	}
	return poc
}

// roundPrice rounds a price to an appropriate tick size based on the bar's HL range.
// For tight ranges (e.g. forex: 0.001 spread), we round to 4 decimal places;
// for wider ranges (e.g. indices, crypto), we round to 2.
func roundPrice(price, rangeHL float64) float64 {
	if price == 0 {
		return 0
	}

	var factor float64
	switch {
	case rangeHL < 0.01:
		factor = 1e5 // 5 decimals (forex)
	case rangeHL < 1:
		factor = 1e4 // 4 decimals
	case rangeHL < 10:
		factor = 1e2 // 2 decimals
	default:
		factor = 1e1 // 1 decimal (large asset: gold, crypto)
	}

	return float64(int64(price*factor+0.5)) / factor
}
