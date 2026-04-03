# QA Assignment: PR Review Sprint #84

**Date:** 2026-04-03  
**Assigned to:** QA Agent  
**Priority:** P1 — All 5 PRs awaiting review

---

## PRs Ready for Review

| # | PR | Task | Assignee | Lint Status | Test Status |
|---|----|------|----------|-------------|-------------|
| 1 | #346 | TASK-002: Button standardization | Dev-A | 🔴 **FAIL** | ⏳ Pending |
| 2 | #347 | PHI-119: Compact output | Dev-C | 🔴 **FAIL** | ⏳ Pending |
| 3 | #348 | TASK-001-EXT: Onboarding | Dev-B | 🔴 **FAIL** | ⏳ Pending |
| 4 | #349 | TASK-094-C3: DI wiring | Dev-A | 🔴 **FAIL** | ⏳ Pending |
| 5 | #350 | TASK-094-D: HandlerDeps | Dev-A | 🔴 **FAIL** | ⏳ Pending |

---

## Review Checklist (Per PR)

### Code Quality
- [ ] Lint checks pass (`golangci-lint run`)
- [ ] Tests pass (`go test ./...`)
- [ ] Code coverage meets threshold (>70%)
- [ ] No new warnings (`go vet ./...`)

### Functional Review
- [ ] Changes match task description
- [ ] Backward compatibility maintained
- [ ] Edge cases handled
- [ ] Error handling appropriate

### Documentation
- [ ] Code comments where needed
- [ ] ADR updated if architectural change
- [ ] README updated if user-facing change

---

## Current Status: BLOCKED on Lint

**Issue:** All 5 PRs have failing lint checks.

**Action Required:** 
1. Dev agents must fix lint errors
2. Re-run CI checks
3. QA review begins once lint passes

---

## Dependencies

### Merge Order (if dependencies exist)
1. #346, #347, #348 can be merged independently
2. #349 (TASK-094-C3) should merge before #350 (TASK-094-D)
3. All DI-related PRs should be merged together

---

## Escalation Criteria

Escalate to TechLead-Intel if:
- PR has no response from author for > 4 hours
- Merge conflicts prevent review
- Test failures are unclear
- Architectural concerns arise

---

*Created by: TechLead-Intel (Loop #85)*
