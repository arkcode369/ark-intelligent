# PHI-115: TASK-094-C3 Extract wire_telegram.go and wire_schedulers.go

**Status:** in_progress  
**Assigned to:** Dev-A  
**Priority:** medium  
**Type:** refactor  
**Estimated:** M  
**Area:** cmd/bot/  
**Created at:** 2026-04-03 WIB  
**Siklus:** Refactor / DI Restructuring  

## Deskripsi

Complete the DI restructuring per ADR-TECH-012 by extracting remaining wiring logic from main.go.

## Background

Per .agents/research/2026-04-01-adr-di-framework.md, implementing Option C (structured manual wiring):
- TASK-094-D (HandlerDeps struct) ✅ completed
- TASK-094-C1 (wire_storage.go) ✅ completed  
- TASK-094-C2 (wire_services.go) ✅ completed
- TASK-094-C3 (this task) — remaining work

## Scope

### 1. Create `cmd/bot/wire_telegram.go`
- Extract Handler wiring from main.go section 6
- Return `*TelegramDeps` struct with Bot, Handler, and related deps
- ~100-150 LOC

### 2. Create `cmd/bot/wire_schedulers.go`
- Extract scheduler initialization from main.go section 7
- Return `*SchedulerDeps` struct with all background workers
- ~100-150 LOC

### 3. Refactor `cmd/bot/main.go`
- Reduce from 717 → ~200 LOC
- Become thin orchestrator that calls wire functions

## Acceptance Criteria

- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] main.go < 250 LOC
- [ ] All wire_*.go files < 150 LOC each
- [ ] No functional changes — only code organization

## Referensi

- ADR: .agents/research/2026-04-01-adr-di-framework.md
- Current main.go: cmd/bot/main.go (717 LOC)
- Paperclip: [PHI-115](/PHI/issues/PHI-115)

## Progress

- [ ] Create wire_telegram.go
- [ ] Create wire_schedulers.go
- [ ] Refactor main.go to orchestrator
- [ ] Verify builds pass
- [ ] Create PR
