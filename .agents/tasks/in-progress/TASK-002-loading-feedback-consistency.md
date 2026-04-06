# TASK-002: Standardize Loading Feedback (SendTyping → SendLoading)

**Priority:** HIGH
**Type:** UX Improvement
**Ref:** UX_AUDIT.md TASK-UX-004, research/2026-04-05-12-ux-audit-cycle1.md
**Status:** In Review
**PR:** #372

---

## Problem

11+ handlers masih menggunakan `SendTyping` saja alih-alih pola `SendLoading` +
`EditMessage`. `SendTyping` hanya menampilkan indikator selama ≤5 detik —
setelah itu user tidak mendapat feedback visual apapun kalau command butuh waktu lama.

Pattern yang benar (sudah dipakai di handler_ict.go, handler_alpha.go, dll):
```go
loadingID, _ := h.bot.SendLoading(ctx, chatID, "⏳ <command>... ⏳")
// ... proses ...
h.bot.EditMessage(ctx, chatID, loadingID, result)
// atau:
h.bot.EditWithKeyboard(ctx, chatID, loadingID, result, kb)
```

---

## Handlers Updated

| File | Command | Loading Message |
|------|---------|-----------------|
| `handler_price.go` | `/price` | "💹 Mengambil data price overview... ⏳" |
| `handler_carry.go` | `/carry` | "💰 Mengambil data carry trades... ⏳" |
| `handler_bis.go` | `/bis` | "🏦 Mengambil data BIS policy rates... ⏳" |
| `handler_onchain.go` | `/onchain` | "⛓️ Mengambil data on-chain... ⏳" |
| `handler_briefing.go` | `/briefing` | "🌅 Memuat daily briefing... ⏳" |
| `handler_levels.go` | `/levels` | "📏 Mengambil data support/resistance levels... ⏳" |
| `handler_scenario.go` | `/scenario` | "📊 Menghitung Monte Carlo scenarios... ⏳" |
| `handler_defi.go` | `/defi` | "🌾 Mengambil data DeFi dashboard... ⏳" |
| `handler_vix_cmd.go` | `/vix` | "📊 Mengambil data CBOE VIX... ⏳" |
| `handler_regime.go` | `/regime` | "📊 Mengambil data regime monitor... ⏳" |
| `handler_cot_compare.go` | `/compare` | "⚖️ Membandingkan data COT... ⏳" |

---

## Acceptance Criteria

- [x] Semua handler di list di atas menggunakan `SendLoading` + `EditMessage`/`EditWithKeyboard`
- [x] Loading message text deskriptif (bukan hanya "⏳ Loading...")
- [x] Error path juga menggunakan `EditMessage` (bukan kirim message baru)
- [x] `go build ./...` bersih

---

## Implementation Notes

Pattern di `handler_orderflow.go` baris 57 hanya `SendTyping` lalu langsung return —
periksa apakah command ini cukup cepat (< 2s) sebelum mengubahnya.

Prioritas upgrade: `/briefing`, `/carry`, `/vix`, `/regime` (semua heavy computations).
