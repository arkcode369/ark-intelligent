# PHI-117: TASK-003 Typing Indicator / Progress Feedback for Long Commands

**Status:** completed  
**Assigned to:** Dev-B  
**Priority:** high  
**Type:** feature  
**Estimated:** S  
**Area:** internal/adapter/telegram/handler/  
**Created at:** 2026-04-03 WIB  
**Completed at:** 2026-04-03 WIB  
**Siklus:** UX/UI  
**Branch:** `feat/PHI-117-typing-indicator`  
**Commits:** 445c794, 24624b4

## Deskripsi

Add typing indicator and progress feedback for commands that take 5-15 seconds (/outlook, /quant, /cta). Users often resend commands because they think the bot hung during long operations.

## Implementation

### Files Modified:
- `internal/adapter/telegram/handler_outlook_cmd.go` - Added typing indicator + 3-step progress messages
- `internal/adapter/telegram/handler_quant.go` - Added typing indicator
- `internal/adapter/telegram/handler_cta.go` - Added `upload_photo` typing indicator
- `internal/adapter/telegram/handler_backtest.go` - Added typing indicators to `/backtest`, `/report`, `/accuracy`

### Typing Indicators Added:
| Command | Action Type | Status |
|---------|-------------|--------|
| `/outlook` | `typing` | ✅ |
| `/quant` | `typing` | ✅ |
| `/cta` | `upload_photo` | ✅ |
| `/backtest` | `typing` | ✅ |
| `/report` | `typing` | ✅ |
| `/accuracy` | `typing` | ✅ |

### Multi-step Progress (for /outlook):
- ⏳ Menganalisis... (1/3) Fetching market data...
- ⏳ Menganalisis... (2/3) Processing macro data & AI analysis...
- ⏳ Menganalisis... (3/3) Formatting response...

## Acceptance Criteria

- [x] go build ./... sukses
- [x] go vet ./... sukses
- [x] Typing indicator shows within 1 second of command
- [x] Progress messages for multi-step operations
- [x] No regression on fast commands (<2s)

## Referensi

- Research: .agents/research/2026-04-01-01-ux-onboarding-navigation.md
- Telegram API: sendChatAction (typing, upload_photo)
- Paperclip: [PHI-117](/PHI/issues/PHI-117)
