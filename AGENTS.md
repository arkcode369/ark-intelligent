# AGENTS.md — Konstitusi Multi-Agent ark-intelligent

> Semua agent WAJIB membaca dan mengikuti dokumen ini sebelum melakukan apapun.

---

## Struktur Tim

| Agent | Branch | Role |
|---|---|---|
| **Research** | `agents/research` | Riset rotating focus, buat task spec, kirim laporan ke Telegram |
| **Dev-A** | `agents/dev-a` | Implementasi + Senior Reviewer (merge PR Dev-B & Dev-C) |
| **Dev-B** | `agents/dev-b` | Pure implementor |
| **Dev-C** | `agents/dev-c` | Pure implementor |

---

## Hierarki Branch

```
main                  ← HANYA owner yang merge ke sini
└── agents/main       ← branch integrasi semua agent (selalu harus build clean)
    ├── agents/research
    ├── agents/dev-a
    ├── agents/dev-b
    └── agents/dev-c
```

**ATURAN KERAS:**
- ❌ Tidak ada yang push langsung ke `main`
- ❌ Tidak ada yang merge ke `main` — itu hak owner
- ✅ Semua PR diarahkan ke `agents/main`
- ✅ Sebelum kerja, selalu `git pull origin agents/main`
- ✅ `agents/main` harus selalu dalam kondisi `go build ./...` sukses

---

## Research Agent — Rotating Focus

Research TIDAK mengerjakan semua sekaligus. Setiap siklus punya **satu fokus**:

| Siklus | Fokus | Referensi |
|---|---|---|
| 1 | UX/UI improvement | `.agents/UX_AUDIT.md` |
| 2 | Data & integrasi baru (gratis) | `.agents/DATA_SOURCES_AUDIT.md` |
| 3 | Fitur baru (ICT, SMC, Quant, Wyckoff, dll) | `.agents/FEATURE_INDEX.md` |
| 4 | Technical refactor & tech debt | `.agents/TECH_REFACTOR_PLAN.md` |
| 5 | Bug hunting & edge cases | Codebase + log analysis |
| → rotate ke siklus 1 | | |

**Loop Research (setiap 30-45 menit):**
1. `git pull origin agents/main`
2. Tentukan fokus siklus ini (cek siklus terakhir di STATUS.md)
3. Baca referensi dokumen sesuai fokus
4. Riset mendalam sesuai topik
5. Tulis hasil ke `.agents/research/YYYY-MM-DD-HH-topik.md`
6. Buat **3-5 task spec** di `.agents/tasks/pending/TASK-XXX-nama.md`
7. Update `.agents/STATUS.md`
8. Commit + push ke `agents/research`
9. Kirim laporan ke Telegram owner
10. Tunggu → ulangi dengan fokus berikutnya

**Aturan Research:**
- Jangan buat PR ke `agents/main` — cukup push ke `agents/research`
- Jangan review atau merge PR — itu tugas Dev-A
- Nomor TASK sequential — cek task terakhir di `pending/` + `done/`
- Jangan duplikasi task yang sudah ada

---

## Dev-A — Senior Developer + Reviewer

**Loop Dev-A:**
1. Ambil + implementasi task dari queue (sama seperti Dev-B/C)
2. Setelah selesai implement, cek open PR dari Dev-B dan Dev-C:
   ```bash
   gh pr list --base agents/main
   ```
3. Review setiap PR:
   - `go build ./...` harus clean
   - `go vet ./...` harus clean
   - Logic sesuai task spec (baca acceptance criteria)
   - Tidak ada conflict dengan PR lain
4. Kalau oke → merge: `gh pr merge <number> --merge --delete-branch`
5. Kalau ada issue → comment di PR + buat task `[BLOCKING-XXX]` di `pending/`
6. Update STATUS.md
7. Push ke `agents/dev-a`

**Aturan Dev-A:**
- TIDAK review PR-nya sendiri
- Kalau PR-nya sendiri selesai → tunggu owner review, atau minta Dev-B/C yang review (exceptional)
- Prioritaskan review PR yang sudah lama pending

---

## Dev-B & Dev-C — Pure Implementor

**Loop Dev-B/C (terus-menerus):**
1. `git pull origin agents/main`
2. Cek `.agents/tasks/pending/` — pilih 1 task (high > medium > low)
   - Hindari task yang sudah diclaim di `claimed/`
3. Claim task:
   ```bash
   cp .agents/tasks/pending/TASK-XXX.md .agents/tasks/claimed/TASK-XXX.DEV-B.md
   rm .agents/tasks/pending/TASK-XXX.md
   git add -A && git commit -m "chore: claim TASK-XXX [Dev-B]"
   git push origin agents/dev-b
   ```
4. Buat feature branch dari `agents/main`:
   ```bash
   git checkout agents/main && git pull origin agents/main
   git checkout -b feat/TASK-XXX-nama
   ```
5. Implement sesuai acceptance criteria
6. Build + vet:
   ```bash
   go build ./... && go vet ./...
   ```
7. Commit + push + PR ke `agents/main`:
   ```bash
   git push origin feat/TASK-XXX-nama
   gh pr create --base agents/main --title "feat(TASK-XXX): nama" --body "Closes TASK-XXX"
   ```
8. Pindah task ke done + update STATUS.md:
   ```bash
   mv .agents/tasks/claimed/TASK-XXX.DEV-B.md .agents/tasks/done/
   git checkout agents/dev-b
   git add -A && git commit -m "chore: done TASK-XXX [Dev-B]"
   git push origin agents/dev-b
   ```
9. Langsung ambil task berikutnya

**Aturan Dev-B/C:**
- Kalau build gagal → fix dulu, jangan PR
- Kalau tidak ada task di pending → tunggu 5 menit, cek lagi
- Jangan edit file yang sama dengan agent lain secara bersamaan
- JANGAN BERHENTI — terus ambil task selagi pending queue ada isinya

---

## Format Task Spec

File: `.agents/tasks/pending/TASK-XXX-nama-singkat.md`

```markdown
# TASK-XXX: Nama Task

**Priority:** high / medium / low
**Type:** feature / refactor / fix / ux / data
**Estimated:** S / M / L (S=<2h, M=2-4h, L=4h+)
**Area:** internal/service | internal/adapter | pkg | docs
**Created by:** Research Agent
**Created at:** YYYY-MM-DD HH:MM WIB
**Siklus:** UX / Data / Fitur / Refactor / BugHunt

## Deskripsi
[Apa yang perlu dilakukan]

## Konteks
[Mengapa ini penting — referensi ke dokumen riset]

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] ...kriteria spesifik task...

## File yang Kemungkinan Diubah
- `path/to/file.go`

## Referensi
- `.agents/research/YYYY-MM-DD-topik.md`
- `.agents/TECH_REFACTOR_PLAN.md#TECH-XXX` (untuk refactor tasks)
```

---

## Format Laporan Research ke Telegram

```
🔬 [RESEARCH REPORT]

📌 Fokus Siklus: <UX/Data/Fitur/Refactor/BugHunt>
📖 Topik: <nama topik spesifik>
🕐 <timestamp WIB>

📊 Temuan Utama:
• <poin 1>
• <poin 2>
• <poin 3>

📋 Task Dibuat:
• TASK-XXX: <nama> [high/medium/low]
• TASK-YYY: <nama> [high/medium/low]

🔗 Detail: .agents/research/YYYY-MM-DD-HH-topik.md
```

---

## Git Identity per Agent

```bash
# Research
git config user.name "Agent Research"
git config user.email "research@ark-intelligent.ai"

# Dev-A
git config user.name "Agent Dev-A"
git config user.email "dev-a@ark-intelligent.ai"

# Dev-B
git config user.name "Agent Dev-B"
git config user.email "dev-b@ark-intelligent.ai"

# Dev-C
git config user.name "Agent Dev-C"
git config user.email "dev-c@ark-intelligent.ai"
```

---

## Aturan Commit

```
feat(TASK-XXX): deskripsi singkat       ← fitur baru
fix(TASK-XXX): deskripsi singkat        ← bug fix
refactor(TASK-XXX): deskripsi singkat   ← refactor (no behavior change)
ux(TASK-XXX): deskripsi singkat         ← UX improvement
research: topik yang diriset            ← dari Research agent
chore: claim/done TASK-XXX [Dev-X]      ← task management
```

---

## Conflict Prevention

- Satu task = satu agent (atomic claim via file rename)
- Kalau dua agent claim task yang sama → yang duluan commit claim menang
- Untuk refactor file besar (formatter.go, handler.go): koordinasi via STATUS.md
  - Tulis "Dev-B: working on formatter.go" sebelum mulai
  - Dev lain hindari file tersebut sampai PR merged

---

## Dokumen Referensi

| File | Isi |
|---|---|
| `.agents/FEATURE_INDEX.md` | Semua fitur yang ada + area riset potensial |
| `.agents/UX_AUDIT.md` | 14 UX improvement tasks |
| `.agents/DATA_SOURCES_AUDIT.md` | Status API (free/paid), peluang Firecrawl |
| `.agents/TECH_REFACTOR_PLAN.md` | 15 refactor items, phased execution |
| `.agents/STATUS.md` | Status real-time semua agent |

---

## STATUS.md Template

```markdown
# Agent Status — last updated: YYYY-MM-DD HH:MM WIB

## Research
- **Siklus saat ini:** 1/5 (UX)
- **Last run:** YYYY-MM-DD HH:MM WIB
- **Current:** idle / researching <topik>
- **Tasks created today:** N

## Dev-A
- **Last run:** YYYY-MM-DD HH:MM WIB
- **Current:** idle / working on TASK-XXX / reviewing PR #N
- **PRs merged today:** N
- **PRs pending review:** N

## Dev-B
- **Last run:** YYYY-MM-DD HH:MM WIB
- **Current:** idle / working on TASK-XXX
- **Files being edited:** path/to/file.go (tulis ini untuk prevent conflict)
- **PRs today:** N

## Dev-C
- **Last run:** YYYY-MM-DD HH:MM WIB
- **Current:** idle / working on TASK-XXX
- **Files being edited:** -
- **PRs today:** N
```
