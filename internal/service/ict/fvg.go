package ict

import (
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// DetectFVG detects Fair Value Gaps from a newest-first bar slice.
//
// When the slice contains ≥20 bars the detection is delegated to the
// canonical ta.CalcICT implementation (single source of truth).
// For smaller slices (3-19 bars) a lightweight scan is used so that
// callers with limited data still get results.
func DetectFVG(bars []ta.OHLCV) []FVGZone {
	n := len(bars)
	if n < 3 {
		return nil
	}

	atr := ta.CalcATR(bars, 14)
	if atr <= 0 {
		atr = estimateATR(bars)
		if atr <= 0 {
			return nil
		}
	}

	// Prefer the canonical implementation when data is sufficient.
	if n >= 20 {
		if taResult := ta.CalcICT(bars, atr); taResult != nil {
			return convertFVGs(taResult.FairValueGaps)
		}
	}

	// Fallback: lightweight scan for small datasets.
	return detectFVGSimple(bars, atr)
}

// detectFVGSimple performs a minimal FVG scan without the full CalcICT
// machinery. It mirrors the three-candle gap logic used by ta/ict.go.
func detectFVGSimple(bars []ta.OHLCV, atr float64) []FVGZone {
	n := len(bars)
	minSize := atr * 0.1
	var zones []FVGZone

	// bars is newest-first; i is the middle candle index.
	for i := 1; i < n-1; i++ {
		prev := bars[i+1] // older
		next := bars[i-1] // newer

		// Bullish FVG: gap between prev.High and next.Low
		if prev.High < next.Low {
			gap := next.Low - prev.High
			if gap >= minSize {
				zones = append(zones, FVGZone{
					Kind:     "BULLISH",
					Top:      next.Low,
					Bottom:   prev.High,
					BarIndex: i,
				})
			}
		}

		// Bearish FVG: gap between next.High and prev.Low
		if next.High < prev.Low {
			gap := prev.Low - next.High
			if gap >= minSize {
				zones = append(zones, FVGZone{
					Kind:     "BEARISH",
					Top:      prev.Low,
					Bottom:   next.High,
					BarIndex: i,
				})
			}
		}
	}

	// Check fill status using subsequent (newer) bars.
	for z := range zones {
		fvg := &zones[z]
		gapSize := fvg.Top - fvg.Bottom
		if gapSize <= 0 {
			fvg.Filled = true
			fvg.FillPct = 100
			continue
		}
		maxPen := 0.0
		for j := 0; j < fvg.BarIndex; j++ {
			b := bars[j]
			if fvg.Kind == "BULLISH" && b.Low < fvg.Top {
				pen := fvg.Top - b.Low
				if pen > maxPen {
					maxPen = pen
				}
			} else if fvg.Kind == "BEARISH" && b.High > fvg.Bottom {
				pen := b.High - fvg.Bottom
				if pen > maxPen {
					maxPen = pen
				}
			}
		}
		pct := (maxPen / gapSize) * 100
		if pct > 100 {
			pct = 100
		}
		fvg.FillPct = pct
		fvg.Filled = pct >= 100
	}

	if len(zones) > 10 {
		zones = zones[len(zones)-10:]
	}
	return zones
}

// estimateATR provides a simple average-range fallback when CalcATR returns 0.
func estimateATR(bars []ta.OHLCV) float64 {
	if len(bars) == 0 {
		return 0
	}
	sum := 0.0
	for _, b := range bars {
		sum += b.High - b.Low
	}
	return sum / float64(len(bars))
}
