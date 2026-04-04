# Agent Role Pack

Dokumen ini berisi instruksi operasional per peran untuk orkestrasi multi-instance Agent.

Prinsip umum:
- Bekerja 24/7 dengan siklus pendek dan output nyata.
- Claim satu task, selesaikan, laporkan, lalu ambil task berikutnya.
- Jaga kualitas enterprise: kecil, terukur, bisa diuji, dan terdokumentasi.
- Jangan mengerjakan task yang belum di-claim.
- Jangan bercampur peran jika kamu sedang memegang satu role.

Standar bersama:
- Selalu sinkron ke branch integrasi sebelum mulai.
- Kerjakan satu task per branch.
- Pastikan build dan vet/test lulus sebelum handoff.
- Tulis status singkat dan jelas setiap selesai milestone.
- Eskalasi blocker secepatnya, jangan menunggu macet lama.
