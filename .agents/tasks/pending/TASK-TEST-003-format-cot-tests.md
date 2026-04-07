# TASK-TEST-003: Unit Tests for format_cot.go Output Formatters

**Priority:** High  
**Type:** Test Coverage  
**Estimated Effort:** 4-5 hours  
**Status:** In Progress  
**Claimed By:** Dev-A

---

## Objective

Create comprehensive unit tests for `internal/adapter/telegram/format_cot.go` — the COT (Commitment of Traders) output formatting functions. This file (1,394 lines) handles all COT-related output formatting including net positioning, changes, percentile analysis, and divergences.

---

## Background

The format_cot.go file provides formatting functions for:
- COT net positioning displays
- Weekly changes and momentum
- Percentile analysis vs historical
- Commercial vs non-commercial divergences
- Multi-asset COT comparisons

**Current State:** 1,394 lines, **0 unit tests**.

---

## Acceptance Criteria

### Coverage Targets
- [ ] Minimum 60% code coverage for pure formatting functions
- [ ] All COT output formatters have test cases
- [ ] Error handling paths tested
- [ ] Edge cases covered (empty data, nil inputs, extreme values)

### Specific Test Cases Required

#### 1. COT Overview Formatting
- [ ] Test `FormatCOTOverview()` with valid multi-asset data
- [ ] Test `FormatCOTOverview()` with empty analysis list
- [ ] Test `FormatCOTOverview()` with nil conviction scores
- [ ] Test emoji/sentiment mapping for bullish/bearish/neutral

#### 2. Single Currency COT Formatting
- [ ] Test `FormatCOTCurrency()` with complete data
- [ ] Test `FormatCOTCurrency()` with missing/ partial data
- [ ] Test commercial positioning display
- [ ] Test non-commercial positioning display
- [ ] Test percentile formatting

#### 3. Change/Momentum Formatting
- [ ] Test weekly change calculations
- [ ] Test momentum indicator formatting
- [ ] Test large change highlighting (>2 std dev)
- [ ] Test zero/small change handling

#### 4. Divergence Detection
- [ ] Test commercial/non-commercial divergence detection
- [ ] Test extreme positioning alerts (>90th percentile)
- [ ] Test format divergence message generation

#### 5. Multi-Currency Comparison
- [ ] Test `FormatCOTCompare()` with 2+ currencies
- [ ] Test ranking by net positioning
- [ ] Test relative strength calculations

#### 6. Edge Cases
- [ ] Test with nil COT analysis
- [ ] Test with empty currency list
- [ ] Test with extreme values (very large positions)
- [ ] Test HTML escaping in output
- [ ] Test date formatting edge cases

---

## Technical Notes

### Test Structure

```go
func TestFormatCOTOverview_ValidData(t *testing.T) { }
func TestFormatCOTOverview_EmptyList(t *testing.T) { }
func TestFormatCOTCurrency_CompleteData(t *testing.T) { }
func TestFormatCOTCurrency_MissingData(t *testing.T) { }
func TestFormatCOTChange_Weekly(t *testing.T) { }
func TestFormatCOTChange_LargeChange(t *testing.T) { }
func TestFormatCOTDivergence_Detected(t *testing.T) { }
func TestFormatCOTDivergence_None(t *testing.T) { }
func TestFormatCOTCompare_MultiCurrency(t *testing.T) { }
func TestFormatCOT_PercentileFormatting(t *testing.T) { }
```

### Mock Data

Create realistic COT test data:
```go
testCOT := &domain.COTAnalysis{
    Currency: "EUR",
    ReportDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
    Commercial: domain.COTPositioning{
        Net: 45000,
        Long: 125000,
        Short: 80000,
    },
    NonCommercial: domain.COTPositioning{
        Net: -32000,
        Long: 65000,
        Short: 97000,
    },
    Percentiles: domain.COTPercentiles{
        NetCommercial: 85.5,
        NetNonCommercial: 12.3,
    },
}
```

---

## Implementation Guidelines

1. **Create** `internal/adapter/telegram/format_cot_test.go`
2. **Use** `testify/assert` for assertions
3. **Use** `testify/require` for fatal assertions
4. **Follow** existing test patterns from `formatter_test.go`
5. **Ensure** tests run quickly (< 3s total)
6. **Test** HTML output contains expected tags
7. **Test** emoji presence in formatted output

---

## Definition of Done

- [x] Test file created with comprehensive coverage
- [x] All tests passing (`go test ./internal/adapter/telegram/... -run TestFormatCOT`)
- [ ] Coverage report shows 60%+ for tested functions
- [x] `go build ./...` passes
- [x] `go vet ./...` passes
- [ ] Code review approved
- [ ] Merged to main branch

---

## Related Files

- `internal/adapter/telegram/format_cot.go` (1,394 lines) — primary target
- `internal/domain/cot.go` — COT domain types
- `internal/service/cot/` — COT service layer
- `internal/adapter/telegram/formatter_test.go` — reference patterns

---

## Context

COT data is critical for institutional positioning analysis. The formatting logic handles:
- Weekly COT report releases (every Friday)
- Commercial hedger vs speculator positioning
- Historical percentile rankings (1y, 3y, 5y lookback)
- Multi-timeframe momentum detection

**Related:**
- TASK-TEST-002: handler_alpha.go tests
- TASK-TEST-005: format_cta.go tests

---

*Task claimed by Dev-A — ARK Intelligent*
