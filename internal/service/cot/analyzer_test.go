package cot

import (
	"math"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/stretchr/testify/assert"
)

// ==================== computeCOTIndex Tests ====================

func TestComputeCOTIndex_AllZero(t *testing.T) {
	// Test with all zeros - should return 50 (neutral)
	nets := []float64{0, 0, 0, 0, 0}
	result := computeCOTIndex(nets)
	assert.Equal(t, 50.0, result)
}

func TestComputeCOTIndex_MaxNet(t *testing.T) {
	// Current at max - should return 100
	nets := []float64{100, 50, 0, -50, -100}
	result := computeCOTIndex(nets)
	assert.Equal(t, 100.0, result)
}

func TestComputeCOTIndex_MinNet(t *testing.T) {
	// Current at min - should return 0
	nets := []float64{-100, -50, 0, 50, 100}
	result := computeCOTIndex(nets)
	assert.Equal(t, 0.0, result)
}

func TestComputeCOTIndex_MiddleValue(t *testing.T) {
	// Current is in the middle of range
	nets := []float64{50, 100, 0, -50, -100}
	result := computeCOTIndex(nets)
	// (50 - (-100)) / (100 - (-100)) * 100 = 150/200 * 100 = 75
	assert.Equal(t, 75.0, result)
}

func TestComputeCOTIndex_SingleElement(t *testing.T) {
	// Edge case: single element - should return 50 (insufficient data)
	nets := []float64{100}
	result := computeCOTIndex(nets)
	assert.Equal(t, 50.0, result)
}

func TestComputeCOTIndex_TwoElements(t *testing.T) {
	// Edge case: two elements - should return 50 (insufficient data)
	nets := []float64{100, 0}
	result := computeCOTIndex(nets)
	assert.Equal(t, 50.0, result)
}

func TestComputeCOTIndex_ThreeElements(t *testing.T) {
	// Minimum required: 3 elements
	nets := []float64{50, 100, 0}
	result := computeCOTIndex(nets)
	// (50 - 0) / (100 - 0) * 100 = 50
	assert.Equal(t, 50.0, result)
}

func TestComputeCOTIndex_ZeroSpan(t *testing.T) {
	// Edge case: all values same (span = 0)
	nets := []float64{50, 50, 50, 50}
	result := computeCOTIndex(nets)
	assert.Equal(t, 50.0, result)
}

// ==================== classifySignal Tests ====================

func TestClassifySignal_BullishSpec_HighIndexWithMomentum(t *testing.T) {
	// Speculator: index >= 75 and momentum > 0 = STRONG_BULLISH
	result := classifySignal(80, 1000, false)
	assert.Equal(t, "STRONG_BULLISH", result)
}

func TestClassifySignal_BullishSpec_HighIndexNoMomentum(t *testing.T) {
	// Speculator: index >= 75 without momentum = BULLISH
	result := classifySignal(80, 0, false)
	assert.Equal(t, "BULLISH", result)
}

func TestClassifySignal_BearishSpec_LowIndexWithMomentum(t *testing.T) {
	// Speculator: index <= 25 and momentum < 0 = STRONG_BEARISH
	result := classifySignal(20, -1000, false)
	assert.Equal(t, "STRONG_BEARISH", result)
}

func TestClassifySignal_BearishSpec_LowIndexNoMomentum(t *testing.T) {
	// Speculator: index <= 25 without momentum = BEARISH
	result := classifySignal(20, 0, false)
	assert.Equal(t, "BEARISH", result)
}

func TestClassifySignal_NeutralSpec(t *testing.T) {
	// Speculator: index between 25 and 75 = NEUTRAL
	result := classifySignal(50, 0, false)
	assert.Equal(t, "NEUTRAL", result)
}

func TestClassifySignal_Commercial_StrongBullish(t *testing.T) {
	// Commercial: index >= 75 with momentum > 0 = STRONG_BULLISH (contrarian)
	result := classifySignal(80, 1000, true)
	assert.Equal(t, "STRONG_BULLISH", result)
}

func TestClassifySignal_Commercial_Bullish(t *testing.T) {
	// Commercial: index >= 75 without momentum = BULLISH
	result := classifySignal(80, 0, true)
	assert.Equal(t, "BULLISH", result)
}

func TestClassifySignal_Commercial_StrongBearish(t *testing.T) {
	// Commercial: index <= 25 with momentum < 0 = STRONG_BEARISH
	result := classifySignal(20, -1000, true)
	assert.Equal(t, "STRONG_BEARISH", result)
}

func TestClassifySignal_Commercial_Bearish(t *testing.T) {
	// Commercial: index <= 25 without momentum = BEARISH
	result := classifySignal(20, 0, true)
	assert.Equal(t, "BEARISH", result)
}

func TestClassifySignal_Commercial_Neutral(t *testing.T) {
	// Commercial: index between 25 and 75 = NEUTRAL
	result := classifySignal(50, 0, true)
	assert.Equal(t, "NEUTRAL", result)
}

// ==================== detectDivergence Tests ====================

func TestDetectDivergence_True(t *testing.T) {
	// Spec and comm moving opposite directions
	result := detectDivergence(5000, -5000)
	assert.True(t, result)
}

func TestDetectDivergence_False_SameDirection(t *testing.T) {
	// Spec and comm moving same direction
	result := detectDivergence(5000, 3000)
	assert.False(t, result)
}

func TestDetectDivergence_False_SpecTooSmall(t *testing.T) {
	// Spec change too small (< 1000 threshold)
	result := detectDivergence(500, -5000)
	assert.False(t, result)
}

func TestDetectDivergence_False_CommTooSmall(t *testing.T) {
	// Comm change too small (< 1000 threshold)
	result := detectDivergence(5000, -500)
	assert.False(t, result)
}

func TestDetectDivergence_False_BothTooSmall(t *testing.T) {
	// Both changes too small
	result := detectDivergence(500, -500)
	assert.False(t, result)
}

func TestDetectDivergence_False_ZeroChanges(t *testing.T) {
	// Zero changes
	result := detectDivergence(0, 0)
	assert.False(t, result)
}

// ==================== classifyMomentumDir Tests ====================

func TestClassifyMomentumDir_MomentumBuilding(t *testing.T) {
	// Spec building, comm unwinding
	result := classifyMomentumDir(1000, -1000)
	assert.Equal(t, domain.MomentumBuilding, result)
}

func TestClassifyMomentumDir_MomentumReversing(t *testing.T) {
	// Spec reducing, comm adding
	result := classifyMomentumDir(-1000, 1000)
	assert.Equal(t, domain.MomentumReversing, result)
}

func TestClassifyMomentumDir_MomentumStable(t *testing.T) {
	// Both momentum values small (< 100)
	result := classifyMomentumDir(50, 50)
	assert.Equal(t, domain.MomentumStable, result)
}

func TestClassifyMomentumDir_MomentumUnwinding(t *testing.T) {
	// Spec negative, comm negative or small positive (but not strongly positive)
	result := classifyMomentumDir(-200, -50)
	assert.Equal(t, domain.MomentumUnwinding, result)
}

func TestClassifyMomentumDir_MomentumBuilding_BothPositive(t *testing.T) {
	// Both positive, spec building more
	result := classifyMomentumDir(200, 50)
	assert.Equal(t, domain.MomentumBuilding, result)
}

// ==================== classifySmallSpec Tests ====================

func TestClassifySmallSpec_CrowdLong(t *testing.T) {
	// Net positive with high crowding
	a := domain.COTAnalysis{
		NetSmallSpec:  1000,
		CrowdingIndex: 70,
	}
	result := classifySmallSpec(a)
	assert.Equal(t, "CROWD_LONG", result)
}

func TestClassifySmallSpec_CrowdShort(t *testing.T) {
	// Net negative with high crowding
	a := domain.COTAnalysis{
		NetSmallSpec:  -1000,
		CrowdingIndex: 70,
	}
	result := classifySmallSpec(a)
	assert.Equal(t, "CROWD_SHORT", result)
}

func TestClassifySmallSpec_Neutral_LowCrowding(t *testing.T) {
	// Low crowding = neutral
	a := domain.COTAnalysis{
		NetSmallSpec:  1000,
		CrowdingIndex: 50,
	}
	result := classifySmallSpec(a)
	assert.Equal(t, "NEUTRAL", result)
}

func TestClassifySmallSpec_Neutral_ZeroNet(t *testing.T) {
	// Zero net position
	a := domain.COTAnalysis{
		NetSmallSpec:  0,
		CrowdingIndex: 70,
	}
	result := classifySmallSpec(a)
	assert.Equal(t, "NEUTRAL", result)
}

// ==================== classifySignalStrength Tests ====================

func TestClassifySignalStrength_Strong(t *testing.T) {
	// High sentiment + extreme
	a := domain.COTAnalysis{
		SentimentScore: 70,
		IsExtremeBull: true,
	}
	result := classifySignalStrength(a)
	assert.Equal(t, domain.SignalStrong, result)
}

func TestClassifySignalStrength_Strong_Bear(t *testing.T) {
	// High negative sentiment + extreme bear
	a := domain.COTAnalysis{
		SentimentScore: -70,
		IsExtremeBear:  true,
	}
	result := classifySignalStrength(a)
	assert.Equal(t, domain.SignalStrong, result)
}

func TestClassifySignalStrength_Moderate(t *testing.T) {
	// Moderate sentiment
	a := domain.COTAnalysis{
		SentimentScore: 50,
		IsExtremeBull:  false,
	}
	result := classifySignalStrength(a)
	assert.Equal(t, domain.SignalModerate, result)
}

func TestClassifySignalStrength_Weak(t *testing.T) {
	// Low sentiment
	a := domain.COTAnalysis{
		SentimentScore: 30,
	}
	result := classifySignalStrength(a)
	assert.Equal(t, domain.SignalWeak, result)
}

func TestClassifySignalStrength_Neutral(t *testing.T) {
	// Very low sentiment
	a := domain.COTAnalysis{
		SentimentScore: 10,
	}
	result := classifySignalStrength(a)
	assert.Equal(t, domain.SignalNeutral, result)
}

func TestClassifySignalStrength_ExtremeOnly_NotStrong(t *testing.T) {
	// Extreme without high sentiment should not be strong
	a := domain.COTAnalysis{
		SentimentScore: 30, // Below 60 threshold
		IsExtremeBull:  true,
	}
	result := classifySignalStrength(a)
	assert.Equal(t, domain.SignalWeak, result) // 30 > 20 but < 40, so weak
}

// ==================== computeCrowding Tests ====================

func TestComputeCrowding_TFF_Balanced(t *testing.T) {
	// Perfectly balanced = 0 crowding
	r := domain.COTRecord{
		LevFundLong:  5000,
		LevFundShort: 5000,
	}
	result := computeCrowding(r, "TFF")
	assert.Equal(t, 0.0, result)
}

func TestComputeCrowding_TFF_FullyLong(t *testing.T) {
	// All long = 100 crowding
	r := domain.COTRecord{
		LevFundLong:  10000,
		LevFundShort: 0,
	}
	result := computeCrowding(r, "TFF")
	assert.Equal(t, 100.0, result)
}

func TestComputeCrowding_TFF_FullyShort(t *testing.T) {
	// All short = 100 crowding
	r := domain.COTRecord{
		LevFundLong:  0,
		LevFundShort: 10000,
	}
	result := computeCrowding(r, "TFF")
	assert.Equal(t, 100.0, result)
}

func TestComputeCrowding_TFF_75PercentLong(t *testing.T) {
	// 75% long = 50 crowding (deviation from 50% * 2)
	r := domain.COTRecord{
		LevFundLong:  7500,
		LevFundShort: 2500,
	}
	result := computeCrowding(r, "TFF")
	assert.Equal(t, 50.0, result)
}

func TestComputeCrowding_DISAGG_Balanced(t *testing.T) {
	// DISAGGREGATED report type
	r := domain.COTRecord{
		ManagedMoneyLong:  5000,
		ManagedMoneyShort: 5000,
	}
	result := computeCrowding(r, "DISAGGREGATED")
	assert.Equal(t, 0.0, result)
}

func TestComputeCrowding_DISAGG_FullyLong(t *testing.T) {
	r := domain.COTRecord{
		ManagedMoneyLong:  10000,
		ManagedMoneyShort: 0,
	}
	result := computeCrowding(r, "DISAGGREGATED")
	assert.Equal(t, 100.0, result)
}

func TestComputeCrowding_ZeroTotal(t *testing.T) {
	// No speculative positions = neutral (50)
	r := domain.COTRecord{
		LevFundLong:  0,
		LevFundShort: 0,
	}
	result := computeCrowding(r, "TFF")
	assert.Equal(t, 50.0, result)
}

// ==================== safeRatio Tests ====================

func TestSafeRatio_Normal(t *testing.T) {
	result := safeRatio(100, 50)
	assert.Equal(t, 2.0, result)
}

func TestSafeRatio_BZero_APositive(t *testing.T) {
	// Division by zero with positive numerator
	result := safeRatio(100, 0)
	assert.Equal(t, 999.99, result)
}

func TestSafeRatio_BZero_Zero(t *testing.T) {
	// Both zero
	result := safeRatio(0, 0)
	assert.Equal(t, 0.0, result)
}

func TestSafeRatio_BZero_ANegative(t *testing.T) {
	// Division by zero with negative numerator
	result := safeRatio(-100, 0)
	assert.Equal(t, 0.0, result)
}

func TestSafeRatio_Rounding(t *testing.T) {
	// Verify rounding to 2 decimal places
	result := safeRatio(100, 33)
	assert.Equal(t, 3.03, result) // 100/33 = 3.0303... rounds to 3.03
}

// ==================== signF Tests ====================

func TestSignF_Positive(t *testing.T) {
	result := signF(100)
	assert.Equal(t, 1.0, result)
}

func TestSignF_Negative(t *testing.T) {
	result := signF(-100)
	assert.Equal(t, -1.0, result)
}

func TestSignF_Zero(t *testing.T) {
	result := signF(0)
	assert.Equal(t, 0.0, result)
}

func TestSignF_SmallPositive(t *testing.T) {
	result := signF(0.001)
	assert.Equal(t, 1.0, result)
}

func TestSignF_SmallNegative(t *testing.T) {
	result := signF(-0.001)
	assert.Equal(t, -1.0, result)
}

// ==================== minInt Tests ====================

func TestMinInt_FirstSmaller(t *testing.T) {
	result := minInt(5, 10)
	assert.Equal(t, 5, result)
}

func TestMinInt_SecondSmaller(t *testing.T) {
	result := minInt(10, 5)
	assert.Equal(t, 5, result)
}

func TestMinInt_Equal(t *testing.T) {
	result := minInt(5, 5)
	assert.Equal(t, 5, result)
}

func TestMinInt_Negative(t *testing.T) {
	result := minInt(-10, -5)
	assert.Equal(t, -10, result)
}

func TestMinInt_Zero(t *testing.T) {
	result := minInt(0, 5)
	assert.Equal(t, 0, result)
}

// ==================== reverseFloats Tests ====================

func TestReverseFloats(t *testing.T) {
	input := []float64{1, 2, 3, 4, 5}
	expected := []float64{5, 4, 3, 2, 1}
	result := reverseFloats(input)
	assert.Equal(t, expected, result)
}

func TestReverseFloats_Empty(t *testing.T) {
	input := []float64{}
	result := reverseFloats(input)
	assert.Empty(t, result)
}

func TestReverseFloats_Single(t *testing.T) {
	input := []float64{42}
	result := reverseFloats(input)
	assert.Equal(t, []float64{42}, result)
}

func TestReverseFloats_Two(t *testing.T) {
	input := []float64{1, 2}
	result := reverseFloats(input)
	assert.Equal(t, []float64{2, 1}, result)
}

// ==================== computeSentiment Tests ====================

func TestComputeSentiment_MaxBullish(t *testing.T) {
	// Max bullish: COTIndex = 100, COTIndexComm = 100 (for DISAGG=same direction), momentum max, no crowding
	a := domain.COTAnalysis{
		Contract:       domain.COTContract{ReportType: "DISAGGREGATED"},
		COTIndex:       100,
		COTIndexComm:   100, // Same direction for DISAGG = bullish
		SpecMomentum4W: 5000,
		CrowdingIndex:  50,
	}
	result := computeSentiment(a)
	// Index: (100-50)*2*0.4 = 40
	// Comm: (100-50)*2*0.3 = 30
	// Mom: 20 (max)
	// Total: 90, clamped to 100
	assert.Greater(t, result, 50.0)
	assert.LessOrEqual(t, result, 100.0)
}

func TestComputeSentiment_MaxBearish(t *testing.T) {
	// Max bearish: COTIndex = 0, COTIndexComm = 0 (for DISAGG)
	a := domain.COTAnalysis{
		Contract:       domain.COTContract{ReportType: "DISAGGREGATED"},
		COTIndex:       0,
		COTIndexComm:   0,
		SpecMomentum4W: -5000,
		CrowdingIndex:  50,
	}
	result := computeSentiment(a)
	// Index: (0-50)*2*0.4 = -40
	// Comm: (0-50)*2*0.3 = -30
	// Mom: -20 (min)
	// Total: -90, clamped to -100
	assert.Less(t, result, -50.0)
	assert.GreaterOrEqual(t, result, -100.0)
}

func TestComputeSentiment_Neutral(t *testing.T) {
	// Neutral: COTIndex = 50, COTIndexComm = 50, no momentum, neutral crowding
	a := domain.COTAnalysis{
		Contract:       domain.COTContract{ReportType: "DISAGGREGATED"},
		COTIndex:       50,
		COTIndexComm:   50,
		SpecMomentum4W: 0,
		CrowdingIndex:  50,
	}
	result := computeSentiment(a)
	// All components at neutral should give ~0
	assert.InDelta(t, 0, result, 5)
}

func TestComputeSentiment_TFF_CommercialInverted(t *testing.T) {
	// For TFF, commercial score is inverted
	a := domain.COTAnalysis{
		Contract:       domain.COTContract{ReportType: "TFF"},
		COTIndex:       50,
		COTIndexComm:   80, // High = bearish for TFF
		SpecMomentum4W: 0,
		CrowdingIndex:  50,
	}
	resultDISAGG := computeSentiment(domain.COTAnalysis{
		Contract:       domain.COTContract{ReportType: "DISAGGREGATED"},
		COTIndex:       50,
		COTIndexComm:   80,
		SpecMomentum4W: 0,
		CrowdingIndex:  50,
	})
	resultTFF := computeSentiment(a)
	// TFF should be more negative (or less positive) than DISAGG for same high commercial index
	assert.Less(t, resultTFF, resultDISAGG)
}

func TestComputeSentiment_Clamped(t *testing.T) {
	// Test that result is clamped to [-100, 100]
	a := domain.COTAnalysis{
		Contract:       domain.COTContract{ReportType: "DISAGGREGATED"},
		COTIndex:       1000, // Impossible but test clamping
		COTIndexComm:   1000,
		SpecMomentum4W: 100000,
		CrowdingIndex:  0,
	}
	result := computeSentiment(a)
	assert.LessOrEqual(t, result, 100.0)
}

// ==================== findContractByCode Tests ====================

func TestFindContractByCode_Found(t *testing.T) {
	result := findContractByCode("EUR")
	assert.Equal(t, "EUR", result.Code)
}

func TestFindContractByCode_NotFound(t *testing.T) {
	result := findContractByCode("UNKNOWN")
	assert.Equal(t, "UNKNOWN", result.Code)
}

// ==================== extractNets Tests ====================

func TestExtractNets(t *testing.T) {
	records := []domain.COTRecord{
		{LevFundLong: 100, LevFundShort: 50},
		{LevFundLong: 200, LevFundShort: 100},
		{LevFundLong: 300, LevFundShort: 150},
	}
	fn := func(r domain.COTRecord) float64 {
		return r.LevFundLong - r.LevFundShort
	}
	result := extractNets(records, fn)
	expected := []float64{50, 100, 150}
	assert.Equal(t, expected, result)
}

func TestExtractNets_Empty(t *testing.T) {
	var records []domain.COTRecord
	fn := func(r domain.COTRecord) float64 { return 0 }
	result := extractNets(records, fn)
	assert.Empty(t, result)
}

// ==================== Edge Case & Fuzz Tests ====================

func TestComputeCOTIndex_LargeValues(t *testing.T) {
	nets := []float64{1e9, 5e8, 0, -5e8, -1e9}
	result := computeCOTIndex(nets)
	assert.Equal(t, 100.0, result)
}

func TestComputeCOTIndex_SmallValues(t *testing.T) {
	nets := []float64{0.001, 0.0005, 0, -0.0005, -0.001}
	result := computeCOTIndex(nets)
	assert.InDelta(t, 100.0, result, 0.0001)
}

func TestComputeCOTIndex_NaNInf(t *testing.T) {
	// Test handling of special float values
	nets := []float64{math.NaN(), 50, 0}
	result := computeCOTIndex(nets)
	// NaN comparison should result in NaN or 50 depending on implementation
	// Our implementation doesn't explicitly handle NaN but math.Min/Max do
	assert.True(t, math.IsNaN(result) || result == 50.0)
}

func TestSafeRatio_VerySmallNumbers(t *testing.T) {
	result := safeRatio(0.001, 0.0001)
	assert.Equal(t, 10.0, result)
}

func TestSafeRatio_VeryLargeNumbers(t *testing.T) {
	result := safeRatio(1e10, 1e5)
	assert.Equal(t, 100000.0, result)
}

func TestClassifySignal_BoundaryValues(t *testing.T) {
	// Test exactly at boundaries
	assert.Equal(t, "BULLISH", classifySignal(75, 0, false))
	assert.Equal(t, "BEARISH", classifySignal(25, 0, false))
	assert.Equal(t, "NEUTRAL", classifySignal(26, 0, false))
	assert.Equal(t, "NEUTRAL", classifySignal(74, 0, false))
}

func TestDetectDivergence_BoundaryThresholds(t *testing.T) {
	// Exactly at threshold (1000)
	result := detectDivergence(1000, -1000)
	assert.True(t, result)

	// Just below threshold
	result = detectDivergence(999, -999)
	assert.False(t, result)
}

func TestMinInt_MaxInt(t *testing.T) {
	result := minInt(math.MaxInt32, math.MaxInt32-1)
	assert.Equal(t, math.MaxInt32-1, result)
}

func TestReverseFloats_LargeSlice(t *testing.T) {
	input := make([]float64, 1000)
	for i := range input {
		input[i] = float64(i)
	}
	result := reverseFloats(input)
	assert.Equal(t, 999.0, result[0])
	assert.Equal(t, 0.0, result[999])
}
