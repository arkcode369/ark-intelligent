package ta

// smc.go — Smart Money Concepts (SMC): Break of Structure (BOS) and
// Change of Character (CHOCH) detection, plus Premium/Discount zone classifier.
//
// Implements:
//   - DetectStructureBreaks(bars []OHLCV) SMCResult
//   - ClassifyPremiumDiscount(bars []OHLCV, lookback int) PremiumDiscountResult
//
// Bars are always newest-first (index 0 = most recent bar).

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// StructurePoint represents a significant swing high or low.
type StructurePoint struct {
	Type     string  // "SWING_HIGH" or "SWING_LOW"
	Price    float64 // price level of the swing
	BarIndex int     // index in bars slice (newest-first)
}

// StructureBreak records a BOS (Break of Structure) or CHOCH (Change of Character).
type StructureBreak struct {
	Type        string  // "BOS" or "CHOCH"
	Direction   string  // "BULLISH" or "BEARISH"
	Level       float64 // swing level that was broken
	ConfirmedAt int     // bar index (newest-first) where break was confirmed
	PriorTrend  string  // "BULLISH" or "BEARISH" — structure before this break
}

// PremiumDiscountResult classifies price position within the current range.
type PremiumDiscountResult struct {
	Zone            string  // "PREMIUM", "DISCOUNT", or "EQUILIBRIUM"
	RangeHigh       float64 // highest price in the lookback window
	RangeLow        float64 // lowest price in the lookback window
	MidPoint        float64 // (RangeHigh + RangeLow) / 2
	CurrentPosition float64 // current price as % of range (0 = low, 100 = high)
}

// SMCResult bundles BOS/CHOCH events and premium/discount zone.
type SMCResult struct {
	Breaks             []StructureBreak    // up to 3 most recent breaks, newest-first
	PremiumDiscount    PremiumDiscountResult
	LastStructureTrend string // "BULLISH", "BEARISH", or "RANGING"
}

// ---------------------------------------------------------------------------
// DetectStructureBreaks
// ---------------------------------------------------------------------------

// detectSMCSwings finds swing highs and lows using a 5-bar pivot on each side.
// Returns points in oldest-first order.
func detectSMCSwings(bars []OHLCV) []StructurePoint {
	const n = 5 // bars on each side
	count := len(bars)
	if count < 2*n+1 {
		return nil
	}

	// Work oldest-first
	asc := reverseOHLCV(bars)
	ascN := len(asc)

	var points []StructurePoint
	for i := n; i < ascN-n; i++ {
		// Check swing high
		isHigh := true
		for j := 1; j <= n; j++ {
			if asc[i-j].High >= asc[i].High || asc[i+j].High >= asc[i].High {
				isHigh = false
				break
			}
		}
		if isHigh {
			points = append(points, StructurePoint{
				Type:     "SWING_HIGH",
				Price:    asc[i].High,
				BarIndex: (ascN - 1 - i), // convert to newest-first index
			})
		}

		// Check swing low
		isLow := true
		for j := 1; j <= n; j++ {
			if asc[i-j].Low <= asc[i].Low || asc[i+j].Low <= asc[i].Low {
				isLow = false
				break
			}
		}
		if isLow {
			points = append(points, StructurePoint{
				Type:     "SWING_LOW",
				Price:    asc[i].Low,
				BarIndex: (ascN - 1 - i), // newest-first index
			})
		}
	}

	// Sort by BarIndex descending (newest-first)
	// Simple insertion sort is sufficient for small n
	for i := 1; i < len(points); i++ {
		for j := i; j > 0 && points[j].BarIndex < points[j-1].BarIndex; j-- {
			points[j], points[j-1] = points[j-1], points[j]
		}
	}

	return points
}

// DetectStructureBreaks scans bars (newest-first) for BOS and CHOCH events.
//
// A BOS (Break of Structure) occurs when price closes beyond the last swing
// high/low in the direction of the prevailing trend (confirmation signal).
//
// A CHOCH (Change of Character) occurs when price breaks against the prevailing
// trend (reversal signal).
//
// Returns up to 3 most recent breaks, newest-first.
func DetectStructureBreaks(bars []OHLCV) SMCResult {
	if len(bars) < 15 {
		return SMCResult{LastStructureTrend: "RANGING"}
	}

	// Detect swing points (oldest-first internally → converted to newest-first)
	points := detectSMCSwings(bars)
	if len(points) < 2 {
		return SMCResult{
			LastStructureTrend: "RANGING",
			PremiumDiscount:    ClassifyPremiumDiscount(bars, 50),
		}
	}

	// Sort oldest-first for sequential analysis
	// (detectSMCSwings returned newest-first, reverse to oldest-first)
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}

	// State machine: track structure and last reference levels
	currentTrend := "RANGING"
	var lastHigh, lastLow float64
	lastHighSet := false
	lastLowSet := false

	var breaks []StructureBreak

	// Initialize with first swing points
	for _, p := range points {
		if !lastHighSet && p.Type == "SWING_HIGH" {
			lastHigh = p.Price
			lastHighSet = true
		}
		if !lastLowSet && p.Type == "SWING_LOW" {
			lastLow = p.Price
			lastLowSet = true
		}
		if lastHighSet && lastLowSet {
			break
		}
	}

	// Now scan each bar (oldest-first) for breaks
	n := len(bars)
	asc := reverseOHLCV(bars)

	for i := 1; i < n; i++ {
		bar := asc[i]
		newestFirstIdx := n - 1 - i

		// Check bullish break (close above last swing high)
		if lastHighSet && bar.Close > lastHigh {
			breakType := "BOS"
			priorTrend := currentTrend

			if currentTrend == "BEARISH" {
				breakType = "CHOCH"
			}

			evt := StructureBreak{
				Type:        breakType,
				Direction:   "BULLISH",
				Level:       lastHigh,
				ConfirmedAt: newestFirstIdx,
				PriorTrend:  priorTrend,
			}
			breaks = append(breaks, evt)
			currentTrend = "BULLISH"

			// Update last high to the new swing high if available
			for _, p := range points {
				// p.BarIndex is newest-first → convert to oldest-first for comparison
				pOldestFirst := n - 1 - p.BarIndex
				if p.Type == "SWING_HIGH" && pOldestFirst <= i && p.Price > lastHigh {
					lastHigh = p.Price
				}
			}
		}

		// Check bearish break (close below last swing low)
		if lastLowSet && bar.Close < lastLow {
			breakType := "BOS"
			priorTrend := currentTrend

			if currentTrend == "BULLISH" {
				breakType = "CHOCH"
			}

			evt := StructureBreak{
				Type:        breakType,
				Direction:   "BEARISH",
				Level:       lastLow,
				ConfirmedAt: newestFirstIdx,
				PriorTrend:  priorTrend,
			}
			breaks = append(breaks, evt)
			currentTrend = "BEARISH"

			// Update last low to the new swing low if available
			for _, p := range points {
				pOldestFirst := n - 1 - p.BarIndex
				if p.Type == "SWING_LOW" && pOldestFirst <= i && p.Price < lastLow {
					lastLow = p.Price
				}
			}
		}

		// Update reference levels from swing points at this bar
		for _, p := range points {
			pOldestFirst := n - 1 - p.BarIndex
			if pOldestFirst == i {
				if p.Type == "SWING_HIGH" {
					lastHigh = p.Price
					lastHighSet = true
				} else {
					lastLow = p.Price
					lastLowSet = true
				}
			}
		}
	}

	// Return most recent 3 breaks (reverse to newest-first)
	for i, j := 0, len(breaks)-1; i < j; i, j = i+1, j-1 {
		breaks[i], breaks[j] = breaks[j], breaks[i]
	}
	if len(breaks) > 3 {
		breaks = breaks[:3]
	}

	return SMCResult{
		Breaks:             breaks,
		PremiumDiscount:    ClassifyPremiumDiscount(bars, 50),
		LastStructureTrend: currentTrend,
	}
}

// ---------------------------------------------------------------------------
// ClassifyPremiumDiscount
// ---------------------------------------------------------------------------

// ClassifyPremiumDiscount determines whether the current price is in a premium,
// discount, or equilibrium zone relative to the recent price range.
//
// - PREMIUM: price > midpoint + 10% of range
// - DISCOUNT: price < midpoint - 10% of range
// - EQUILIBRIUM: within ±10% of the midpoint
func ClassifyPremiumDiscount(bars []OHLCV, lookback int) PremiumDiscountResult {
	if len(bars) == 0 {
		return PremiumDiscountResult{Zone: "EQUILIBRIUM"}
	}

	if lookback <= 0 || lookback > len(bars) {
		lookback = len(bars)
	}

	window := bars[:lookback]
	rangeHigh := window[0].High
	rangeLow := window[0].Low

	for _, b := range window {
		if b.High > rangeHigh {
			rangeHigh = b.High
		}
		if b.Low < rangeLow {
			rangeLow = b.Low
		}
	}

	priceRange := rangeHigh - rangeLow
	midPoint := (rangeHigh + rangeLow) / 2
	currentPrice := bars[0].Close

	// Current position as % of range
	currentPct := 0.0
	if priceRange > 0 {
		currentPct = (currentPrice - rangeLow) / priceRange * 100
	}

	zone := "EQUILIBRIUM"
	if priceRange > 0 {
		premiumThreshold := midPoint + priceRange*0.10
		discountThreshold := midPoint - priceRange*0.10
		if currentPrice > premiumThreshold {
			zone = "PREMIUM"
		} else if currentPrice < discountThreshold {
			zone = "DISCOUNT"
		}
	}

	return PremiumDiscountResult{
		Zone:            zone,
		RangeHigh:       rangeHigh,
		RangeLow:        rangeLow,
		MidPoint:        midPoint,
		CurrentPosition: currentPct,
	}
}
