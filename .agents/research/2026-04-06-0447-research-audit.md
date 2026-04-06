# Research Audit Report — 2026-04-06 04:47 UTC

**Agent:** Research Agent (ARK Intelligent)  
**Type:** Scheduled audit  
**Status:** Complete — corrections made to previous audit inaccuracies

---

## Executive Summary

This audit found **discrepancies in the previous audit reports** and identified **4 real issues** requiring task specs. The previous STATUS.md entries for TASK-BUG-001, TASK-SECURITY-001, TASK-CODEQUALITY-002, and multiple TASK-TEST entries referenced non-existent files or contained fabricated information.

**New task specs created:** 4  
**Test coverage:** 21.1% (107 test files / 507 total Go files)  
**All agents:** Idle

---

## Corrections to Previous Audits

### ❌ Previously Reported (Inaccurate)

| Item | Previous Claim | Reality |
|------|---------------|---------|
| TASK-BUG-001 | Data race in `handler_session.go:23` | **File does not exist** |
| TASK-SECURITY-001 | http.DefaultClient at `tradingeconomics_client.go:246` | ✅ Exists, confirmed real issue |
| TASK-CODEQUALITY-002 | 9 context.Background() in 6 files | Actually 10 occurrences, several with timeouts (acceptable) |
| stdlog.Printf | 3 occurrences in `vix/fetcher.go` | **None found** — may have been fixed |
| sentiment/cache.go | Unchecked type assertion at line 116 | ✅ Exists but file is 204 lines (previous audit said 70 lines) |
| Test coverage | 26.9% | Actually **21.1%** |
| Pending tasks | 22 tasks | **28 feature tasks** exist, but 0 bug/security tasks |

### ✅ Confirmed Real Issues

1. **http.DefaultClient without timeout** — `internal/service/macro/tradingeconomics_client.go:246`
2. **context.Background() without timeout** — `internal/service/ai/chat_service.go:312` (owner notify goroutine)
3. **context.Background() not using parentCtx** — `internal/scheduler/scheduler_skew_vix.go:56, 74`
4. **Unchecked type assertion** — `internal/service/sentiment/cache.go:116`

---

## New Task Specs Created

| Task ID | Issue | Priority | Effort |
|---------|-------|----------|--------|
| TASK-SECURITY-001 | http.DefaultClient timeout | High | 1-2h |
| TASK-CODEQUALITY-003 | chat_service context timeout | Medium | 1h |
| TASK-CODEQUALITY-004 | scheduler_skew_vix use parentCtx | Low | 30m |
| TASK-CODEQUALITY-005 | sentiment cache type assertion | Low | 15m |

---

## Detailed Findings

### 1. http.DefaultClient Timeout (High Priority)

**File:** `internal/service/macro/tradingeconomics_client.go:246`

```go
resp, err := http.DefaultClient.Do(req)  // No timeout!
```

**Risk:** Goroutine leaks if API hangs  
**Fix:** Use `&http.Client{Timeout: 30 * time.Second}`

---

### 2. context.Background() Without Timeout (Medium Priority)

**File:** `internal/service/ai/chat_service.go:312`

```go
go cs.ownerNotify(context.Background(), html)  // No timeout/cancellation
```

**Risk:** Goroutine leak if owner notification hangs  
**Fix:** Wrap in timeout context

---

### 3. Unused parentCtx Parameter (Low Priority)

**File:** `internal/scheduler/scheduler_skew_vix.go:56, 74`

The function receives `parentCtx context.Context` but uses `context.Background()` for broadcasts. This prevents proper cancellation during shutdown.

---

### 4. Unchecked Type Assertion (Low Priority)

**File:** `internal/service/sentiment/cache.go:116`

```go
return v.(*SentimentData), nil  // No ok check
```

While singleflight guarantees the type, defensive programming recommends checking.

---

## Test Coverage Analysis

### Large Untested Files (>25KB)

| File | Size | Priority for Testing |
|------|------|---------------------|
| `internal/adapter/telegram/format_cot.go` | 49.0 KB | High (COT formatting) |
| `internal/scheduler/scheduler.go` | 43.4 KB | **Critical** (core orchestration) |
| `internal/adapter/telegram/handler_alpha.go` | 40.0 KB | High (alpha signals) |
| `internal/service/news/scheduler.go` | 36.7 KB | **Critical** (alert infrastructure) |
| `internal/service/ta/indicators.go` | 27.0 KB | High (calculation logic) |

**Note:** Previous audit mentioned `scheduler.go` (TASK-TEST-013), `ta/indicators.go` (TASK-TEST-014), and `news/scheduler.go` (TASK-TEST-015) as needing tests, but no task files exist for these.

---

## Codebase Health Check

| Check | Status |
|-------|--------|
| HTTP body.Close() | ✅ Properly used throughout |
| SQL injection | ✅ No risks (uses BadgerDB, not SQL) |
| TODO/FIXME in production | ✅ 0 found |
| Panic recovery | ✅ Implemented in worker pools |
| Race conditions | ⚠️ No confirmed races found (previous TASK-BUG-001 was invalid) |
| HTTP timeouts | ⚠️ 1 issue found (tradingeconomics_client) |

---

## Agent Activity

| Role | Status |
|------|--------|
| Coordinator | Idle |
| Research | Completing audit |
| Dev-A | Idle |
| Dev-B | Idle |
| Dev-C | Idle |
| QA | Idle |

---

## Recommendations

1. **Fix TASK-SECURITY-001 immediately** — http.DefaultClient is a real security/stability issue
2. **Create test task specs** for critical untested files (scheduler.go, news/scheduler.go)
3. **Audit STATUS.md accuracy** — previous entries contained fabricated file references
4. **Verify all task IDs in STATUS.md** have corresponding files in `.agents/tasks/pending/`

---

*Report generated by Research Agent at 2026-04-06 04:47 UTC*  
*Previous audit discrepancies identified and corrected*
