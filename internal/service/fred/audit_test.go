package fred

import (
	"math"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ==========================================================================
// RATE DIFFERENTIAL AUDIT
// ==========================================================================

func TestCarryAdjustment_Audit_AlignedBullish(t *testing.T) {
	diff := domain.RateDifferential{Differential: 2.0}
	adj := CarryAdjustment(diff, "BULLISH")
	// Positive carry + BULLISH → boost
	if adj <= 0 {
		t.Errorf("Expected positive adjustment for aligned bullish carry, got %f", adj)
	}
	if adj > 5 {
		t.Errorf("Adjustment capped at 5, got %f", adj)
	}
	// Expected: min(2.0*2, 5) = 4.0
	if math.Abs(adj-4.0) > 0.001 {
		t.Errorf("Expected 4.0, got %f", adj)
	}
}

func TestCarryAdjustment_Audit_AlignedBearish(t *testing.T) {
	diff := domain.RateDifferential{Differential: -2.0}
	adj := CarryAdjustment(diff, "BEARISH")
	// Negative carry + BEARISH → boost
	if adj <= 0 {
		t.Errorf("Expected positive adjustment for aligned bearish carry, got %f", adj)
	}
	// Expected: min(abs(-2)*2, 5) = 4.0
	if math.Abs(adj-4.0) > 0.001 {
		t.Errorf("Expected 4.0, got %f", adj)
	}
}

func TestCarryAdjustment_Audit_OpposedBullish(t *testing.T) {
	diff := domain.RateDifferential{Differential: -2.0}
	adj := CarryAdjustment(diff, "BULLISH")
	// Negative carry + BULLISH → penalty
	if adj >= 0 {
		t.Errorf("Expected negative adjustment for opposed bullish, got %f", adj)
	}
	// Expected: max(-2*1.5, -5) = max(-3, -5) = -3
	if math.Abs(adj-(-3.0)) > 0.001 {
		t.Errorf("Expected -3.0, got %f", adj)
	}
}

func TestCarryAdjustment_Audit_OpposedBearish(t *testing.T) {
	diff := domain.RateDifferential{Differential: 2.0}
	adj := CarryAdjustment(diff, "BEARISH")
	// Positive carry + BEARISH → penalty
	if adj >= 0 {
		t.Errorf("Expected negative adjustment for opposed bearish, got %f", adj)
	}
	// Expected: -min(2*1.5, 5) = -3
	if math.Abs(adj-(-3.0)) > 0.001 {
		t.Errorf("Expected -3.0, got %f", adj)
	}
}

func TestCarryAdjustment_Audit_Neutral(t *testing.T) {
	diff := domain.RateDifferential{Differential: 0.5}
	adj := CarryAdjustment(diff, "BEARISH")
	// Small positive diff + BEARISH, but diff <= 1.0 → no opposed penalty
	if adj != 0 {
		t.Errorf("Expected 0 for small opposed carry, got %f", adj)
	}
}

func TestCarryAdjustment_Audit_ExtremeValues(t *testing.T) {
	// Very large differential should be capped
	diff := domain.RateDifferential{Differential: 10.0}
	adj := CarryAdjustment(diff, "BULLISH")
	if adj > 5 {
		t.Errorf("Carry adjustment should be capped at 5, got %f", adj)
	}

	diff2 := domain.RateDifferential{Differential: -10.0}
	adj2 := CarryAdjustment(diff2, "BULLISH")
	if adj2 < -5 {
		t.Errorf("Carry adjustment should be capped at -5, got %f", adj2)
	}
}

func TestCarryScore_Audit_Normalization(t *testing.T) {
	// diff = 5 → carryScore = clamp(5*20, -100, 100) = 100
	score := clampFloat(5.0*20, -100, 100)
	if score != 100 {
		t.Errorf("Expected carry score 100 for diff=5, got %f", score)
	}

	// diff = -5 → carryScore = -100
	score2 := clampFloat(-5.0*20, -100, 100)
	if score2 != -100 {
		t.Errorf("Expected carry score -100 for diff=-5, got %f", score2)
	}

	// diff = 0 → carryScore = 0
	score3 := clampFloat(0*20, -100, 100)
	if score3 != 0 {
		t.Errorf("Expected carry score 0 for diff=0, got %f", score3)
	}
}

func TestClampFloat_Audit(t *testing.T) {
	if clampFloat(-200, -100, 100) != -100 {
		t.Error("clamp -200 to [-100,100] should be -100")
	}
	if clampFloat(200, -100, 100) != 100 {
		t.Error("clamp 200 to [-100,100] should be 100")
	}
	if clampFloat(50, -100, 100) != 50 {
		t.Error("clamp 50 to [-100,100] should be 50")
	}
}
