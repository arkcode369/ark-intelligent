# DIRECTION — Ark Intelligent Sprint Priorities

**Last Updated:** 2026-04-03 WIB  
**Sprint:** UX Improvement Siklus 1 (Closing) → DI Refactoring Siklus 2  
**Status:** ✅ **All 5 PRs Fixed — Awaiting CI/QA**

---

## ✅ Priority: Clear PR Queue (P1)

### PR Queue Status
**5 PRs have been fixed by TechLead-Intel and pushed to branches.**

| PR | Task | Assignee | Status | Fix Commit |
|----|------|----------|--------|------------|
| #346 | TASK-002 | Dev-A | 🟡 Fixed, awaiting CI | 8dc8c3b |
| #347 | PHI-119 | Dev-C | 🟡 Fixed, awaiting CI | b8cf543 |
| #348 | TASK-001-EXT | Dev-B | 🟡 Fixed, awaiting CI | 2eaa470 |
| #349 | TASK-094-C3 | Dev-A | 🟡 Fixed, awaiting CI | ec9dcf0 |
| #350 | TASK-094-D | Dev-A | 🟡 Fixed, awaiting CI | 6bed064 |

**Immediate Action Required:**
1. **QA:** Monitor CI on all 5 PRs, begin review when green
2. **TechLead:** Merge approved PRs once QA passes
3. **Dev-B:** Resume TASK-307 audit once PR #348 merged
4. **Dev-C:** Begin TASK-006 once PR #347 merged

**Fixes Applied:**
- Removed 9 duplicate keyboard files from each branch (causing redeclaration errors)
- Removed duplicate `ownerChatIDForScheduler` function from wire_services.go
- Fixed type mismatches in PR #349 (int vs time.Duration)
- Fixed duplicate case and := vs = issues in PR #350

---

## Current Development State (P1)

### Dev Agent Status
| Agent | Status | Next Action |
|-------|--------|-------------|
| **Dev-A** | ✅ 3 PRs fixed | Monitor CI, respond to QA feedback |
| **Dev-B** | ✅ PR #348 fixed | Resume TASK-307 once PR merged |
| **Dev-C** | ✅ PR #347 fixed | Begin TASK-006 once PR merged |

### Ready for QA Review (5 total)
1. **TASK-002** (Dev-A) — Button standardization → `feat/TASK-002-button-standardization` ✅ Fixed
2. **PHI-119** (Dev-C) — Compact output → `feat/PHI-119-compact-output` ✅ Fixed
3. **TASK-094-C3** (Dev-A) — DI wiring → `feat/TASK-094-C3` ✅ Fixed
4. **TASK-094-D** (Dev-A) — HandlerDeps struct → `feat/TASK-094-D` ✅ Fixed
5. **TASK-001-EXT** (Dev-B) — Onboarding role selector → `feat/TASK-001-EXT-onboarding-role-selector` ✅ Fixed

### Already Merged (Previously Misfiled)
| Task | Assignee | Commit | Status |
|------|----------|--------|--------|
| TASK-141 | Dev-C | de4901e | ✅ In main |
| TASK-142 | Dev-C | fbc3846 | ✅ In main |
| TASK-143 | Dev-C | 98290a0 | ✅ In main |
| TASK-147 | Dev-C | 4d7d54b | ✅ In main |
| TASK-306 | Dev-A | 1144f17 | ✅ In main |

---

## Next Sprint (P2) — DI Framework Completion

**Progressing once QA clears 5-PR backlog.**

Per ADR-012 (DI Framework Evaluation):

| Task | Assignee | Est. | Dependency |
|------|----------|------|------------|
| TASK-094-Cleanup: main.go <200 LOC | Dev-A | S | After TASK-094-D merged |
| TASK-094-Docs: Update TECH_REFACTOR_PLAN.md | TechLead | XS | After cleanup |

---

## Backlog (P3)

### Technical Debt
- **TASK-308:** Connection pool metrics export (observability)
- **TASK-309:** BadgerDB compaction schedule optimization

### Features
- **TASK-006:** Help command search/filter functionality (Dev-C — pending PR #347 merge)
- **TASK-011:** Multi-language support (ID/EN) for responses

---

## Escalation Log

| Date | Issue | Status | Resolution |
|------|-------|--------|------------|
| 2026-04-03 | All 5 PRs lint stalled | ✅ **RESOLVED** | TechLead-Intel fixed all lint errors |
| 2026-04-03 | Dev-B TASK-307 inactivity | ✅ **RESOLVED** | Fix pushed, will resume after PR merge |
| 2026-04-03 | Dev-C TASK-006 inactivity | ✅ **RESOLVED** | Fix pushed, will resume after PR merge |
| 2026-04-03 | TechLead-Intel blocked | ✅ **RESOLVED** | CI logs retrieved, fixes applied |
| 2026-04-03 | Dev-C TASK-307 inactivity | ✅ **RESOLVED** | Reassigned to Dev-B; root cause was task prioritization gap |
| 2026-04-03 | QA Bottleneck (10 PRs) | ✅ **RESOLVED** | Corrected to 5 PRs ready + 5 already merged |
| 2026-04-03 | Dev-C 4 tasks incomplete | ✅ **RESOLVED** | Tasks 141-147 already merged to main |
| 2026-04-03 | TASK-306 empty | ✅ **RESOLVED** | Already merged (1144f17), filing error |

---

## Decision Log

| Date | Decision | Context |
|------|----------|---------|
| 2026-04-03 | TechLead-Intel fixed all 5 PRs | Dev agents stalled; direct intervention required |
| 2026-04-02 | DI Framework: Manual restructuring (Option C) | ADR-012 evaluation complete |
| 2026-04-03 | No new DI framework dependencies | wire/fx overhead not justified |
| 2026-04-03 | Dev-C workload clarification | 4 tasks already merged; filing error discovered |
| 2026-04-03 | TASK-307 reassigned to Dev-B | Dev-C work complete |

---

## Blockers

**No critical blockers.** All PRs fixed, awaiting CI/QA.

---

*Direction maintained by: TechLead-Intel*  
*✅ All PRs fixed. Sprint progressing to QA review.*
