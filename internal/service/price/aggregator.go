package price

import (
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// SupportedIntradayIntervals lists all intervals the system stores.
var SupportedIntradayIntervals = []string{"15m", "30m", "1h", "4h", "6h", "12h"}

// AggregateFromBase takes 15m bars and produces aggregated bars for larger timeframes.
// Returns a map of interval -> []IntradayBar.
func AggregateFromBase(baseBars []domain.IntradayBar) map[string][]domain.IntradayBar {
	result := make(map[string][]domain.IntradayBar)

	// 15m bars are the base — include as-is
	result["15m"] = baseBars

	// Aggregate to each higher timeframe
	for _, interval := range []struct {
		name    string
		minutes int
	}{
		{"30m", 30},
		{"1h", 60},
		{"4h", 240},
		{"6h", 360},
		{"12h", 720},
	} {
		result[interval.name] = aggregateBars(baseBars, interval.name, interval.minutes)
	}

	return result
}

// aggregateBars groups 15m bars into larger buckets.
// Bars are sorted chronologically first so Open = first bar, Close = last bar.
func aggregateBars(bars []domain.IntradayBar, targetInterval string, minutes int) []domain.IntradayBar {
	if len(bars) == 0 {
		return nil
	}

	// Sort chronologically (oldest first) to ensure correct Open/Close assignment.
	sorted := make([]domain.IntradayBar, len(bars))
	copy(sorted, bars)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	type bucket struct {
		open     float64
		high     float64
		low      float64
		closeVal float64
		volume   float64
		ts       time.Time
		count    int
	}

	buckets := make(map[string]*bucket)

	for _, bar := range sorted {
		// Calculate bucket start time
		bucketStart := alignToBucket(bar.Timestamp, minutes)
		key := bucketStart.Format("200601021504")

		b, ok := buckets[key]
		if !ok {
			// First bar in bucket (chronologically earliest due to sort)
			b = &bucket{
				open: bar.Open,
				high: bar.High,
				low:  bar.Low,
				ts:   bucketStart,
			}
			buckets[key] = b
		}

		if bar.High > b.high {
			b.high = bar.High
		}
		if bar.Low < b.low {
			b.low = bar.Low
		}
		// Overwrite close on each bar — last iteration = chronologically latest
		b.closeVal = bar.Close
		b.volume += bar.Volume
		b.count++
	}

	// Convert buckets to bars
	barsPerBucket := minutes / 15
	minBars := barsPerBucket / 2 // require at least 50% complete
	if minBars < 1 {
		minBars = 1
	}

	result := make([]domain.IntradayBar, 0, len(buckets))
	contractCode := bars[0].ContractCode
	symbol := bars[0].Symbol
	source := bars[0].Source

	for _, b := range buckets {
		if b.count < minBars {
			continue // skip incomplete buckets
		}
		result = append(result, domain.IntradayBar{
			ContractCode: contractCode,
			Symbol:       symbol,
			Interval:     targetInterval,
			Timestamp:    b.ts,
			Open:         b.open,
			High:         b.high,
			Low:          b.low,
			Close:        b.closeVal,
			Volume:       b.volume,
			Source:       source,
		})
	}

	// Sort newest-first
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	return result
}

// alignToBucket rounds a timestamp down to the nearest bucket boundary.
func alignToBucket(t time.Time, minutes int) time.Time {
	// Total minutes since midnight
	totalMin := t.Hour()*60 + t.Minute()
	bucketMin := (totalMin / minutes) * minutes
	h := bucketMin / 60
	m := bucketMin % 60
	return time.Date(t.Year(), t.Month(), t.Day(), h, m, 0, 0, t.Location())
}
