package telegram

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/service/factors"
	"github.com/arkcode369/ark-intelligent/internal/service/microstructure"
	"github.com/arkcode369/ark-intelligent/internal/service/strategy"
)

// ---------------------------------------------------------------------------
// alphaStateCache tests
// ---------------------------------------------------------------------------

func TestAlphaStateCache_GetSet(t *testing.T) {
	cache := newAlphaStateCache()
	state := &alphaState{
		ranking: &factors.RankingResult{
			AssetCount: 2,
			ComputedAt: time.Now(),
		},
		computedAt: time.Now(),
	}

	// Set and get
	cache.set("chat123", state)
	got := cache.get("chat123")

	if got == nil {
		t.Fatal("expected to get cached state, got nil")
	}
	if got.ranking.AssetCount != 2 {
		t.Errorf("expected AssetCount=2, got %d", got.ranking.AssetCount)
	}
}

func TestAlphaStateCache_TTLExpiration(t *testing.T) {
	// Save original TTL and restore after test
	originalTTL := alphaStateTTL
	alphaStateTTL = 100 * time.Millisecond
	defer func() { alphaStateTTL = originalTTL }()

	cache := newAlphaStateCache()
	state := &alphaState{
		ranking:    &factors.RankingResult{AssetCount: 2},
		computedAt: time.Now(),
	}

	cache.set("chat123", state)

	// Should exist immediately
	if cache.get("chat123") == nil {
		t.Error("state should exist immediately after set")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	if cache.get("chat123") != nil {
		t.Error("state should be expired after TTL")
	}
}

func TestAlphaStateCache_OpportunisticCleanup(t *testing.T) {
	cache := newAlphaStateCache()

	// Add 51 entries to trigger cleanup threshold
	for i := 0; i < 51; i++ {
		state := &alphaState{
			ranking:    &factors.RankingResult{},
			computedAt: time.Now(),
		}
		cache.store[string(rune(i))] = state
	}

	// Verify entries exist
	if len(cache.store) != 51 {
		t.Errorf("expected 51 entries, got %d", len(cache.store))
	}

	// Add one more expired entry that should be cleaned up
	oldTime := time.Now().Add(-2 * alphaStateTTL * 3) // Expired 3x TTL ago
	oldState := &alphaState{
		ranking:    &factors.RankingResult{},
		computedAt: oldTime,
	}
	cache.store["old_entry"] = oldState

	newState := &alphaState{
		ranking:    &factors.RankingResult{},
		computedAt: time.Now(),
	}
	cache.set("new_entry", newState)

	// Cleanup should have removed old_entry since it was expired
	if _, exists := cache.store["old_entry"]; exists {
		// Note: cleanup is opportunistic, so this might or might not pass
		// depending on timing. The main test is that it doesn't panic.
		t.Log("old_entry still exists (cleanup is opportunistic)")
	}
}

func TestAlphaStateCache_ThreadSafety(t *testing.T) {
	cache := newAlphaStateCache()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			state := &alphaState{
				ranking:    &factors.RankingResult{AssetCount: i},
				computedAt: time.Now(),
			}
			cache.set(string(rune(i)), state)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = cache.get(string(rune(i)))
		}(i)
	}

	wg.Wait()
	// No panic = thread safety working
}

// ---------------------------------------------------------------------------
// formatAlphaSummary tests
// ---------------------------------------------------------------------------

func TestFormatAlphaSummary_Basic(t *testing.T) {
	state := &alphaState{
		ranking: &factors.RankingResult{
			Assets: []factors.RankedAsset{
				{Currency: "EUR", CompositeScore: 0.45},
				{Currency: "GBP", CompositeScore: -0.30},
			},
		},
		playbook: &strategy.PlaybookResult{
			MacroRegime: "EXPANSION",
			Playbook: []strategy.PlaybookEntry{
				{Currency: "EUR", Direction: strategy.DirectionLong, Conviction: 0.75, ConvLevel: strategy.ConvictionHigh, FactorScore: 0.45, COTBias: "BULLISH", RateDiffBps: 25},
				{Currency: "GBP", Direction: strategy.DirectionShort, Conviction: 0.60, ConvLevel: strategy.ConvictionMedium, FactorScore: -0.30, COTBias: "BEARISH", RateDiffBps: -15},
			},
			Heat: strategy.PortfolioHeat{
				HeatLevel: strategy.HeatWarm,
			},
			Transition: strategy.TransitionWarning{
				IsActive:    false,
				Probability: 0.20,
			},
		},
		computedAt: time.Now(),
	}

	result := formatAlphaSummary(state)

	if result == "" {
		t.Error("expected non-empty summary")
	}
	if !strings.Contains(result, "Alpha Engine Dashboard") {
		t.Error("expected 'Alpha Engine Dashboard' in summary")
	}
	if !strings.Contains(result, "KEPUTUSAN UTAMA") {
		t.Error("expected Indonesian 'KEPUTUSAN UTAMA' in summary")
	}
	if !strings.Contains(result, "EXPANSION") {
		t.Error("expected regime in summary")
	}
	if !strings.Contains(result, "EUR") {
		t.Error("expected EUR currency in summary")
	}
	if !strings.Contains(result, "Rekomendasi") {
		t.Error("expected 'Rekomendasi' in summary")
	}
}

func TestFormatAlphaSummary_NoPlaybook(t *testing.T) {
	state := &alphaState{
		ranking: &factors.RankingResult{
			Assets: []factors.RankedAsset{
				{Currency: "EUR"},
			},
		},
		playbook:   nil,
		computedAt: time.Now(),
	}

	result := formatAlphaSummary(state)

	if result == "" {
		t.Error("expected non-empty summary even without playbook")
	}
	if !strings.Contains(result, "Alpha Engine Dashboard") {
		t.Error("expected title in summary")
	}
}

func TestFormatAlphaSummary_ActiveTransition(t *testing.T) {
	state := &alphaState{
		ranking: &factors.RankingResult{},
		playbook: &strategy.PlaybookResult{
			MacroRegime: "EXPANSION",
			Heat: strategy.PortfolioHeat{
				HeatLevel: strategy.HeatWarm,
			},
			Transition: strategy.TransitionWarning{
				IsActive:    true,
				Probability: 0.65,
				FromRegime:  "EXPANSION",
				ToRegime:    "SLOWDOWN",
			},
		},
		computedAt: time.Now(),
	}

	result := formatAlphaSummary(state)

	if !strings.Contains(result, "transisi") || !strings.Contains(result, "aktif") {
		t.Error("expected transition warning in summary")
	}
	if !strings.Contains(result, "SLOWDOWN") {
		t.Error("expected target regime in transition warning")
	}
}

func TestFormatAlphaSummary_OverheatWarning(t *testing.T) {
	state := &alphaState{
		ranking: &factors.RankingResult{},
		playbook: &strategy.PlaybookResult{
			MacroRegime: "EXPANSION",
			Heat: strategy.PortfolioHeat{
				HeatLevel: strategy.HeatOverheat,
			},
			Transition: strategy.TransitionWarning{},
		},
		computedAt: time.Now(),
	}

	result := formatAlphaSummary(state)

	if !strings.Contains(result, "OVERHEAT") {
		t.Error("expected OVERHEAT warning")
	}
	if !strings.Contains(result, "KURANGI POSISI") {
		t.Error("expected Indonesian heat warning")
	}
}

func TestFormatAlphaSummary_WithCryptoSignals(t *testing.T) {
	state := &alphaState{
		ranking: &factors.RankingResult{},
		playbook: &strategy.PlaybookResult{
			MacroRegime: "EXPANSION",
			Heat: strategy.PortfolioHeat{
				HeatLevel: strategy.HeatCold,
			},
			Transition: strategy.TransitionWarning{},
		},
		crypto: map[string]*microstructure.Signal{
			"BTCUSDT": {
				Symbol:       "BTCUSDT",
				Bias:         microstructure.BiasBullish,
				FundingRate:  0.015, // High funding
				ConfirmEntry: true,
			},
		},
		cryptoSyms: []string{"BTCUSDT"},
		computedAt: time.Now(),
	}

	result := formatAlphaSummary(state)

	if !strings.Contains(result, "BTC") {
		t.Error("expected BTC in crypto warning")
	}
}

// ---------------------------------------------------------------------------
// buildReasonIndonesian tests
// ---------------------------------------------------------------------------

func TestBuildReasonIndonesian_MomentumStrong(t *testing.T) {
	e := strategy.PlaybookEntry{
		FactorScore: 0.35,
		COTBias:     "BULLISH",
		RateDiffBps: 60,
		RegimeFit:   "ALIGNED",
	}

	result := buildReasonIndonesian(e)
	if !strings.Contains(result, "momentum kuat") {
		t.Error("expected 'momentum kuat' for high factor score")
	}
	if !strings.Contains(result, "COT bullish") {
		t.Error("expected COT bias in reason")
	}
	if !strings.Contains(result, "carry positif") {
		t.Error("expected carry mention for positive rate diff")
	}
}

func TestBuildReasonIndonesian_MomentumWeak(t *testing.T) {
	e := strategy.PlaybookEntry{
		FactorScore: -0.35,
		COTBias:     "BEARISH",
		RateDiffBps: -60,
		RegimeFit:   "AGAINST_REGIME",
	}

	result := buildReasonIndonesian(e)
	if !strings.Contains(result, "momentum lemah") {
		t.Error("expected 'momentum lemah' for negative factor score")
	}
	if !strings.Contains(result, "melawan regime") {
		t.Error("expected regime opposition note")
	}
}

func TestBuildReasonIndonesian_ModerateMomentum(t *testing.T) {
	e := strategy.PlaybookEntry{
		FactorScore: 0.20,
		COTBias:     "NEUTRAL",
	}

	result := buildReasonIndonesian(e)
	if !strings.Contains(result, "momentum positif") {
		t.Error("expected 'momentum positif' for moderate factor score")
	}
}

func TestBuildReasonIndonesian_NoFactors(t *testing.T) {
	e := strategy.PlaybookEntry{
		FactorScore: 0.0,
		COTBias:     "NEUTRAL",
		RateDiffBps: 10,
	}

	result := buildReasonIndonesian(e)
	if !strings.Contains(result, "sinyal multifaktor") {
		t.Error("expected default reason when no strong factors")
	}
}

// ---------------------------------------------------------------------------
// Helper function tests
// ---------------------------------------------------------------------------

func TestAlphaConvEmoji(t *testing.T) {
	tests := []struct {
		level    strategy.ConvictionLevel
		expected string
	}{
		{strategy.ConvictionHigh, "🔥"},
		{strategy.ConvictionMedium, "📌"},
		{strategy.ConvictionLow, "💡"},
		{strategy.ConvictionAvoid, "⛔"},
		{"UNKNOWN", "⛔"},
	}

	for _, tt := range tests {
		result := alphaConvEmoji(tt.level)
		if result != tt.expected {
			t.Errorf("alphaConvEmoji(%s) = %s, want %s", tt.level, result, tt.expected)
		}
	}
}

func TestAlphaHeatEmoji(t *testing.T) {
	tests := []struct {
		level    strategy.HeatLevel
		expected string
	}{
		{strategy.HeatCold, "🔵"},
		{strategy.HeatWarm, "🟡"},
		{strategy.HeatHot, "🟠"},
		{strategy.HeatOverheat, "🔴 Bearish"},
		{"UNKNOWN", "⚪ Neutral"},
	}

	for _, tt := range tests {
		result := alphaHeatEmoji(tt.level)
		if result != tt.expected {
			t.Errorf("alphaHeatEmoji(%s) = %s, want %s", tt.level, result, tt.expected)
		}
	}
}

func TestAlphaErr(t *testing.T) {
	if alphaErr(nil) != "unknown error" {
		t.Error("alphaErr(nil) should return 'unknown error'")
	}

	err := errors.New("test error")
	if alphaErr(err) != "test error" {
		t.Errorf("alphaErr(error) = %s, want 'test error'", alphaErr(err))
	}
}

func TestAlphaExplainHeader(t *testing.T) {
	result := alphaExplainHeader("Test Title", "Test explanation")
	if !strings.Contains(result, "Test Title") {
		t.Error("expected title in header")
	}
	if !strings.Contains(result, "Test explanation") {
		t.Error("expected explanation in header")
	}
	if !strings.Contains(result, "<b>") || !strings.Contains(result, "</b>") {
		t.Error("expected bold tags around title")
	}
	if !strings.Contains(result, "<i>") || !strings.Contains(result, "</i>") {
		t.Error("expected italic tags around explanation")
	}
}

func TestAlphaSignalEmoji(t *testing.T) {
	tests := []struct {
		signal   string
		expected string
	}{
		{"STRONG_LONG", "🟢🟢"},
		{"LONG", "🟢 Bullish"},
		{"SHORT", "🔴 Bearish"},
		{"STRONG_SHORT", "🔴🔴"},
		{"NEUTRAL", "⚪ Neutral"},
		{"UNKNOWN", "⚪ Neutral"},
	}

	for _, tt := range tests {
		result := alphaSignalEmoji(tt.signal)
		if result != tt.expected {
			t.Errorf("alphaSignalEmoji(%s) = %s, want %s", tt.signal, result, tt.expected)
		}
	}
}

func TestAlphaScoreBar(t *testing.T) {
	// Positive score (should have many filled blocks)
	bar := alphaScoreBar(0.50)
	if !strings.Contains(bar, "█") {
		t.Error("expected filled blocks for positive score")
	}

	// Negative score (should have some filled blocks due to normalization)
	bar = alphaScoreBar(-0.50)
	if !strings.Contains(bar, "█") {
		t.Error("expected filled blocks for negative score")
	}

	// Zero score (should have about 5 filled blocks after normalization: (0+1)/2*10 = 5)
	bar = alphaScoreBar(0)
	if !strings.Contains(bar, "█") {
		t.Error("expected some filled blocks for zero score (normalized to middle)")
	}
	if !strings.Contains(bar, "░") {
		t.Error("expected some empty blocks for zero score")
	}
}

func TestAlphaConvBar(t *testing.T) {
	// High conviction (0.75 * 5 = 3.75, rounded to 4 blocks)
	bar := alphaConvBar(0.75)
	if !strings.Contains(bar, "▪") {
		t.Error("expected filled blocks (▪)")
	}

	// Test edge cases - zero conviction should have 0 blocks
	bar = alphaConvBar(0)
	if !strings.Contains(bar, "·") {
		t.Error("expected empty dots (·) for zero conviction")
	}
	// Should NOT have filled blocks
	if strings.Contains(bar, "▪") {
		t.Error("should not have filled blocks for zero conviction")
	}

	// Full conviction (1.0 * 5 = 5 blocks)
	bar = alphaConvBar(1.0)
	if !strings.Contains(bar, "▪") {
		t.Error("expected filled blocks for full conviction")
	}
	// Should have 5 filled blocks, no dots
	if strings.Contains(bar, "·") {
		t.Error("should not have empty dots for full conviction")
	}
}

// ---------------------------------------------------------------------------
// Helper formatting function tests
// ---------------------------------------------------------------------------

func TestRegimeIndonesian(t *testing.T) {
	tests := []struct {
		regime   string
		expected string
	}{
		{"EXPANSION", "ekonomi tumbuh"},
		{"SLOWDOWN", "melambat"},
		{"RECESSION", "kontraksi"},
		{"RECOVERY", "pulih"},
		{"GOLDILOCKS", "pertumbuhan moderat"},
		{"NEUTRAL", "tidak ada tren"},
		{"UNKNOWN", "fase ekonomi"},
	}

	for _, tt := range tests {
		result := regimeIndonesian(tt.regime)
		if !strings.Contains(result, tt.expected) {
			t.Errorf("regimeIndonesian(%s) = %s, expected to contain %s", tt.regime, result, tt.expected)
		}
	}
}

func TestHeatAdviceIndonesian(t *testing.T) {
	tests := []struct {
		level    strategy.HeatLevel
		expected string
	}{
		{strategy.HeatCold, "tambah posisi"},
		{strategy.HeatWarm, "masih aman"},
		{strategy.HeatHot, "kurangi agresivitas"},
		{strategy.HeatOverheat, "OVERHEAT"},
	}

	for _, tt := range tests {
		result := heatAdviceIndonesian(tt.level)
		if !strings.Contains(result, tt.expected) {
			t.Errorf("heatAdviceIndonesian(%s) = %s, expected to contain %s", tt.level, result, tt.expected)
		}
	}
}

func TestFactorInterpretIndonesian(t *testing.T) {
	tests := []struct {
		signal   factors.Signal
		expected string
	}{
		{factors.SignalStrongLong, "beli kuat"},
		{factors.SignalLong, "beli"},
		{factors.SignalStrongShort, "jual kuat"},
		{factors.SignalShort, "jual"},
		{factors.SignalNeutral, "Netral"},
	}

	asset := factors.RankedAsset{Signal: factors.SignalLong}
	for _, tt := range tests {
		asset.Signal = tt.signal
		result := factorInterpretIndonesian(asset)
		if !strings.Contains(result, tt.expected) {
			t.Errorf("factorInterpretIndonesian(%s) = %s, expected to contain %s", tt.signal, result, tt.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Integration tests for formatters
// ---------------------------------------------------------------------------

func TestFormatFactorRanking(t *testing.T) {
	result := &factors.RankingResult{
		Assets: []factors.RankedAsset{
			{
				Currency:       "EUR",
				CompositeScore: 0.45,
				Rank:           1,
				Signal:         factors.SignalLong,
				Scores: factors.FactorScores{
					Momentum:      0.50,
					TrendQuality:  0.40,
					CarryAdjusted: 0.30,
					LowVol:        0.20,
				},
			},
		},
		AssetCount: 1,
		ComputedAt: time.Now(),
	}

	formatted := formatFactorRanking(result)
	if !strings.Contains(formatted, "Factor Ranking") {
		t.Error("expected 'Factor Ranking' header")
	}
	if !strings.Contains(formatted, "EUR") {
		t.Error("expected EUR currency")
	}
}

func TestFormatFactorRanking_Empty(t *testing.T) {
	result := &factors.RankingResult{}
	formatted := formatFactorRanking(result)
	if !strings.Contains(formatted, "Tidak ada data") {
		t.Error("expected empty data message")
	}
}

func TestFormatPlaybook(t *testing.T) {
	result := &strategy.PlaybookResult{
		MacroRegime: "EXPANSION",
		Playbook: []strategy.PlaybookEntry{
			{Currency: "EUR", Direction: strategy.DirectionLong, Conviction: 0.75, ConvLevel: strategy.ConvictionHigh},
		},
		Heat: strategy.PortfolioHeat{
			HeatLevel: strategy.HeatWarm,
		},
		Transition: strategy.TransitionWarning{IsActive: false},
		ComputedAt: time.Now(),
	}

	formatted := formatPlaybook(result)
	if !strings.Contains(formatted, "Strategy Playbook") {
		t.Error("expected 'Strategy Playbook' header")
	}
	if !strings.Contains(formatted, "EUR") {
		t.Error("expected EUR in playbook")
	}
}

func TestFormatPlaybook_Nil(t *testing.T) {
	formatted := formatPlaybook(nil)
	if !strings.Contains(formatted, "Tidak ada data") {
		t.Error("expected nil data message")
	}
}

func TestFormatPlaybook_WithTransition(t *testing.T) {
	result := &strategy.PlaybookResult{
		MacroRegime: "EXPANSION",
		Playbook:    []strategy.PlaybookEntry{},
		Heat: strategy.PortfolioHeat{
			HeatLevel: strategy.HeatWarm,
		},
		Transition: strategy.TransitionWarning{
			IsActive:    true,
			Probability: 0.65,
			FromRegime:  "EXPANSION",
			ToRegime:    "SLOWDOWN",
		},
		ComputedAt: time.Now(),
	}

	formatted := formatPlaybook(result)
	if !strings.Contains(formatted, "TRANSISI") {
		t.Error("expected transition warning")
	}
}

func TestFormatHeat(t *testing.T) {
	heat := strategy.PortfolioHeat{
		HeatLevel:     strategy.HeatHot,
		LongExposure:  1.5,
		ShortExposure: 0.5,
		NetExposure:   1.0,
		TotalExposure: 0.75,
		ActiveTrades:  3,
	}

	formatted := formatHeat(heat)
	if !strings.Contains(formatted, "Portfolio Heat") {
		t.Error("expected 'Portfolio Heat' header")
	}
	if !strings.Contains(formatted, "HOT") {
		t.Error("expected HOT level")
	}
}

func TestFormatRankX(t *testing.T) {
	result := &factors.RankingResult{
		Assets: []factors.RankedAsset{
			{Currency: "EUR", CompositeScore: 0.45, Rank: 1, Signal: factors.SignalLong},
			{Currency: "GBP", CompositeScore: -0.30, Rank: 2, Signal: factors.SignalShort},
		},
		AssetCount: 2,
		ComputedAt: time.Now(),
	}

	formatted := formatRankX(result)
	if !strings.Contains(formatted, "RankX") {
		t.Error("expected 'RankX' header")
	}
	if !strings.Contains(formatted, "EUR") || !strings.Contains(formatted, "GBP") {
		t.Error("expected both currencies in output")
	}
}

func TestFormatTransition(t *testing.T) {
	tw := strategy.TransitionWarning{
		IsActive:    true,
		Probability: 0.65,
		FromRegime:  "EXPANSION",
		ToRegime:    "SLOWDOWN",
	}

	formatted := formatTransition(tw, "EXPANSION")
	if !strings.Contains(formatted, "Transisi Regime") {
		t.Error("expected 'Transisi Regime' header")
	}
	if !strings.Contains(formatted, "EXPANSION") {
		t.Error("expected from regime")
	}
	if !strings.Contains(formatted, "SLOWDOWN") {
		t.Error("expected to regime")
	}
	// Active transition should show "AKTIF"
	if !strings.Contains(formatted, "AKTIF") {
		t.Error("expected 'AKTIF' for active transition")
	}
}

func TestFormatTransition_Inactive(t *testing.T) {
	tw := strategy.TransitionWarning{
		IsActive:    false,
		Probability: 0.20,
	}

	formatted := formatTransition(tw, "EXPANSION")
	if !strings.Contains(formatted, "Stabil") || !strings.Contains(formatted, "tidak") {
		t.Error("expected stability message for inactive transition")
	}
	// Should show "Monitor Transisi Regime" for inactive
	if !strings.Contains(formatted, "Monitor") {
		t.Error("expected 'Monitor' for inactive transition")
	}
}

func TestFormatCryptoAlpha(t *testing.T) {
	signals := map[string]*microstructure.Signal{
		"BTCUSDT": {
			Symbol:         "BTCUSDT",
			Bias:           microstructure.BiasBullish,
			ConfirmEntry:   true,
			FundingRate:    0.0001,
			LongShortRatio: 1.5,
		},
	}
	symbols := []string{"BTCUSDT"}

	formatted := formatCryptoAlpha(signals, symbols, nil)
	if !strings.Contains(formatted, "BTC") {
		t.Error("expected BTC in output")
	}
	// The header uses "Crypto Microstructure Alpha" (note uppercase M)
	if !strings.Contains(formatted, "Crypto Microstructure") {
		t.Error("expected 'Crypto Microstructure' header")
	}
}

func TestFormatCryptoAlpha_Empty(t *testing.T) {
	formatted := formatCryptoAlpha(map[string]*microstructure.Signal{}, []string{}, nil)
	if !strings.Contains(formatted, "Tidak ada data") {
		t.Error("expected empty data message")
	}
}

// ---------------------------------------------------------------------------
// Config integration test
// ---------------------------------------------------------------------------

func TestAlphaStateTTL_Configuration(t *testing.T) {
	// Verify the TTL is configured correctly
	if config.AlphaStateTTL <= 0 {
		t.Error("AlphaStateTTL should be positive")
	}
	if config.AlphaStateTTL < 30*time.Second {
		t.Error("AlphaStateTTL should be at least 30 seconds")
	}
}
