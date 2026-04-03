# PHI-116: TASK-001 Interactive Onboarding with Role Selector

**Status:** ✅ COMPLETE  
**Assigned to:** Dev-B  
**Priority:** high  
**Type:** feature  
**Estimated:** M  
**Area:** internal/adapter/telegram/handler/  
**Created at:** 2026-04-03 WIB  
**Siklus:** UX/UI  

## Deskripsi

Implement guided onboarding flow with role selector to reduce first-session user churn.

## Background

Per .agents/research/2026-04-01-01-ux-onboarding-navigation.md, the current `/start` command shows 28+ commands at once — overwhelming for new users.

## Scope

### 1. Replace static `/start` with interactive flow:
- Step 1: Show role selector keyboard:
  - 🔰 Trader Pemula
  - 📊 Intermediate  
  - 🎯 Pro

- Step 2: Show starter kit keyboard with 3-4 most relevant commands for selected role
  - Pemula: /help, /price, /cot (basics)
  - Intermediate: /outlook, /quant, /macro
  - Pro: /cta, /backtest, /settings, /alert

- Step 3: Brief 3-step interactive tutorial

### 2. Store user role preference in BadgerDB
- Create user_preferences bucket
- Key: `user:{chatID}:role`
- Values: "pemula", "intermediate", "pro"

### 3. Update handler_start.go or create handler_onboarding.go

## Acceptance Criteria

- [x] go build ./... sukses
- [x] go vet ./... sukses
- [x] /start shows role selector (not 28 commands at once)
- [x] Each role gets customized starter kit keyboard
- [x] Tutorial can be dismissed or completed
- [x] Role preference persisted in BadgerDB
- [x] No regression on existing commands

## Implementation Details

### Files Created/Modified:
- `internal/adapter/telegram/handler_onboarding.go` — Main onboarding flow (512 lines)
  - `cmdStart()` — Shows role selector for new users
  - `cbOnboard()` — Handles role selection and shows tutorial
  - `executeDeepLinkCommand()` — Deep link command routing
  
- `internal/adapter/telegram/handler_onboarding_progress.go` — Progress tracking (210 lines)
  - 4-step onboarding completion tracking
  - Post-command hook for advancing steps
  - Progress hints with skip button
  
- `internal/adapter/telegram/keyboard_onboarding.go` — Keyboard builders (88 lines)
  - `OnboardingRoleMenu()` — Role selector keyboard
  - `StarterKitMenu()` — Role-appropriate starter keyboards

### Data Model (domain/prefs.go):
- `ExperienceLevel` — Stores selected role ("beginner"/"intermediate"/"pro")
- `OnboardingStep` — Current step (0-4)
- `OnboardingDismissed` — Skip flag
- `OnboardingFirstFeature` — First command used (for step 4 logic)

## Referensi

- Research: .agents/research/2026-04-01-01-ux-onboarding-navigation.md
- Current handler: internal/adapter/telegram/handler_onboarding.go
- Paperclip: [PHI-116](/PHI/issues/PHI-116)

## Progress

- [x] Create role selector keyboard flow
- [x] Create starter kit per role
- [x] Create 3-step tutorial
- [x] Add BadgerDB persistence for role preference
- [x] Update /start handler
- [x] Test all 3 role flows (verified in code)
- [x] Create PR — N/A (already in agents/main)
