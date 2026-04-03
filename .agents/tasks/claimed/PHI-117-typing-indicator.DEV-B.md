# PHI-117: TASK-003 Typing Indicator / Progress Feedback for Long Commands

**Status:** in_progress  
**Assigned to:** Dev-B  
**Priority:** high  
**Type:** feature  
**Estimated:** S  
**Area:** internal/adapter/telegram/handler/  
**Created at:** 2026-04-03 WIB  
**Siklus:** UX/UI  

## Deskripsi

Add typing indicator and progress feedback for commands that take 5-15 seconds (/outlook, /quant, /cta). Users often resend commands because they think the bot hung during long operations.

## Scope

### 1. Send `sendChatAction` typing indicator immediately when command received:
- `typing` for text-based analysis commands (/outlook, /quant, /cta)
- `upload_photo` for chart generation commands (/chart)

### 2. For multi-step operations, edit message with progress:
- "⏳ Menganalisis... (1/3) Fetching data..."
- "⏳ Menganalisis... (2/3) Processing with AI..."
- "⏳ Menganalisis... (3/3) Formatting response..."

### 3. Target commands and expected timing:
| Command | Duration | Type |
|---------|----------|------|
| `/outlook` | 5-10s | text |
| `/quant` | 5-15s | text |
| `/cta` | 3-8s | text |
| `/backtest` | 5-20s | text |
| `/chart` | 2-5s | photo |

## Acceptance Criteria

- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Typing indicator shows within 1 second of command
- [ ] Progress messages for multi-step operations
- [ ] No regression on fast commands (<2s)

## Referensi

- Research: .agents/research/2026-04-01-01-ux-onboarding-navigation.md
- Telegram API: sendChatAction (typing, upload_photo)
- Handler files: handler_outlook.go, handler_quant.go, handler_cta.go, handler_backtest.go, handler_chart.go
- Paperclip: [PHI-117](/PHI/issues/PHI-117)

## Progress

- [ ] Add sendChatAction to handler_outlook.go
- [ ] Add sendChatAction to handler_quant.go
- [ ] Add sendChatAction to handler_cta.go
- [ ] Add sendChatAction to handler_backtest.go
- [ ] Add sendChatAction/upload_photo to handler_chart.go
- [ ] Add progress messages for multi-step operations
- [ ] Test all commands
- [ ] Create PR
