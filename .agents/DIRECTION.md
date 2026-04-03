# DIRECTION — Ark Intelligent Sprint Priorities

**Last Updated:** 2026-04-03 WIB  
**Sprint:** UX Improvement Siklus 1 (Closing) → DI Refactoring Siklus 2  
**Status:** ⚠️ **QA Backlog — 5 PRs Pending (Corrected from 10)**

---

## ⚠️ Priority: QA Capacity (P1)

### QA Backlog Alert
**5 PRs with actual commits are awaiting QA review**, which is manageable but needs steady attention.

| PR Count | Status | Age |
|----------|--------|-----|
| 3 | Original UX Siklus 1 PRs | > 8 hours |
| 2 | New PRs ready for submission | Current |

**Immediate Action Required:**
1. **QA:** Begin review of TASK-002, PHI-119, TASK-094-C3
2. **Dev agents:** Submit remaining 2 PRs (TASK-094-D, TASK-001-EXT)
3. **Dev-A:** Implement TASK-306 (empty branch needs work)
4. **Dev-C:** Begin 1 of the 4 claimed VIX tasks

---

## Current Development State (P1)

### Dev Agent Status
| Agent | Status | Next Action |
|-------|--------|-------------|
| **Dev-A** | 🔄 3 PRs ready, 1 needs work | Submit 3 PRs, implement TASK-306 |
| **Dev-B** | 🔄 TASK-307 assigned | Submit TASK-001-EXT PR, begin audit |
| **Dev-C** | 🔄 1 PR ready (PHI-119) | Submit PHI-119, begin 1 of 4 claimed tasks |

### Ready for PR Submission (5 total)
1. **TASK-002** (Dev-A) — Button standardization → `feat/TASK-002-button-standardization`
2. **PHI-119** (Dev-C) — Compact output → `feat/PHI-119-compact-output`
3. **TASK-094-C3** (Dev-A) — DI wiring → `feat/TASK-094-C3`
4. **TASK-094-D** (Dev-A) — HandlerDeps struct → `feat/TASK-094-D`
5. **TASK-001-EXT** (Dev-B) — Onboarding role selector → `feat/TASK-001-EXT-onboarding-role-selector`

### Needs Implementation (Empty Branches)
| Task | Assignee | Issue |
|------|----------|-------|
| TASK-306 | Dev-A | Branch empty — 18 services need httpclient migration |
| TASK-141 | Dev-C | Branch empty — VIX EOF handling |
| TASK-142 | Dev-C | Branch empty — VIX cache error propagation |
| TASK-143 | Dev-C | Branch empty — Log silenced errors |
| TASK-147 | Dev-C | Branch empty — Wyckoff phase guard |

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
- **TASK-006:** Help command search/filter functionality
- **TASK-011:** Multi-language support (ID/EN) for responses

---

## Escalation Log

| Date | Issue | Status | Resolution |
|------|-------|--------|------------|
| 2026-04-03 | Dev-C TASK-307 inactivity | ✅ **RESOLVED** | Reassigned to Dev-B; root cause was task prioritization gap |
| 2026-04-03 | QA Bottleneck (10 PRs) | ⚠️ **ACTIVE** | CTO review required; consider additional QA resources |

---

## Decision Log

| Date | Decision | Context |
|------|----------|---------|
| 2026-04-02 | DI Framework: Manual restructuring (Option C) | ADR-012 evaluation complete |
| 2026-04-03 | No new DI framework dependencies | wire/fx overhead not justified |
| 2026-04-03 | Dev-C workload clarification | 4 active tasks discovered; not inactive |
| 2026-04-03 | TASK-307 reassigned to Dev-B | Dev-C has sufficient workload |

---

## Blockers

| Blocker | Severity | Owner | ETA |
|---------|----------|-------|-----|
| ⚠️ QA Bottleneck (10 PRs) | **CRITICAL** | CTO/QA | TBD |

---

*Direction maintained by: TechLead-Intel*  
*⚠️ CRITICAL: QA capacity is blocking sprint progression. Immediate attention required.*
