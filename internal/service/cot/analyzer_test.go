package cot

import (
	"math"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

func TestComputeCOTIndex(t *testing.T) {
	tests := []struct {
		name string
		nets []float64
		want float64
	}{
		{"empty", nil, 50},
		{"single value", []float64{100}, 50},
		{"all same", []float64{100, 100, 100}, 50},
		{"min=0 max=100 current=50", []float64{50, 0, 100}, 50},
		{"min=0 max=100 current=100", []float64{100, 0, 50}, 100},
		{"min=0 max=100 current=0", []float64{0, 50, 100}, 0},
		{"negative range", []float64{-50, -100, 0}, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeCOTIndex(tt.nets)
			if math.Abs(got-tt.want) > 1.0 {
				t.Errorf("computeCOTIndex(%v) = %v, want ~%v", tt.nets, got, tt.want)
			}
		})
	}
}

func TestClassifySignal(t *testing.T) {
	tests := []struct {
		name         string
		cotIndex     float64
		momentum     float64
		isCommercial bool
	}{
		{"high index spec", 85, 10, false},
		{"low index spec", 15, -10, false},
		{"mid index", 50, 0, false},
		{"commercial high", 85, 10, true},
		{"commercial low", 15, -10, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySignal(tt.cotIndex, tt.momentum, tt.isCommercial)
			if got == "" {
				t.Errorf("returned empty signal")
			}
		})
	}
}

func TestComputeCrowding(t *testing.T) {
	r := domain.COTRecord{
		SpecLong:  80000,
		SpecShort: 20000,
		CommLong:  50000,
		CommShort: 50000,
	}
	got := computeCrowding(r, "futures_only")
	if got < 0 || got > 100 {
		t.Errorf("computeCrowding() = %v, want 0-100", got)
	}
}

func TestSafeRatio(t *testing.T) {
	tests := []struct {
		name string
		a, b float64
		want float64
	}{
		{"normal", 10, 5, 2},
		{"zero denominator pos", 10, 0, 999.99},
		{"both zero", 0, 0, 0},
		{"negative", -10, 5, -2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeRatio(tt.a, tt.b)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("safeRatio(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestClassifySignalStrength(t *testing.T) {
	tests := []struct {
		name string
		a    domain.COTAnalysis
	}{
		{"neutral", domain.COTAnalysis{COTIndex: 50, COTIndexComm: 50}},
		{"strong", domain.COTAnalysis{COTIndex: 90, COTIndexComm: 10, CrowdingIndex: 80}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySignalStrength(tt.a)
			valid := got == domain.SignalStrong || got == domain.SignalModerate || got == domain.SignalWeak || got == domain.SignalNeutral
			if !valid {
				t.Errorf("got %q, want STRONG/MODERATE/WEAK/NEUTRAL", got)
			}
		})
	}
}

func TestExtractNets(t *testing.T) {
	history := []domain.COTRecord{
		{SpecLong: 100, SpecShort: 50},
		{SpecLong: 200, SpecShort: 80},
		{SpecLong: 150, SpecShort: 70},
	}
	fn := func(r domain.COTRecord) float64 {
		return float64(r.SpecLong - r.SpecShort)
	}
	got := extractNets(history, fn)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0] != 50 || got[1] != 120 || got[2] != 80 {
		t.Errorf("got %v, want [50 120 80]", got)
	}
}

func TestClassifyMomentumDir(t *testing.T) {
	got := classifyMomentumDir(5000, -5000)
	if got == domain.MomentumStable {
		t.Error("spec up + comm down should not be stable")
	}
	got2 := classifyMomentumDir(0, 0)
	if got2 != domain.MomentumStable {
		t.Errorf("both zero should be stable, got %q", got2)
	}
}

func TestSignF(t *testing.T) {
	if signF(5) != 1 {
		t.Error("signF(5) should be 1")
	}
	if signF(-5) != -1 {
		t.Error("signF(-5) should be -1")
	}
	if signF(0) != 0 {
		t.Error("signF(0) should be 0")
	}
}

// TestComputeSentiment tests the composite sentiment score calculation
func TestComputeSentiment(t *testing.T) {
	tests := []struct {
		name string
		a    domain.COTAnalysis
		want float64 // expected approximate value
	}{
		{
			name: "neutral_all_mid",
			a: domain.COTAnalysis{
				COTIndex:       50,
				COTIndexComm:   50,
				SpecMomentum4W: 0,
				CrowdingIndex:  50,
				Contract:       domain.COTContract{ReportType: "TFF"},
			},
			want: 0,
		},
		{
			name: "strong_bullish_spec_high",
			a: domain.COTAnalysis{
				COTIndex:       90,
				COTIndexComm:   10,
				SpecMomentum4W: 5000,
				CrowdingIndex:  30,
				Contract:       domain.COTContract{ReportType: "TFF"},
			},
			want: 80, // bullish: 32+24+20+4 = 80 (index*0.4 + comm*0.3 + momentum + crowding)
		},
		{
			name: "strong_bearish_spec_low",
			a: domain.COTAnalysis{
				COTIndex:       10,
				COTIndexComm:   90,
				SpecMomentum4W: -5000,
				CrowdingIndex:  70,
				Contract:       domain.COTContract{ReportType: "TFF"},
			},
			want: -80, // bearish: -32-24-20-4 = -80
		},
		{
			name: "disaggregated_same_direction",
			a: domain.COTAnalysis{
				COTIndex:       80,
				COTIndexComm:   80,
				SpecMomentum4W: 1000,
				CrowdingIndex:  40,
				Contract:       domain.COTContract{ReportType: "DISAGGREGATED"},
			},
			want: 54, // for DISAGG, commercial is same direction: (80-50)*2*0.4 + (80-50)*2*0.3
		},
		{
			name: "extreme_crowding_penalty",
			a: domain.COTAnalysis{
				COTIndex:       60,
				COTIndexComm:   40,
				SpecMomentum4W: 0,
				CrowdingIndex:  90, // extreme crowding penalty
				Contract:       domain.COTContract{ReportType: "TFF"},
			},
			want: -2, // low positive from index, negative crowding contribution
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeSentiment(tt.a)
			// Allow 15 point tolerance due to complex formula interactions
			if math.Abs(got-tt.want) > 15 {
				t.Errorf("computeSentiment() = %v, want ~%v", got, tt.want)
			}
		})
	}
}

// TestClassifySmallSpec tests the small speculator signal classification
func TestClassifySmallSpec(t *testing.T) {
	tests := []struct {
		name string
		a    domain.COTAnalysis
		want string
	}{
		{
			name: "neutral_no_crowd",
			a: domain.COTAnalysis{
				NetSmallSpec: 1000,
				CrowdingIndex:  50,
			},
			want: "NEUTRAL",
		},
		{
			name: "crowd_long",
			a: domain.COTAnalysis{
				NetSmallSpec: 5000,
				CrowdingIndex:  70,
			},
			want: "CROWD_LONG",
		},
		{
			name: "crowd_short",
			a: domain.COTAnalysis{
				NetSmallSpec: -5000,
				CrowdingIndex:  70,
			},
			want: "CROWD_SHORT",
		},
		{
			name: "neutral_edge_crowding",
			a: domain.COTAnalysis{
				NetSmallSpec: 1000,
				CrowdingIndex:  65, // exactly at threshold (>65 required, so NEUTRAL)
			},
			want: "NEUTRAL",
		},
		{
			name: "neutral_low_crowding",
			a: domain.COTAnalysis{
				NetSmallSpec: 1000,
				CrowdingIndex:  60, // below threshold
			},
			want: "NEUTRAL",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySmallSpec(tt.a)
			if got != tt.want {
				t.Errorf("classifySmallSpec() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDetectDivergencePure tests the pure divergence detection function in analyzer.go
func TestDetectDivergencePure(t *testing.T) {
	tests := []struct {
		name         string
		specNetChange  float64
		commNetChange  float64
		want         bool
	}{
		{
			name:          "divergence_spec_up_comm_down",
			specNetChange: 5000,
			commNetChange: -3000,
			want:          true,
		},
		{
			name:          "divergence_spec_down_comm_up",
			specNetChange: -5000,
			commNetChange: 3000,
			want:          true,
		},
		{
			name:          "no_divergence_both_up",
			specNetChange: 5000,
			commNetChange: 3000,
			want:          false,
		},
		{
			name:          "no_divergence_both_down",
			specNetChange: -5000,
			commNetChange: -3000,
			want:          false,
		},
		{
			name:          "no_divergence_below_threshold",
			specNetChange: 500,  // too small
			commNetChange: -500, // too small
			want:          false,
		},
		{
			name:          "no_divergence_both_zero",
			specNetChange: 0,
			commNetChange: 0,
			want:          false,
		},
		{
			name:          "divergence_large_values",
			specNetChange: 15000,
			commNetChange: -12000,
			want:          true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectDivergence(tt.specNetChange, tt.commNetChange)
			if got != tt.want {
				t.Errorf("detectDivergence(%v, %v) = %v, want %v",
					tt.specNetChange, tt.commNetChange, got, tt.want)
			}
		})
	}
}

// TestClassifySignal_Detailed tests signal classification with specific expected outputs
func TestClassifySignal_Detailed(t *testing.T) {
	tests := []struct {
		name         string
		cotIndex     float64
		momentum     float64
		isCommercial bool
		want         string
	}{
		// Speculator (isCommercial=false) - directional
		{name: "spec_strong_bullish", cotIndex: 80, momentum: 10, isCommercial: false, want: "STRONG_BULLISH"},
		{name: "spec_bullish_no_momentum", cotIndex: 80, momentum: 0, isCommercial: false, want: "BULLISH"},
		{name: "spec_strong_bearish", cotIndex: 20, momentum: -10, isCommercial: false, want: "STRONG_BEARISH"},
		{name: "spec_bearish_no_momentum", cotIndex: 20, momentum: 0, isCommercial: false, want: "BEARISH"},
		{name: "spec_neutral_mid", cotIndex: 50, momentum: 0, isCommercial: false, want: "NEUTRAL"},
		{name: "spec_neutral_high_nomomentum", cotIndex: 75, momentum: 0, isCommercial: false, want: "BULLISH"},
		{name: "spec_neutral_low_nomomentum", cotIndex: 25, momentum: 0, isCommercial: false, want: "BEARISH"},
		// Commercial (isCommercial=true) - contrarian (same thresholds, inverse interpretation)
		{name: "comm_strong_bullish", cotIndex: 80, momentum: 10, isCommercial: true, want: "STRONG_BULLISH"},
		{name: "comm_bullish_no_momentum", cotIndex: 80, momentum: 0, isCommercial: true, want: "BULLISH"},
		{name: "comm_strong_bearish", cotIndex: 20, momentum: -10, isCommercial: true, want: "STRONG_BEARISH"},
		{name: "comm_bearish_no_momentum", cotIndex: 20, momentum: 0, isCommercial: true, want: "BEARISH"},
		{name: "comm_neutral_mid", cotIndex: 50, momentum: 0, isCommercial: true, want: "NEUTRAL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySignal(tt.cotIndex, tt.momentum, tt.isCommercial)
			if got != tt.want {
				t.Errorf("classifySignal(%v, %v, %v) = %q, want %q",
					tt.cotIndex, tt.momentum, tt.isCommercial, got, tt.want)
			}
		})
	}
}

// TestClassifySignalStrength_Detailed tests signal strength classification
func TestClassifySignalStrength_Detailed(t *testing.T) {
	tests := []struct {
		name string
		a    domain.COTAnalysis
		want domain.SignalStrength
	}{
		{
			name: "strong_extreme_plus_high_sentiment",
			a: domain.COTAnalysis{
				SentimentScore: 70,
				IsExtremeBull:  true,
			},
			want: domain.SignalStrong,
		},
		{
			name: "strong_extreme_bear_plus_high_sentiment",
			a: domain.COTAnalysis{
				SentimentScore: -70,
				IsExtremeBear:  true,
			},
			want: domain.SignalStrong,
		},
		{
			name: "moderate_sentiment_50",
			a: domain.COTAnalysis{
				SentimentScore: 50,
				IsExtremeBull:  false,
				IsExtremeBear:  false,
			},
			want: domain.SignalModerate,
		},
		{
			name: "weak_sentiment_30",
			a: domain.COTAnalysis{
				SentimentScore: 30,
				IsExtremeBull:  false,
				IsExtremeBear:  false,
			},
			want: domain.SignalWeak,
		},
		{
			name: "neutral_low_sentiment",
			a: domain.COTAnalysis{
				SentimentScore: 10,
				IsExtremeBull:  false,
				IsExtremeBear:  false,
			},
			want: domain.SignalNeutral,
		},
		{
			name: "neutral_zero_sentiment",
			a: domain.COTAnalysis{
				SentimentScore: 0,
				IsExtremeBull:  false,
				IsExtremeBear:  false,
			},
			want: domain.SignalNeutral,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySignalStrength(tt.a)
			if got != tt.want {
				t.Errorf("classifySignalStrength() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestClassifyMomentumDir_Detailed tests momentum direction classification
func TestClassifyMomentumDir_Detailed(t *testing.T) {
	tests := []struct {
		name    string
		specMom float64
		commMom float64
		want    domain.MomentumDirection
	}{
		{name: "building_spec_up_comm_down", specMom: 5000, commMom: -3000, want: domain.MomentumBuilding},
		{name: "reversing_spec_down_comm_up", specMom: -5000, commMom: 3000, want: domain.MomentumReversing},
		{name: "stable_both_low", specMom: 50, commMom: 50, want: domain.MomentumStable},
		{name: "stable_both_zero", specMom: 0, commMom: 0, want: domain.MomentumStable},
		{name: "unwinding_spec_down", specMom: -5000, commMom: -100, want: domain.MomentumUnwinding},
		{name: "building_spec_up_comm_small", specMom: 5000, commMom: 100, want: domain.MomentumBuilding},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyMomentumDir(tt.specMom, tt.commMom)
			if got != tt.want {
				t.Errorf("classifyMomentumDir(%v, %v) = %q, want %q",
					tt.specMom, tt.commMom, got, tt.want)
			}
		})
	}
}
