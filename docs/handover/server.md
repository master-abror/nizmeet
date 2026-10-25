# HANDOVER — TRACK S

- **PIC:** Owner
- **Current Milestone:** C.1 — ERD & DDL Draf (dikerjakan sebagai Track X oleh Owner; dicatat di berkas ini atas permintaan Owner karena skema DB dipakai langsung oleh Track S)
- **Branch / Tag terakhir:** `feat/c.1-erd-ddl` / belum ada tag. Tag rencana `v0.10.1` dibuat Owner di `main` setelah squash merge.
- **Completed Features:**
  - [x] `docs/contracts/db/ddl.sql`: 27 tabel (26 di `public`, 1 di `hello`). Berjalan bersih pada PostgreSQL 18.6 kosong dengan `psql -v ON_ERROR_STOP=1` (exit code 0).
  - [x] `docs/contracts/erd.md`: diagram Mermaid `erDiagram`, penjelasan tiap tabel, konvensi, mekanisme sync, kebijakan retensi (usulan), 8 keputusan desain, 13 OPEN QUESTION.
  - [x] Setiap tabel milik pengguna punya `user_id` dan indeks berawalan `user_id` (diperiksa lewat katalog PG: 0 pelanggaran).
  - [x] Semua tabel punya komentar; daftar tabel DB cocok 27/27 dengan entitas ERD dan baris penjelasan.
  - [x] `pg_dump --schema-only` (18.6) lalu pulih ke DB baru berhasil.
  - [x] Smoke test perilaku (`docs/contracts/db/smoke.sql`) lolos: revisi monoton, soft delete ↔ tombstone, FK komposit lintas-user ditolak, FTS, hapus akun lewat cascade, `audit_log` append-only.
  - [ ] Review Kontributor dari sudut pandang klien sync (AC C.1; menunggu PR).
  - [ ] Diagram Mermaid dikonfirmasi tampil benar di pratinjau GitHub (belum dikonfirmasi).
- **In-Progress / Next Task:**
  1. Buka PR berlabel `contract` + `track:X` dari `feat/c.1-erd-ddl`; minta Kontributor meninjau bagian "Mekanisme sync pada skema" di `erd.md` (bagian 4) dan keputusan D1, D2, D7.
  2. Setelah merge: Owner membuat tag `v0.10.1` dan memperbarui baris di `HANDOVER.md` (indeks).
  3. Lanjut ke **C.2 (OpenAPI v1 draf)**: mulai dari `docs/contracts/erd.md` bagian 3 (daftar tabel) lalu tulis `docs/contracts/openapi/` per modul (auth, licensing, billing, documents, sync, presets, sessions, admin). Setiap field harus ada di DDL atau diberi alasan (checklist C.8).
  4. Selesaikan OPEN QUESTION yang menjadi prasyarat milestone berikutnya: Q1, Q10 (sebelum C.3); Q11, Q13 (sebelum C.4); Q3, Q4, Q9 (di C.6); Q2 (awal 1.8).
- **Area berkas milik track ini:** `docs/contracts/` (Track X), `/server`, `/deploy`
- **Key Files Modified:**
  - `docs/contracts/db/ddl.sql` (baru)
  - `docs/contracts/erd.md` (baru)
  - `docs/contracts/db/smoke.sql` (baru; alat bantu validasi, bukan bagian kontrak)
  - `docs/handover/server.md`
- **Dependensi lintas track:** Kontrak masih draf (belum dibekukan C.8). Track D (desktop 2.3) menyelaraskan skema SQLite lokal dengan DDL ini di C.4. Track W belum bergantung. Milestone 1.2 (migrasi) memecah `ddl.sql` menjadi migrasi per modul; schema `hello` mengikuti 0.4.
- **Context / Special Notes:**
  - **Lingkungan validasi:** PostgreSQL 18.6 (image `postgres:18`), `pg_dump` 18.6. `uuidv7()` bawaan PG 18 terverifikasi; tidak ada ekstensi yang dipakai. Konfigurasi FTS `simple`, `english`, `indonesian` tersedia.
  - **Keputusan Owner:** transkrip sesi live disimpan di cloud sebagai riwayat. Konsekuensinya `live_sessions` dan `transcript_segments` ditambahkan dan ada kind sync ketiga `live_sessions`. Mode "tanpa simpan" (6.2) berarti tidak ada baris di tabel itu. Ini memperluas cakupan di luar daftar tabel eksplisit Master Plan dan memengaruhi Fase 1B (kind sync), 3 (Live Session Service), 2.3 (cache lokal), dan C.6 (peta privasi: ucapan pihak ketiga).
  - **Revisi:** satu counter per pengguna (`sync_counters`), diisi trigger `assign_revision`; tombstone memakai counter yang sama. Hapus akun melewati trigger sync lewat guard `EXISTS (SELECT 1 FROM users ...)`; guard ini terbukti oleh smoke test T5.
  - **Retensi:** mekanisme (indeks waktu untuk job purge) sudah ada di DDL; nilainya hanya usulan di `erd.md` bagian 5 sampai C.6.
  - **Jebakan:** psql membuka pager untuk keluaran lebar (`SELECT version(), uuidv7()`) dan menahan perintah berikutnya; set `PAGER=cat PSQL_PAGER=cat`. Variabel `DB_URL` tidak terbawa ke terminal baru. Menempel skrip panjang ke heredoc terminal dapat terpotong; jalankan dari berkas.
  - **Perubahan skema sesudahnya:** aditif saja; setiap perubahan DDL wajib mengulang `ddl.sql` pada DB kosong dan `smoke.sql`.

## Log Checkpoint (terbaru di atas)

### 2026-10-25 — C.1 (selesai di tingkat draf, bukan checkpoint)
- **Track / Milestone:** X (dicatat di S) / C.1
- **Branch & commit terakhir:** `feat/c.1-erd-ddl` / commit `docs(contracts): add ERD and draft PostgreSQL DDL`
- **Ringkasan:** `ddl.sql` dan `erd.md` selesai dan tervalidasi pada PG 18.6 kosong; menunggu review Kontributor dan konfirmasi render diagram.
- **Berkas diubah:** lihat Key Files Modified.
- **Keputusan yang diambil:** D1–D8 di `erd.md` bagian 7, termasuk transkrip di cloud (D7).
- **Langkah berikutnya (persis):** buka PR berlabel `contract`; setelah merge tag `v0.10.1`; mulai C.2.
- **Blocker / pertanyaan terbuka:** OPEN QUESTION Q1–Q13 di `erd.md` bagian 8.
- **Pesan commit WIP:** tidak ada (milestone selesai dalam satu commit).
