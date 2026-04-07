package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/factors"
	"github.com/arkcode369/ark-intelligent/internal/service/microstructure"
	"github.com/arkcode369/ark-intelligent/internal/service/strategy"
)

// ---------------------------------------------------------------------------
// Alpha State Cache Tests
// ---------------------------------------------------------------------------

func TestNewAlphaStateCache(t *testing.T) {
	cache := newAlphaStateCache()
	if cache == nil {
		t.Fatal("expected cache to be initialized")
	}
	if cache.store == nil {
		t.Fatal("expected store map to be initialized")
	}
}

func TestAlphaStateCache_SetAndGet(t *testing.T) {
	cache := newAlphaStateCache()
	state := &alphaState{
		ranking:    &factors.RankingResult{},
		computedAt: time.Now(),
	}

	// Set state
	cache.set("chat123", state)

	// Get existing state
	got := cache.get("chat123")
	if got == nil {
		t.Fatal("expected to get cached state")
	}
	if got.ranking == nil {
		t.Error("expected ranking to be set")
	}
}

func TestAlphaStateCache_GetNonExistent(t *testing.T) {
	cache := newAlphaStateCache()
	got := cache.get("nonexistent")
	if got != nil {
		t.Error("expected nil for non-existent chatID")
	}
}

func TestAlphaStateCache_TTLExpiration(t *testing.T) {
	// Save original TTL and restore after test
	originalTTL := alphaStateTTL
	alphaStateTTL = 1 * time.Millisecond
	defer func() { alphaStateTTL = originalTTL }()

	cache := newAlphaStateCache()
	state := &alphaState{
		ranking:    &factors.RankingResult{},
		computedAt: time.Now(),
	}

	cache.set("chat123", state)

	// Wait for TTL to expire
	time.Sleep(5 * time.Millisecond)

	got := cache.get("chat123")
	if got != nil {
		t.Error("expected nil after TTL expiration")
	}
}

func TestAlphaStateCache_Cleanup(t *testing.T) {
	cache := newAlphaStateCache()

	// Add many entries to trigger cleanup
	for i := 0; i < 55; i++ {
		state := &alphaState{
			ranking:    &factors.RankingResult{},
			computedAt: time.Now(),
		}
		cache.set(string(rune('a'+i)), state)
	}

	// Verify we can still get recent entries
	recent := cache.get(string(rune('a' + 54)))
	if recent == nil {
		t.Error("expected to get recent entry after cleanup")
	}
}

// ---------------------------------------------------------------------------
// Pure Function Tests — Formatters
// ---------------------------------------------------------------------------

func TestAlphaExplainHeader(t *testing.T) {
	title := "Test Title"
	explanation := "Test explanation text"

	got := alphaExplainHeader(title, explanation)

	if got == "" {
		t.Error("expected non-empty header")
	}
	// Check that it contains the title
	if !strings.Contains(got, title) {
		t.Errorf("expected header to contain title %q, got %q", title, got)
	}
	// Check that it contains the explanation
	if !strings.Contains(got, explanation) {
		t.Errorf("expected header to contain explanation %q, got %q", explanation, got)
	}
}

func TestRegimeIndonesian(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"EXPANSION", "ekonomi tumbuh, risk-on"},
		{"SLOWDOWN", "ekonomi melambat, hati-hati"},
		{"RECESSION", "kontraksi ekonomi, risk-off"},
		{"RECOVERY", "ekonomi pulih, awal risk-on"},
		{"GOLDILOCKS", "pertumbuhan moderat, inflasi terkendali"},
		{"NEUTRAL", "tidak ada tren makro dominan"},
		{"unknown", "fase ekonomi saat ini"},
		{"", "fase ekonomi saat ini"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := regimeIndonesian(tt.input)
			if got != tt.expected {
				t.Errorf("regimeIndonesian(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestHeatAdviceIndonesian(t *testing.T) {
	tests := []struct {
		name     string
		input    strategy.HeatLevel
		expected string
	}{
		{"cold", strategy.HeatCold, "Eksposur rendah — aman untuk tambah posisi baru"},
		{"warm", strategy.HeatWarm, "Eksposur sedang — masih aman, jangan terlalu agresif"},
		{"hot", strategy.HeatHot, "Eksposur tinggi — kurangi agresivitas, pertimbangkan take profit"},
		{"overheat", strategy.HeatOverheat, "⚠️ OVERHEAT — segera kurangi posisi!"},
		{"unknown", strategy.HeatLevel("UNKNOWN"), "Evaluasi eksposur portfolio"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := heatAdviceIndonesian(tt.input)
			if got != tt.expected {
				t.Errorf("heatAdviceIndonesian(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAlphaSignalEmoji(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"STRONG_LONG", "🟢🟢"},
		{"LONG", "🟢 Bullish"},
		{"STRONG_SHORT", "🔴🔴"},
		{"SHORT", "🔴 Bearish"},
		{"NEUTRAL", "⚪ Neutral"},
		{"UNKNOWN", "⚪ Neutral"},
		{"", "⚪ Neutral"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := alphaSignalEmoji(tt.input)
			if got != tt.expected {
				t.Errorf("alphaSignalEmoji(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestHeatBar(t *testing.T) {
	tests := []struct {
		pct             float64
		thresholdPerPos float64
	}{
		{0, 10},
		{50, 10},
		{100, 10},
		{25, 5},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.pct)), func(t *testing.T) {
			got := heatBar(tt.pct, tt.thresholdPerPos)
			if len(got) == 0 {
				t.Error("expected non-empty heat bar")
			}
		})
	}
}

func TestBuildReasonIndonesian(t *testing.T) {
	tests := []struct {
		name string
		entry strategy.PlaybookEntry
	}{
		{
			name: "bullish momentum",
			entry: strategy.PlaybookEntry{
				Direction: strategy.DirectionLong,
				FactorScore: 0.6,
			},
		},
		{
			name: "bearish cot",
			entry: strategy.PlaybookEntry{
				Direction: strategy.DirectionShort,
				FactorScore: -0.4,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildReasonIndonesian(tt.entry)
			if got == "" {
				t.Error("expected non-empty reason string")
			}
		})
	}
}

func TestFactorInterpretIndonesian(t *testing.T) {
	tests := []struct {
		name  string
		asset factors.RankedAsset
	}{
		{
			name: "strong bullish",
			asset: factors.RankedAsset{
				CompositeScore: 0.6,
			},
		},
		{
			name: "strong bearish",
			asset: factors.RankedAsset{
				CompositeScore: -0.6,
			},
		},
		{
			name: "neutral",
			asset: factors.RankedAsset{
				CompositeScore: 0.05,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := factorInterpretIndonesian(tt.asset)
			if got == "" {
				t.Error("expected non-empty interpretation")
			}
		})
	}
}

func TestCryptoInterpretIndonesian(t *testing.T) {
	tests := []struct {
		name   string
		signal *microstructure.Signal
	}{
		{
			name: "bullish signal",
			signal: &microstructure.Signal{
				Bias:         microstructure.BiasBullish,
				Strength:     0.8,
				FundingRate:  -0.001,
			},
		},
		{
			name: "bearish signal",
			signal: &microstructure.Signal{
				Bias:         microstructure.BiasBearish,
				Strength:     0.6,
				FundingRate:  0.001,
			},
		},
		{
			name: "neutral signal",
			signal: &microstructure.Signal{
				Bias:         microstructure.BiasNeutral,
				Strength:     0.3,
				FundingRate:  0.0001,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cryptoInterpretIndonesian(tt.signal)
			if got == "" {
				t.Error("expected non-empty interpretation")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AlphaConvEmoji Helper
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
		t.Run(string(tt.level), func(t *testing.T) {
			got := alphaConvEmoji(tt.level)
			if got != tt.expected {
				t.Errorf("alphaConvEmoji(%q) = %q, want %q", tt.level, got, tt.expected)
			}
		})
	}
}
