package sentiment

import "testing"

// TestContrarianSignal verifies the contrarian signal classification.
func TestContrarianSignal_Bullish(t *testing.T) {
	// Most retail short (30% long) → contrarian bullish
	result := ContrarianSignal(30.0)
	if result != "CONTRARIAN_BULLISH" {
		t.Errorf("expected CONTRARIAN_BULLISH for 30%% long, got %q", result)
	}
}

func TestContrarianSignal_Bearish(t *testing.T) {
	// Most retail long (70% long) → contrarian bearish
	result := ContrarianSignal(70.0)
	if result != "CONTRARIAN_BEARISH" {
		t.Errorf("expected CONTRARIAN_BEARISH for 70%% long, got %q", result)
	}
}

func TestContrarianSignal_Neutral(t *testing.T) {
	// Balanced (50% long) → neutral
	result := ContrarianSignal(50.0)
	if result != "NEUTRAL" {
		t.Errorf("expected NEUTRAL for 50%% long, got %q", result)
	}

	// Near boundary (64.9% long) → neutral
	result = ContrarianSignal(64.9)
	if result != "NEUTRAL" {
		t.Errorf("expected NEUTRAL for 64.9%% long, got %q", result)
	}
}

func TestContrarianSignal_Boundary(t *testing.T) {
	// Exactly 65.1% long → bearish (strictly > 65)
	result := ContrarianSignal(65.1)
	if result != "CONTRARIAN_BEARISH" {
		t.Errorf("expected CONTRARIAN_BEARISH at 65.1%% long, got %q", result)
	}

	// Exactly 34.9% long → bullish (strictly < 35)
	result = ContrarianSignal(34.9)
	if result != "CONTRARIAN_BULLISH" {
		t.Errorf("expected CONTRARIAN_BULLISH at 34.9%% long, got %q", result)
	}

	// Exactly 65.0% long → NEUTRAL (boundary is exclusive)
	result = ContrarianSignal(65.0)
	if result != "NEUTRAL" {
		t.Errorf("expected NEUTRAL at 65.0%% long (exclusive boundary), got %q", result)
	}
}

// TestMyfxbookData_Structure verifies struct fields are accessible.
func TestMyfxbookData_Structure(t *testing.T) {
	d := MyfxbookData{
		Pairs: []MyfxbookPairSentiment{
			{Symbol: "EURUSD", LongPct: 32.5, ShortPct: 67.5, Signal: "CONTRARIAN_BULLISH"},
		},
		Available: true,
	}
	if !d.Available {
		t.Error("expected Available=true")
	}
	if len(d.Pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(d.Pairs))
	}
	if d.Pairs[0].Symbol != "EURUSD" {
		t.Errorf("unexpected symbol: %q", d.Pairs[0].Symbol)
	}
	if d.Pairs[0].Signal != "CONTRARIAN_BULLISH" {
		t.Errorf("unexpected signal: %q", d.Pairs[0].Signal)
	}
}

// TestSentimentData_HasMyfxbookField verifies SentimentData has Myfxbook pointer.
func TestSentimentData_HasMyfxbookField(t *testing.T) {
	s := SentimentData{}
	if s.Myfxbook != nil {
		t.Error("expected nil Myfxbook by default")
	}
	s.Myfxbook = &MyfxbookData{Available: true}
	if s.Myfxbook == nil {
		t.Error("expected non-nil after assignment")
	}
}
