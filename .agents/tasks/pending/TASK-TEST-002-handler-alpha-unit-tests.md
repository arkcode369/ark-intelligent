# TASK-TEST-002: Unit Tests for handler_alpha.go Signal Generation

| Field | Value |
|-------|-------|
| **ID** | TASK-TEST-002 |
| **Priority** | HIGH |
| **Type** | Test Coverage |
| **Estimate** | 4-6 hours |
| **Area** | internal/adapter/telegram/handler_alpha_test.go (new) |
| **Created by** | Dev-A |
| **Created at** | 2026-04-07 |

---

## Problem Statement

`internal/adapter/telegram/handler_alpha.go` (~1100 LOC) contains alpha signal generation and ranking logic that is completely untested. This file includes critical functionality for:
- Alpha rank bias calculation
- Signal generation and formatting
- Keyboard building for alpha commands

Without tests, changes to this logic risk breaking the alpha signal feature silently.

---

## Current State

- **File**: `internal/adapter/telegram/handler_alpha.go` (~1100 LOC)
- **Test file**: None
- **Coverage**: 0%

---

## Functions Requiring Tests

### Pure/Mockable Functions:
1. **Alpha ranking logic** - `cmdAlphaRank()`, `alphaDetailMenu()`
2. **Signal generation** - Bias calculation, signal formatting
3. **Keyboard builders** - Alpha-related keyboards
4. **Helper functions** - Any pure calculation functions

---

## Acceptance Criteria

- [ ] Create `internal/adapter/telegram/handler_alpha_test.go`
- [ ] Test `cmdAlphaRank` command handler with mocked services
- [ ] Test `alphaDetailMenu` callback handler
- [ ] Test alpha signal generation logic
- [ ] Test keyboard building for alpha commands
- [ ] Test error handling paths
- [ ] Minimum 10 test cases covering core functionality
- [ ] All tests pass: `go test ./internal/adapter/telegram/... -run TestAlpha`
- [ ] Build passes: `go build ./...`
- [ ] Branch: `test/handler-alpha-unit-tests`

---

## Test Strategy

### Mock Pattern
```go
func TestCmdAlphaRank(t *testing.T) {
    // Arrange
    mocks := setupAlphaMocks()
    h := NewHandler(mocks.deps...)
    update := makeTestUpdate("/alpha EURUSD")
    
    // Act
    err := h.cmdAlphaRank(update)
    
    // Assert
    assert.NoError(t, err)
    mocks.alphaService.AssertCalled(t, "GetAlphaRank", "EURUSD")
}
```

### Table-Driven Pattern
```go
func TestAlphaDetailMenu(t *testing.T) {
    tests := []struct {
        name      string
        callback  string
        wantErr   bool
    }{
        {
            name:     "valid alpha detail",
            callback: "alpha_detail_EURUSD",
            wantErr:  false,
        },
        {
            name:     "invalid callback data",
            callback: "alpha_detail_invalid",
            wantErr:  true,
        },
    }
    // ...
}
```

---

## Files to Create/Modify

- `internal/adapter/telegram/handler_alpha_test.go` (new)
- May need to add interfaces or mock helpers in test files

---

## Risk Assessment

**Impact**: MEDIUM — Alpha signals are a key feature  
**Effort**: MEDIUM — 4-6 hours  
**Priority**: HIGH (critical feature coverage gap)

---

*Task created by Dev-A — ARK Intelligent*
