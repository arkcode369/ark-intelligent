# Technical Refactor Plan — ark-intelligent

> Dokumen ini adalah panduan teknis untuk Research Agent dan Dev-A
> dalam merencanakan refactor, penghapusan tech debt, dan peningkatan kualitas kode.

---

## ⚠️ Aturan Refactor untuk Dev Agents

1. **Satu PR = satu refactor item** — jangan gabung TECH-001 dan TECH-002 dalam satu PR
2. **Refactor = NO behavior change** — kalau ada behavior change, itu bukan refactor, itu feature
3. **Wajib build clean** sebelum PR: `go build ./... && go vet ./...`
4. **Wajib test existing** tidak ada yang break: `go test ./...`
5. **Kalau ragu** — buat task di `pending/` dengan tag `[NEEDS DISCUSSION]` dan notif Dev-A

---

## 📜 Update Log
