# TASK-094-C3: Extract wire_telegram.go + wire_schedulers.go

**Assignee:** Dev-A  
**Priority:** MEDIUM  
**Paperclip:** PHI-115  
**ADR:** .agents/research/2026-04-01-adr-di-framework.md

---

## Goal
Extract Telegram bot wiring and scheduler wiring from `main.go` into separate files per TECH-012 ADR (Option C: structured manual wiring).

## Acceptance Criteria

### wire_telegram.go
- [ ] Create `TelegramDeps` struct holding: Bot, Middleware, Handler
- [ ] `InitializeTelegram(cfg, storageDeps, serviceDeps) (*TelegramDeps, error)`
- [ ] Extract from main.go sections 4, 8:
  - Bot creation with auth middleware
  - Python chart deps check
  - Handler creation with all service registrations
  - All `handler.WithXXX()` calls

### wire_schedulers.go
- [ ] Create `SchedulerDeps` struct holding: Main scheduler, News scheduler
- [ ] `InitializeSchedulers(cfg, storageDeps, serviceDeps, telegramDeps) (*SchedulerDeps, error)`
- [ ] Extract from main.go section 7:
  - Main scheduler setup with all dependencies
  - News scheduler setup with all wiring
  - Impact recorder, alert gates, Fed speech provider

### main.go reduction
- [ ] Replace extracted code with calls to `InitializeTelegram()` and `InitializeSchedulers()`
- [ ] main.go should be ~200 LOC after extraction
- [ ] All existing functionality preserved

## Files to Modify
- `cmd/bot/main.go` — reduce to ~200 LOC
- `cmd/bot/wire_telegram.go` — NEW
- `cmd/bot/wire_schedulers.go` — NEW

## Validation
```bash
go build ./...
go vet ./...
```

Must pass before PR submission.
