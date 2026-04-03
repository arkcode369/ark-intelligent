# PHI-116: TASK-001 Interactive Onboarding with Role Selector

**Status:** in_progress  
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

- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] /start shows role selector (not 28 commands at once)
- [ ] Each role gets customized starter kit keyboard
- [ ] Tutorial can be dismissed or completed
- [ ] Role preference persisted in BadgerDB
- [ ] No regression on existing commands

## Referensi

- Research: .agents/research/2026-04-01-01-ux-onboarding-navigation.md
- Current handler: internal/adapter/telegram/handler/handler_start.go
- Paperclip: [PHI-116](/PHI/issues/PHI-116)

## Progress

- [ ] Create role selector keyboard flow
- [ ] Create starter kit per role
- [ ] Create 3-step tutorial
- [ ] Add BadgerDB persistence for role preference
- [ ] Update /start handler
- [ ] Test all 3 role flows
- [ ] Create PR
