# Agent Orchestration Guide

Dokumen ini menjelaskan setup yang disarankan untuk kerja autonomous 24/7 dengan banyak instance Agent.

---

## 1. Branching Strategy

Rekomendasi utama:
- 1 branch integrasi: `agents/main`
- 1 branch kerja per agent per task
- QA menjadi gate terakhir sebelum merge ke `main`

Struktur yang disarankan:
```
main
└── agents/main
    ├── agents/research
    ├── agents/dev-a
    ├── agents/dev-b
    └── agents/dev-c
```

Prinsip:
- `main` tidak boleh dipush langsung.
- `agents/main` dipakai sebagai branch integrasi bersama.
- Tiap agent bekerja di branch terpisah saat mengerjakan task aktif.
- Branch kerja harus pendek umur dan dihapus setelah merge.

Kenapa bukan monobranch penuh:
- Risiko konflik tinggi.
- Review dan rollback lebih sulit.
- Sulit menjaga ownership task.
- Tidak cocok untuk paralel 24/7.

---

## 2. Branch Naming

Gunakan format berikut:
- Research: `agents/research`
- Dev-A: `agents/dev-a`
- Dev-B: `agents/dev-b`
- Dev-C: `agents/dev-c`
- Task branch: `feat/PHI-123-short-name`
- Fix branch: `fix/PHI-123-short-name`
- Refactor branch: `refactor/PHI-123-short-name`
- Docs branch: `docs/PHI-123-short-name`

Aturan:
- Satu branch = satu task utama.
- Nama branch harus singkat dan mudah dibaca.
- Jangan gabung banyak task dalam satu branch.
- Setelah merge, branch kerja dihapus.

---

## 3. Scheduler Pattern

Gunakan pola scheduler yang sederhana dan deterministik.

Rekomendasi siklus:
- Setiap 5 sampai 10 menit: coordinator cek queue dan status.
- Setiap agent cek inbox/queue lokalnya sebelum mulai kerja.
- Setelah task selesai: agent update status dan release task.
- Setelah PR masuk: QA langsung review jika queue kosong atau prioritas tinggi.

Pola kerja:
1. Coordinator membaca status board.
2. Research menambah task spesifik.
3. Dev mengambil task dari queue.
4. QA memeriksa hasil kerja.
5. Coordinator mengulang prioritas berikutnya.

Jika memakai cron atau background loop:
- Jadikan interval pendek, tetap, dan mudah diprediksi.
- Jangan jalankan terlalu agresif sampai menabrak branch atau task yang sama.
- Pastikan setiap job idempotent.

Contoh aturan operasional:
- Satu job hanya boleh claim satu task.
- Bila task sudah di-claim, job harus skip.
- Bila task gagal, job harus menandai blocker.

---

## 4. 24/7 Workflow by Role

### Coordinator
- Monitor queue, blocker, dan review.
- Prioritaskan task paling unblockable.
- Assign task ke agent yang paling cocok.
- Pastikan tidak ada task menggantung.

### Research
- Audit codebase dan docs.
- Cari gap atau peluang improvement.
- Ubah temuan jadi task spec kecil.
- Serahkan ke coordinator untuk assignment.

### Dev-A
- Ambil task penting atau sensitif.
- Implementasi kecil dan presisi.
- Tambah test atau update test.
- Siapkan untuk review.

### Dev-B
- Ambil task fitur atau bug fix yang cepat selesai.
- Jaga scope tetap kecil.
- Selesaikan end-to-end secepat mungkin.
- Update status setelah validasi.

### Dev-C
- Ambil task integrasi, wiring, migrasi, atau cleanup.
- Jaga kompatibilitas.
- Validasi dependency lintas modul.
- Laporkan risiko integrasi.

### QA
- Review semua PR ke branch integrasi.
- Jalankan build, vet, dan test relevan.
- Tolak perubahan yang belum siap.
- Merge hanya jika aman.

---

## 5. One-Shot Bootstrap Flow

Untuk setup awal repository:
1. Clone atau sync repository.
2. Install dependency yang diperlukan.
3. Siapkan branch integrasi.
4. Buat status board dan role prompts.
5. Siapkan scheduler atau cron.
6. Hubungkan subagent atau instance Agent.
7. Jalankan verifikasi end-to-end.

Target akhir:
- repo siap kerja
- queue siap dipakai
- role prompts siap dipanggil
- scheduler siap jalan
- workflow bisa berjalan terus

---

## 6. Operational Rules

- Claim task sebelum kerja.
- Jangan kerja paralel pada file yang sama.
- Update status setelah milestone penting.
- Eskalasi blocker cepat.
- Jaga setiap perubahan bisa diuji.
- Hindari scope drift.
- Pertahankan kualitas enterprise.

---

## 7. Recommended Files

- [AGENTS.md](AGENTS.md)
- [STATUS.md](.agents/STATUS.md)
- [agent-workflows.md](.agents/docs/agent-workflows.md)
- [roles/README.md](.agents/roles/README.md)
- [.agents/prompts/README.md](.agents/prompts/README.md)

---

## 8. Summary

Rekomendasi paling stabil adalah:
- satu branch integrasi bersama,
- branch kerja terpisah per task,
- scheduler yang deterministik,
- role prompt yang spesifik,
- QA sebagai gate terakhir.

Itu paling cocok untuk workflow autonomous 24/7 yang cepat, rapi, dan masih terasa seperti tim enterprise.
