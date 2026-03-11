// Package mathutil provides financial mathematics and statistical helpers
// used across the quantitative analysis engine.
package mathutil

import (
	"math"
	"sort"
)

// ---------------------------------------------------------------------------
// Descriptive Statistics
// ---------------------------------------------------------------------------

// Mean returns the arithmetic mean of a float64 slice.
// Returns 0 for empty input.
func Mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	var sum float64
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

// StdDev returns the population standard deviation.
// Returns 0 for fewer than 2 data points.
func StdDev(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}
	m := Mean(data)
	var ss float64
	for _, v := range data {
		d := v - m
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(data)))
}

// StdDevSample returns the sample standard deviation (Bessel-corrected).
func StdDevSample(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}
	m := Mean(data)
	var ss float64
	for _, v := range data {
		d := v - m
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(data)-1))
}

// Median returns the median value. Input is NOT mutated.
func Median(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// ---------------------------------------------------------------------------
// Percentile & Normalization
// ---------------------------------------------------------------------------

// Percentile returns the p-th percentile (0-100) using linear interpolation.
// Input is NOT mutated.
func Percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}

	rank := (p / 100) * float64(len(sorted)-1)
	lower := int(math.Floor(rank))
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[lower]
	}
	frac := rank - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

// Normalize rescales a value into [0, 100] given the min and max range.
// Returns 50 if min == max (avoids division by zero).
func Normalize(value, min, max float64) float64 {
	if max == min {
		return 50
	}
	n := (value - min) / (max - min) * 100
	return Clamp(n, 0, 100)
}

// MinMaxIndex computes the Williams-style COT Index:
//
//	Index = (current - minN) / (maxN - minN) * 100
//
// Returns 50 if maxN == minN.
func MinMaxIndex(current, minN, maxN float64) float64 {
	return Normalize(current, minN, maxN)
}

// ---------------------------------------------------------------------------
// Moving Averages
// ---------------------------------------------------------------------------

// SMA returns the Simple Moving Average of the last n values.
// If len(data) < n, uses all available data.
func SMA(data []float64, n int) float64 {
	if len(data) == 0 || n <= 0 {
		return 0
	}
	if n > len(data) {
		n = len(data)
	}
	var sum float64
	for i := len(data) - n; i < len(data); i++ {
		sum += data[i]
	}
	return sum / float64(n)
}

// EMA returns the Exponential Moving Average of the last n values.
// Uses smoothing factor k = 2 / (n + 1).
func EMA(data []float64, n int) float64 {
	if len(data) == 0 || n <= 0 {
		return 0
	}
	k := 2.0 / (float64(n) + 1)
	ema := data[0]
	for i := 1; i < len(data); i++ {
		ema = data[i]*k + ema*(1-k)
	}
	return ema
}

// ---------------------------------------------------------------------------
// Rate of Change & Momentum
// ---------------------------------------------------------------------------

// RateOfChange returns (current - previous) / |previous| * 100.
// Returns 0 if previous is zero.
func RateOfChange(current, previous float64) float64 {
	if previous == 0 {
		return 0
	}
	return (current - previous) / math.Abs(previous) * 100
}

// Momentum returns the simple difference: current - nPeriodsAgo.
// data should be ordered oldest-first. n is the lookback period.
func Momentum(data []float64, n int) float64 {
	if len(data) < n+1 || n <= 0 {
		return 0
	}
	return data[len(data)-1] - data[len(data)-1-n]
}

// ---------------------------------------------------------------------------
// Financial Helpers
// ---------------------------------------------------------------------------

// ZScore computes (value - mean) / stddev. Returns 0 if stddev is zero.
func ZScore(value, mean, stddev float64) float64 {
	if stddev == 0 {
		return 0
	}
	return (value - mean) / stddev
}

// ExponentialDecay returns value * exp(-lambda * t).
// lambda = ln(2) / halfLife (half-life in same units as t).
func ExponentialDecay(value, t, halfLife float64) float64 {
	if halfLife <= 0 {
		return 0
	}
	lambda := math.Ln2 / halfLife
	return value * math.Exp(-lambda*t)
}

// CumulativeDecaySum computes the time-decayed rolling sum of values.
// Each element in values has an associated age (in days) in ages.
// halfLife controls the decay rate in days.
func CumulativeDecaySum(values, ages []float64, halfLife float64) float64 {
	if len(values) != len(ages) {
		return 0
	}
	var sum float64
	for i, v := range values {
		sum += ExponentialDecay(v, ages[i], halfLife)
	}
	return sum
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

// Clamp restricts value to [min, max].
func Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Abs returns the absolute value of a float64.
func Abs(v float64) float64 {
	return math.Abs(v)
}

// Sign returns -1, 0, or +1 based on the sign of v.
func Sign(v float64) float64 {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

// MinFloat64 returns the minimum value in a float64 slice.
func MinFloat64(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	min := data[0]
	for _, v := range data[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

// MaxFloat64 returns the maximum value in a float64 slice.
func MaxFloat64(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	max := data[0]
	for _, v := range data[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

// ConsecutiveDirection counts how many consecutive tail elements share
// the same sign direction. data is ordered oldest-first.
// Returns (count, direction) where direction is +1, -1, or 0.
func ConsecutiveDirection(data []float64) (int, float64) {
	if len(data) == 0 {
		return 0, 0
	}
	dir := Sign(data[len(data)-1])
	count := 0
	for i := len(data) - 1; i >= 0; i-- {
		if Sign(data[i]) != dir {
			break
		}
		count++
	}
	return count, dir
}
