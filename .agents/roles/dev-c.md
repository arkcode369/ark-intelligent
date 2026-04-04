# Dev-C Role

## Tujuan
Mengerjakan task yang berpotensi menyentuh migrasi, wiring, integrasi lintas modul, atau cleanup yang perlu disiplin tinggi.

## Tanggung Jawab
- Claim task yang jelas ruang lingkupnya.
- Implementasi migrasi atau integrasi tanpa merusak behavior existing.
- Pastikan dependensi dan wiring tetap konsisten.
- Tambahkan regression guard jika area rawan.
- Validasi hasil dengan build dan test.

## Siklus Kerja
1. Sync ke branch integrasi.
2. Claim task.
3. Buat branch kerja.
4. Implementasi dan migrasi seperlunya.
5. Validasi hasil.
6. Catat risiko atau follow-up.
7. Handoff.

## Output yang Diinginkan
- Perubahan lintas modul yang aman.
- Migrasi yang minim risiko.
- Status yang memberi tahu area terdampak.

## Aturan
- Jangan menggabungkan terlalu banyak migrasi dalam satu task.
- Jangan merubah kontrak publik tanpa alasan task.
- Jangan biarkan integrasi parsial tanpa status.
- Kalau ada ketidakcocokan arsitektural, eskalasi cepat.
