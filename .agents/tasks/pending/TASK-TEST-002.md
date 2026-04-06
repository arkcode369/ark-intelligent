# TASK-TEST-002: Unit Tests for handler_alpha.go Signal Generation

**Priority:** High  
**Estimated Effort:** 4-6 hours  
**Type:** Test Coverage  
**Scope:** `internal/adapter/telegram/handler_alpha.go`

---

## Overview

Implement comprehensive unit tests for the alpha handler signal generation logic. The handler_alpha.go file contains the Alpha Engine dashboard commands (/alpha, /xfactors, /playbook, /heat, /rankx, /transition, /cryptoalpha) with complex signal generation, caching, and callback navigation.

---

## Acceptance Criteria

- [ ] Tests for `alphaStateCache` (get, set, TTL expiration, cleanup)
- [ ] Tests for `computeAlphaState` (factor ranking, strategy playbook, crypto microstructure)
- [ ] Tests for `formatAlphaSummary` (regime detection, recommendations, warnings)
- [ ] Tests for `buildReasonIndonesian` (reason string generation)
- [ ] Tests for helper functions: `alphaConvEmoji`, `alphaHeatEmoji`, `alphaErr`
- [ ] Mock implementations for AlphaServices dependencies
- [ ] Test coverage target: >80% for signal generation logic
- [ ] All tests pass: `go test ./internal/adapter/telegram/...`

---

## Technical Details

### Key Components to Test

1. **alphaStateCache** (lines 69-104)
   - Thread-safe get/set operations
   - TTL expiration (AlphaStateTTL from config)
   - Opportunistic cleanup when store > 50 entries

2. **computeAlphaState** (lines 136-180)
   - AssetProfileBuilder.BuildProfiles() integration
   - FactorEngine.Rank() result processing
   - StrategyEngine.Generate() playbook creation
   - MicroEngine.AnalyzeMultiple() crypto signals
   - Error handling when engines not configured

3. **formatAlphaSummary** (lines 351-472)
   - Regime and stability assessment
   - Top recommendations extraction (TopLong/TopShort)
   - Warnings generation (heat, transition, crypto)
   - HTML escaping and formatting

4. **Callback handlers** (lines 212-344)
   - handleAlphaCallback routing
   - State recompute on expiration
   - Action handling: back, refresh, factors, playbook, heat, rankx, transition, crypto

### Dependencies to Mock

```go
type AlphaServices struct {
    FactorEngine   *factors.Engine
    StrategyEngine *strategy.Engine
    MicroEngine    *microstructure.Engine
    ProfileBuilder AssetProfileBuilder
}
```

### Test File Location

`internal/adapter/telegram/handler_alpha_test.go`

---

## Implementation Notes

1. Use testify/mock for dependency mocking
2. Create test fixtures for AssetProfile, RankingResult, PlaybookResult
3. Test both success paths and error conditions
4. Verify thread safety for cache operations
5. Test TTL expiration with time.Now() patching or manual time manipulation

---

## Related

- handler_alpha.go (1276 lines of signal generation logic)
- factors.Engine (ranking algorithm)
- strategy.Engine (playbook generation)
- microstructure.Engine (crypto analysis)

---

## Created

2026-04-06 by Dev-A ARK Intelligent
