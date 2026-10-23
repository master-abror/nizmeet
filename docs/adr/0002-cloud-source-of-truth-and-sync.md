# ADR-0002: Cloud sebagai Sumber Kebenaran dan Mekanisme Sync

- **Status:** Diterima
- **Tanggal:** 2026-10-22
- **Konteks:** Master Plan V2.2, Bagian 0 butir 2–5 dan Fase 1B.

## Keputusan
1. **Cloud = sumber kebenaran.** SQLite lokal hanya cache baca (Library, riwayat sesi). Aliran: desktop/portal → cloud → desktop. Desktop tidak pernah menjadi master data.
2. **Sync berbasis revisi, bukan timestamp.** Setiap dokumen punya `revision` yang naik monoton; setiap device punya cursor; penghapusan memakai tombstone.
3. **Konflik v1.0: server menang per dokumen.** Versi lama tetap tersimpan dan dapat dipulihkan.
4. **Mutasi dari desktop** masuk antrian lokal (outbox), idempoten lewat `client_mutation_id`, dikirim dengan `If-Match: <revision>`; server membalas `409` bila revisi usang.
5. **File asli di object storage, teks di PostgreSQL.** Chunking dilakukan server agar indeks konsisten di semua device.

## Privasi
Data karier sensitif: enkripsi at-rest di cloud, semua query ber-scope `user_id` (dengan test isolasi), hapus akun = hapus seluruh data, ekspor data pribadi. Catatan cache lokal: SQLCipher membutuhkan CGO sehingga tidak dipakai secara default; opsi enkripsi cache lokal ditinjau di Milestone 2.3/6.2.

## Konsekuensi
- Desktop perlu logika outbox dan resolusi `409` (Milestone 2.6).
- Server perlu menyimpan riwayat versi dan tombstone (Milestone 1B.1–1B.3).
- Spesifikasi detail ditulis di `docs/contracts/sync-spec.md` (Milestone C.4).

## Alternatif yang ditolak
- Sync berbasis timestamp (rawan jam device keliru), CRDT/merge tingkat-field (kompleks untuk v1.0; masuk backlog).
