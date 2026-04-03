# PHI-119: TASK-004 Compact Output Mode Default + Expand Button

**Status:** completed ✅  
**Assigned to:** Dev-C  
**Priority:** medium  
**Type:** feature  
**Estimated:** M  
**Area:** internal/adapter/telegram/handler/  
**Created at:** 2026-04-03 WIB  
**Completed at:** 2026-04-03 WIB  
**Siklus:** UX/UI  

## Deskripsi

Implement compact output mode as default with "📖 Detail Lengkap" expand button for long outputs.

## Background

Per .agents/research/2026-04-01-01-ux-onboarding-navigation.md, some outputs (/cot, /macro) are very long (>4000 chars). Telegram cuts messages or users scroll extensively on mobile.

## Scope

### 1. Default to "compact" view for commands:
- `/cot` — show summary + key numbers only
- `/macro` — show key indicators only

### 2. Add expand button `📖 Detail Lengkap` to show full output

### 3. Store user preference (compact/full) in BadgerDB:
- Key: `user:{chatID}:output_mode`
- Values: `compact`, `full`

### 4. Add `/settings output_mode` command to change preference

## Acceptance Criteria

- [x] go build ./... sukses
- [x] go vet ./... sukses
- [x] /cot shows compact view by default (<1000 chars)
- [x] /macro shows compact view by default
- [x] Expand button shows full details
- [x] User preference persisted in BadgerDB

## Implementation

### Changes Made

1. **handler_cot_cmd.go**: Modified `cmdCOT` to use `renderCOTOverview` which respects OutputMode preference (compact by default)
2. **handler_macro_cmd.go**: Modified `macroSendSummary` to check OutputMode and show compact view by default with expand button
3. **handler_settings_cmd.go**: Added `output_mode_toggle` handler and toast message for settings

### Branch

`feat/PHI-119-compact-output` — pushed and ready for review

## Referensi

- Research: .agents/research/2026-04-01-01-ux-onboarding-navigation.md
- Files: handler_cot_cmd.go, handler_macro_cmd.go, handler_settings_cmd.go
- Paperclip: [PHI-119](/PHI/issues/PHI-119)

## Progress

- [x] Add compact formatter functions (already existed)
- [x] Add expand button callback handler (already existed)
- [x] Add BadgerDB persistence for preference (via prefsRepo)
- [x] Update /cot handler
- [x] Update /macro handler
- [x] Add /settings output_mode command
- [x] Test both modes
- [x] Create PR
