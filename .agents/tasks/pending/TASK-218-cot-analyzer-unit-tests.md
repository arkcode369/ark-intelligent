# TASK-218: COT Analyzer Pure Function Unit Tests

**Priority:** MEDIUM
**Type:** Test Coverage
**Estimated:** M
**Area:** internal/service/cot/analyzer_test.go (new)
**Ref:** TECH-009 in TECH_REFACTOR_PLAN.md (COT coverage target: 60%)
**Created by:** Research Agent
**Created at:** 2026-04-02 08:00 WIB
**Siklus:** 4 — Technical Refactor

## Problem

`internal/service/cot/analyzer.go` (822 LOC) adalah jantung dari seluruh COT analysis — tapi tidak punya satu pun unit test. Fungsi-fungsi pure di dalamnya (tidak butuh DB, network, atau external dependency) bisa langsung ditest:

| Fungsi | Yang Ditest |
|--------|-------------|
| `computeCOTIndex(nets []float64) float64` | Range 0-100, percentile calculation |
| `computeSentiment(a domain.COTAnalysis) float64` | Composite weighting |
| `classifySignal(cotIndex, momentum float64, isCommercial bool)` | Signal category matrix |
| `classifySignalStrength(a domain.COTAnalysis)` | Strength 1-5 |
| `classifySmallSpec(a domain.COTAnalysis)` | Small spec positioning |
| `detectDivergence(specNetChange, commNetChange float64) bool` | Spec vs commercial divergence |
| `classifyMomentumDir(specMom, commMom float64) domain.MomentumDirection` | Momentum direction |

## Test Cases yang Harus Ada

```go
// computeCOTIndex
TestComputeCOTIndex_AllZero          → 50.0
TestComputeCOTIndex_MaxNet           → 100.0
TestComputeCOTIndex_MinNet           → 0.0
TestComputeCOTIndex_MiddleValue      → nilai antara 0-100
TestComputeCOTIndex_SingleElement    → edge case 1 elemen

// classifySignal
TestClassifySignal_BullishSpec       → cotIndex=80, isCommercial=false → BULLISH
TestClassifySignal_BearishSpec       → cotIndex=20, isCommercial=false → BEARISH
TestClassifySignal_NeutralSpec       → cotIndex=50 → NEUTRAL
TestClassifySignal_CommercialInverse → commercial true, cotIndex=80 → BEARISH (inverse logic)

// detectDivergence
TestDetectDivergence_True            → spec+comm berlawanan arah → true
TestDetectDivergence_False           → spec+comm searah → false

// classifySignalStrength
TestClassifySignalStrength_Strong    → high index + high z-score → strength 4-5
TestClassifySignalStrength_Weak      → neutral index → strength 1-2
```

## File Changes

- `internal/service/cot/analyzer_test.go` — NEW: 15+ test cases

## Acceptance Criteria

- [x] File baru `internal/service/cot/analyzer_test.go`
- [x] Minimal 15 test cases covering 7 fungsi pure (✅ 45+ test cases covering 12 pure functions)
- [x] Test tidak memerlukan DB/network (pure function only)
- [x] `go test ./internal/service/cot/... -run TestComputeCOT\|TestClassify\|TestDetect` semua PASS
- [x] `go build ./...` clean
- [x] Branch: `feat/TASK-218-cot-analyzer-tests`

## Implementation

**Branch:** `feat/TASK-218-cot-analyzer-tests`  
**PR:** #378 - https://github.com/arkcode369/ark-intelligent/pull/378  
**Status:** In Review (pending QA approval)
