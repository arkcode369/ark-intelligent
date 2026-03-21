package cot_test

import (
	"context"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
)

// --- Mock SignalRepository ---

type mockSignalRepo struct {
	signals []domain.PersistedSignal
}

func (m *mockSignalRepo) SaveSignals(_ context.Context, _ []domain.PersistedSignal) error {
	return nil
}
func (m *mockSignalRepo) GetSignalsByContract(_ context.Context, _ string) ([]domain.PersistedSignal, error) {
	return m.signals, nil
}
func (m *mockSignalRepo) GetSignalsByType(_ context.Context, sigType string) ([]domain.PersistedSignal, error) {
	var out []domain.PersistedSignal
	for _, s := range m.signals {
		if s.SignalType == sigType {
			out = append(out, s)
		}
	}
	return out, nil
}
func (m *mockSignalRepo) GetAllSignals(_ context.Context) ([]domain.PersistedSignal, error) {
	return m.signals, nil
}
func (m *mockSignalRepo) GetPendingSignals(_ context.Context) ([]domain.PersistedSignal, error) {
	return nil, nil
}
func (m *mockSignalRepo) UpdateSignal(_ context.Context, _ domain.PersistedSignal) error {
	return nil
}

// buildEvaluatedSignal builds a signal with a given outcome for testing.
func buildEvaluatedSignal(sigType, outcome string) domain.PersistedSignal {
	return domain.PersistedSignal{
		ContractCode: "099741",
		Currency:     "EUR",
		SignalType:   sigType,
		Direction:    "BULLISH",
		Strength:     4,
		Confidence:   75,
		ReportDate:   time.Now().AddDate(0, 0, -14),
		EntryPrice:   1.0850,
		Outcome1W:    outcome,
		Return1W:     map[string]float64{"WIN": 0.5, "LOSS": -0.4}[outcome],
	}
}

// --- Tests ---

// TestRecalibratedDetector_NilRepo verifies nil-safe degradation.
func TestRecalibratedDetector_NilRepo(t *testing.T) {
	rd := cot.NewRecalibratedDetector(nil)
	err := rd.LoadTypeStats(context.Background())
	if err != nil {
		t.Fatalf("LoadTypeStats with nil repo should not error, got: %v", err)
	}
	// Stats should be nil — no crash
	stats := rd.TypeStats("SMART_MONEY")
	if stats != nil {
		t.Errorf("expected nil stats with nil repo, got %+v", stats)
	}
}

// TestLoadTypeStats_WinRateCalculation verifies win rate computation.
func TestLoadTypeStats_WinRateCalculation(t *testing.T) {
	// Build 15 signals: 10 wins + 5 losses for SMART_MONEY
	signals := make([]domain.PersistedSignal, 0, 15)
	for i := 0; i < 10; i++ {
		signals = append(signals, buildEvaluatedSignal("SMART_MONEY", domain.OutcomeWin))
	}
	for i := 0; i < 5; i++ {
		signals = append(signals, buildEvaluatedSignal("SMART_MONEY", domain.OutcomeLoss))
	}

	repo := &mockSignalRepo{signals: signals}
	rd := cot.NewRecalibratedDetector(repo)
	if err := rd.LoadTypeStats(context.Background()); err != nil {
		t.Fatalf("LoadTypeStats error: %v", err)
	}

	stats := rd.TypeStats("SMART_MONEY")
	if stats == nil {
		t.Fatal("expected stats for SMART_MONEY, got nil")
	}

	expectedWinRate := 66.67 // 10/15
	if stats.WinRate < 66.0 || stats.WinRate > 67.0 {
		t.Errorf("expected win rate ~66.67%%, got %.2f%%", stats.WinRate)
	}
	_ = expectedWinRate

	if stats.SampleSize != 15 {
		t.Errorf("expected sample size 15, got %d", stats.SampleSize)
	}
	if !stats.HasEdge {
		t.Error("expected HasEdge=true for 66.67% win rate")
	}
	if stats.Suppressed {
		t.Error("expected Suppressed=false for profitable signal")
	}
}

// TestLoadTypeStats_SuppressionThreshold verifies suppression logic.
func TestLoadTypeStats_SuppressionThreshold(t *testing.T) {
	tests := []struct {
		name       string
		wins       int
		losses     int
		wantSupp   bool
		wantEdge   bool
	}{
		// n=15, win=4 (26.7%) → suppressed
		{"low_winrate_large_n", 4, 11, true, false},
		// n=15, win=8 (53.3%) → has edge
		{"above_50_large_n", 8, 7, false, true},
		// n=8, win=2 (25%) → NOT suppressed (insufficient sample)
		{"low_winrate_small_n", 2, 6, false, false},
		// n=10 exactly (boundary), win=4 (40%) → suppressed
		{"boundary_n10_low", 4, 6, true, false},
		// n=9, win=3 (33%) → NOT suppressed (n<10)
		{"below_boundary_n9", 3, 6, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var signals []domain.PersistedSignal
			for i := 0; i < tt.wins; i++ {
				signals = append(signals, buildEvaluatedSignal("THIN_MARKET", domain.OutcomeWin))
			}
			for i := 0; i < tt.losses; i++ {
				signals = append(signals, buildEvaluatedSignal("THIN_MARKET", domain.OutcomeLoss))
			}

			repo := &mockSignalRepo{signals: signals}
			rd := cot.NewRecalibratedDetector(repo)
			_ = rd.LoadTypeStats(context.Background())

			stats := rd.TypeStats("THIN_MARKET")
			if stats == nil {
				t.Fatal("stats should not be nil")
			}

			if stats.Suppressed != tt.wantSupp {
				t.Errorf("Suppressed: want=%v got=%v (wins=%d losses=%d)",
					tt.wantSupp, stats.Suppressed, tt.wins, tt.losses)
			}
			if stats.HasEdge != tt.wantEdge {
				t.Errorf("HasEdge: want=%v got=%v", tt.wantEdge, stats.HasEdge)
			}
		})
	}
}

// TestRiskContext_ConfidenceAdjustment verifies VIX multiplier math.
func TestRiskContext_ConfidenceAdjustment(t *testing.T) {
	tests := []struct {
		name       string
		vix        float64
		baseConf   float64
		wantLow    float64
		wantHigh   float64
	}{
		{"panic_vix35", 35, 80, 50, 58},     // 80 * 0.70 = 56, ± margin
		{"elevated_vix25", 25, 80, 64, 70},  // 80 * 0.85 = 68
		{"normal_vix17", 17, 80, 76, 84},    // 80 * 1.00 = 80
		{"low_vix12", 12, 80, 88, 96},       // 80 * 1.15 = 92
		{"clamp_max", 10, 95, 95, 100},      // 95 * 1.15 = 109.25 → clamped to 100
		{"clamp_min", 40, 10, 5, 8},         // 10 * 0.70 = 7 → above min 0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &domain.RiskContext{
				VIXLevel: tt.vix,
				Regime:   domain.ClassifyRiskRegime(tt.vix),
			}
			adjusted := rc.AdjustConfidence(tt.baseConf)
			if adjusted < tt.wantLow || adjusted > tt.wantHigh {
				t.Errorf("AdjustConfidence(%.0f%%) with VIX=%.0f: want [%.0f, %.0f], got %.2f",
					tt.baseConf, tt.vix, tt.wantLow, tt.wantHigh, adjusted)
			}
		})
	}
}

// TestRiskContext_RegimeClassification verifies VIX regime buckets.
func TestRiskContext_RegimeClassification(t *testing.T) {
	cases := []struct {
		vix    float64
		regime domain.RiskRegime
	}{
		{35, domain.RiskRegimePanic},
		{30.1, domain.RiskRegimePanic},
		{30, domain.RiskRegimeElevated},
		{25, domain.RiskRegimeElevated},
		{20.1, domain.RiskRegimeElevated},
		{20, domain.RiskRegimeNormal},
		{17, domain.RiskRegimeNormal},
		{15.1, domain.RiskRegimeNormal},
		{15, domain.RiskRegimeLow},
		{12, domain.RiskRegimeLow},
	}
	for _, c := range cases {
		got := domain.ClassifyRiskRegime(c.vix)
		if got != c.regime {
			t.Errorf("VIX=%.1f: want %s, got %s", c.vix, c.regime, got)
		}
	}
}

// TestPriceMappings_VIXAndSPX verifies VIX/SPX are in DefaultPriceSymbolMappings.
func TestPriceMappings_VIXAndSPX(t *testing.T) {
	var foundVIX, foundSPX bool
	for _, m := range domain.DefaultPriceSymbolMappings {
		if m.Currency == "VIX" {
			foundVIX = true
			if !m.RiskOnly {
				t.Error("VIX mapping should have RiskOnly=true")
			}
			if m.Yahoo != "^VIX" {
				t.Errorf("VIX Yahoo symbol: want ^VIX, got %s", m.Yahoo)
			}
		}
		if m.Currency == "SPX" {
			foundSPX = true
			if !m.RiskOnly {
				t.Error("SPX mapping should have RiskOnly=true")
			}
			if m.Yahoo != "^GSPC" {
				t.Errorf("SPX Yahoo symbol: want ^GSPC, got %s", m.Yahoo)
			}
		}
	}
	if !foundVIX {
		t.Error("VIX not found in DefaultPriceSymbolMappings")
	}
	if !foundSPX {
		t.Error("SPX not found in DefaultPriceSymbolMappings")
	}
}

// TestCOTPriceMappings_ExcludesRiskOnly verifies COTPriceSymbolMappings() filters correctly.
func TestCOTPriceMappings_ExcludesRiskOnly(t *testing.T) {
	cotMappings := domain.COTPriceSymbolMappings()
	for _, m := range cotMappings {
		if m.RiskOnly {
			t.Errorf("COTPriceSymbolMappings() should not include RiskOnly mapping %s", m.Currency)
		}
	}
	// Should still have 11 COT contracts
	if len(cotMappings) != 11 {
		t.Errorf("expected 11 COT mappings, got %d", len(cotMappings))
	}
}

// TestSuppressedSignal_DroppedFromOutput verifies suppressed signals don't reach output.
func TestSuppressedSignal_DroppedFromOutput(t *testing.T) {
	// Build 10 LOSS signals for THIN_MARKET → should be suppressed
	var signals []domain.PersistedSignal
	for i := 0; i < 10; i++ {
		signals = append(signals, buildEvaluatedSignal("THIN_MARKET", domain.OutcomeLoss))
	}
	// Build 10 WIN signals for SMART_MONEY → should pass through
	for i := 0; i < 10; i++ {
		signals = append(signals, buildEvaluatedSignal("SMART_MONEY", domain.OutcomeWin))
	}

	repo := &mockSignalRepo{signals: signals}
	rd := cot.NewRecalibratedDetector(repo)
	_ = rd.LoadTypeStats(context.Background())

	suppressed := rd.SuppressedTypes()
	foundThinMarket := false
	for _, s := range suppressed {
		if s == "THIN_MARKET" {
			foundThinMarket = true
		}
	}
	if !foundThinMarket {
		t.Errorf("expected THIN_MARKET in suppressed types, got %v", suppressed)
	}

	foundSmartMoney := false
	for _, s := range suppressed {
		if s == "SMART_MONEY" {
			foundSmartMoney = true
		}
	}
	if foundSmartMoney {
		t.Error("SMART_MONEY should NOT be suppressed (100% win rate)")
	}
}
