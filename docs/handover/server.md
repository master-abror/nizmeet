# HANDOVER — TRACK S (Server)

- **PIC:** Owner
- **Current Milestone:** 0.1 selesai (tag v0.0.1), 0.2 selesai (tag v0.0.2, Track D), C.1 selesai (tag v0.10.1, Track X); 0.3 berkas `/deploy` selesai dan tervalidasi di branch `feat/0.3-dev-infra` (belum di-merge; `make dev-up` menunggu PR Makefile; tag v0.0.3 belum dibuat) → berikutnya 0.4 / C.2
- **Branch / Tag terakhir:** `feat/0.3-dev-infra` (kerja 0.3) · main / v0.10.1 (commit squash `6978e6d`, PR #67)
- **Completed Features:**
  - [x] G.1–G.3 (Fase G)
  - [x] 0.1 Repo scaffolding & ADR awal: `/server` (Go, `slog` hello), `/desktop` (Wails v2 + Svelte 5 + Tailwind), `/web` (SvelteKit + Tailwind), `/packages/ui` (komponen `Button` dipakai di web dan desktop), `go.work`
  - [x] C.1 (selesai; PR #67 squash `6978e6d`, tag `v0.10.1`; kontrak masih draf sampai C.8) ERD & DDL: `docs/contracts/db/ddl.sql` (27 tabel: 26 di `public`, 1 di `hello`; berjalan bersih pada PostgreSQL 18.6 kosong dengan `psql -v ON_ERROR_STOP=1`, exit code 0) dan `docs/contracts/erd.md` (diagram Mermaid, penjelasan tabel, konvensi, mekanisme sync, retensi usulan, keputusan D1–D11, OPEN QUESTION Q1–Q14). Terbukti: setiap tabel ber-`user_id` punya indeks berawalan `user_id` (0 pelanggaran), semua tabel berkomentar, 27 tabel DB = 27 entitas ERD, `pg_dump --schema-only` lalu pulih berhasil, smoke test perilaku (`docs/contracts/db/smoke.sql`) lolos.
  - [x] C.1: review Kontributor dari sudut pandang klien sync (AC C.1). BizzyReal mereview (7 poin, tanpa perubahan tabel); ditanggapi di `erd.md` bagian 4 dan 8 (keputusan D9 sampai D11, Q14), lalu **Approved**. Diagram Mermaid dikonfirmasi tampil di pratinjau PR. Issue #9 CLOSED.
  - [x] 0.3 (sebagian; tag belum dibuat): `deploy/docker-compose.yml` (PostgreSQL 18.6 + Redis 8.10.3, dipin eksak), `deploy/.env.example`, `deploy/scripts/dev-up.sh`. Terbukti oleh Owner (2026-10-27): `dev-up.sh` dijalankan dua kali (kedua kali sehat), `pg_isready` "accepting connections", `redis-cli ping` membalas `PONG`, `select version()` = PostgreSQL 18.6 dengan `data_directory` `/var/lib/postgresql/18/docker`, `deploy/.env` diabaikan git, password tidak ada di berkas lain. Belum: target `make dev-up` (menunggu PR Makefile), tag `v0.0.3`.
- **In-Progress / Next Task:**
  1. Pastikan Kontributor (BizzyReal, sudah collaborator) menerima onboarding dan membuat PR uji (AC G.1).
  2. Baris C.1 dan 0.2 di `HANDOVER.md` (indeks) diperbarui lewat PR #68 (`a9824b4`, "update handover index after C.1 and 0.2"). Masih Owner: periksa kartu #9 di board (issue sudah CLOSED).
  3. Owner: ruleset `main-protection`: jadikan job `ci` required status check (sudah terbukti hijau pada PR docs-only #67); putuskan nasib `repo-hygiene` dan squash-only (butir 16). Status perubahan ini belum dikonfirmasi.
  4. Selesaikan 0.3 (lihat Catatan Milestone 0.3: merge PR 0.3, PR Makefile untuk `dev-up` ditinjau BizzyReal, lalu Owner membuat tag `v0.0.3`), lanjut 0.4 (kontrak ekstensibilitas; menyelaraskan schema `hello` dan Q7 dengan ADR-0003), lalu C.2 (OpenAPI v1 draf: mulai dari `docs/contracts/erd.md` bagian 3, satu berkas per modul di `docs/contracts/openapi/`; setiap field harus ada di DDL atau diberi alasan, checklist C.8).
  5. OPEN QUESTION yang menjadi prasyarat: Q1 dan Q10 sebelum C.3; Q14 sebelum C.4 (Q11 dan Q13 sudah terjawab, D10 dan D11); Q3, Q4, Q9 di C.6; Q2 di awal 1.8.
  6. Syarat untuk spesifikasi berikutnya (dari review PR #67): C.3, event transkrip final di WebSocket membawa `session_id` dan `seq` yang sama dengan `transcript_segments`. C.4, tulis kontrak `409` (membawa data server terbaru), rekonsiliasi sync penuh (hapus baris lokal yang tidak ada di server), aturan cursor kedaluwarsa lewat `sync_cursors.updated_at`, dan invarian set chunk (`erd.md` bagian 4 butir 8).
- **Area berkas milik track ini:** /server, /deploy
- **Key Files Modified:** `go.work`, `package.json`, `pnpm-lock.yaml`, `pnpm-workspace.yaml`, `.gitignore`, `README.md`, `server/`, `desktop/`, `web/`; C.1: `docs/contracts/db/ddl.sql`, `docs/contracts/erd.md`, `docs/contracts/db/smoke.sql` (alat bantu validasi, bukan bagian kontrak), `docs/handover/server.md`; 0.3: `deploy/docker-compose.yml`, `deploy/.env.example`, `deploy/scripts/dev-up.sh`, `docs/handover/server.md`
- **Dependensi lintas track:** kontrak v0 belum dibekukan (C.8). Gunakan mock server (C.7) untuk pekerjaan yang butuh server. Skema DB draf C.1 ada di `docs/contracts/` (sudah di `main`, tag `v0.10.1`). Track D menyelaraskan skema SQLite lokal (2.3) dengan DDL ini di C.4. Milestone 1.2 memecah `ddl.sql` menjadi migrasi per modul.
- **Context / Special Notes:** Wails v2 (ADR-0001). Versi: Go 1.27.1, Node v24.21.0 (status "LTS aktif" belum diverifikasi), pnpm 12.15.0, Wails v2.16.0, Svelte 5.57.2, SvelteKit 3.1.0, Vite 8.3.5, Tailwind 4.3.3, sv 1.1.1. Dev infra (0.3): PostgreSQL 18.6 dan Redis 8.10.3 (pin eksak di `deploy/docker-compose.yml`); Docker Engine 29.7.2 hanya versi server yang teramati, bukan pin. `package.json` masih memakai rentang `^`; versi persis dikunci `pnpm-lock.yaml` (pin eksak per Bagian 0 butir 10 belum diterapkan; usul: Milestone 0.2).

### Catatan Milestone C.1
- **Lingkungan validasi:** PostgreSQL 18.6 (image `postgres:18`), `pg_dump` 18.6. `uuidv7()` bawaan PG 18 terverifikasi; tidak ada ekstensi yang dipakai. Konfigurasi FTS `simple`, `english`, `indonesian` tersedia.
- **Keputusan Owner:** transkrip sesi live disimpan di cloud sebagai riwayat. Konsekuensinya `live_sessions` dan `transcript_segments` ditambahkan dan ada kind sync ketiga `live_sessions`. Mode "tanpa simpan" (6.2) berarti tidak ada baris di tabel itu. Ini memperluas cakupan di luar daftar tabel eksplisit Master Plan dan memengaruhi Fase 1B (kind sync), 3 (Live Session Service), 2.3 (cache lokal), dan C.6 (peta privasi: ucapan pihak ketiga). Selaras dengan ADR-0002 butir 1 dan Master Plan Bagian 0 butir 2 (SQLite = cache baca untuk riwayat sesi yang dapat dibangun ulang dari cloud), sehingga tidak perlu ADR baru; yang perlu diselaraskan adalah teks 2.3/3.x (desktop "menyimpan ke `transcripts`" lokal) dan peta privasi C.6.
- **Revisi:** satu counter per pengguna (`sync_counters`), diisi trigger `assign_revision`; tombstone memakai counter yang sama. Hapus akun melewati trigger sync lewat guard `EXISTS (SELECT 1 FROM users ...)`; terbukti oleh smoke test T5.
- **Retensi:** mekanisme (indeks waktu untuk job purge) sudah ada di DDL; nilainya hanya usulan di `erd.md` bagian 5 sampai C.6.
- **Jebakan:** psql membuka pager untuk keluaran lebar dan menahan perintah berikutnya; set `PAGER=cat PSQL_PAGER=cat`. Variabel `DB_URL` tidak terbawa ke terminal baru. Menempel skrip panjang ke heredoc terminal dapat terpotong; jalankan dari berkas. Menyalin berkas unduhan ke repo dapat menimpa berkas yang sudah ada (handover ini sempat tertimpa lalu dipulihkan) atau mengambil unduhan lama bila namanya bertambah `(1)`; periksa dengan `ls` dan `wc -c` sebelum `git add`.
- **Proses git:** commit C.1 dibuat dari WSL, bukan Git Bash (butir 17 di bawah); identitas git diset lokal di repo (`.git/config`). Samakan dengan identitas commit sebelumnya bila berbeda.
- **Perubahan skema sesudahnya:** aditif saja; setiap perubahan DDL wajib mengulang `ddl.sql` pada DB kosong dan `smoke.sql`.

### Catatan Milestone 0.3
- **Cakupan:** `deploy/docker-compose.yml` (proyek `nizmeet-dev`; `postgres:18.6`, `redis:8.10.3`; port hanya di `127.0.0.1`; healthcheck `pg_isready` dan `redis-cli ping`; volume bernama `pgdata`), `deploy/.env.example` (hanya variabel yang dipakai compose), `deploy/scripts/dev-up.sh`. Di luar cakupan: skema DB (1.2), object storage, variabel server/provider (`.env.example` penuh di 1.6).
- **Keputusan Owner (2026-10-27):** Makefile (milik Track D) tidak diubah di PR 0.3. Logika ada di skrip `/deploy`; Makefile hanya mendapat satu target `dev-up` yang memanggilnya, lewat PR terpisah yang ditinjau BizzyReal (tanpa `--admin`). Tag `v0.0.3` dibuat setelah PR Makefile ter-merge dan `make dev-up` tervalidasi. Tanda 🚩 di 0.3 diperlakukan sebagai PR biasa; gate Fase 0 ada di 0.4.
- **Kredensial:** compose tidak punya nilai bawaan untuk user, password, dan db (`${VAR:?}`). `dev-up.sh` membuat `deploy/.env` dengan password acak bila belum ada, menolak password contoh, dan tidak mencetak password. Redis tanpa password dan hanya di loopback: khusus dev lokal.
- **Jebakan:** image PostgreSQL 18+ memakai `PGDATA` `/var/lib/postgresql/18/docker`; volume dipasang di `/var/lib/postgresql` (bukan `.../data`). Docker Desktop harus berjalan (bila tidak, muncul error `npipe:////./pipe/dockerDesktopLinuxEngine`). Kontainer `deploy-postgres-1` dan `deploy-mailpit-1` (proyek compose lain bernama `deploy`, status Exited, terikat ke `0.0.0.0:5432`, `1025`, `8025`) ditemukan di mesin Owner; asalnya tidak diverifikasi dan bukan dari berkas 0.3. Bila kontainer itu hidup lagi, `dev-up.sh` gagal karena port 5432 bentrok: ubah `POSTGRES_PORT` di `deploy/.env`. Kartu unduhan menamai `.env.example` sebagai `env.example`; salin dengan nama tujuan eksplisit. `.gitignore` root mengabaikan `.env` tetapi mengecualikan `.env.example` (`!.env.example`).
- **Belum selesai:** PR Makefile (target `dev-up`), tag `v0.0.3`; pencatatan versi ter-pin di `README.md` dan `HANDOVER.md` (indeks) oleh Owner, karena di luar area Track S.
- **OPEN QUESTION:** object storage (MinIO/R2) disebut di Master Plan Bagian 0 butir 4 tetapi tidak ada di tugas 0.3 dan belum ada milestone yang menetapkan kapan masuk compose; belum ditambahkan.

### Penyimpangan dari `docs/milestone-0.1-instructions.md`
1. Repo dipindah dari OneDrive ke `C:\Users\maste\NizMeet\REPO` (OneDrive menyebabkan error `go: ... cloud file provider is not running`).
2. pnpm dipin (`corepack use`) setelah branch dibuat; perintah itu ikut menjalankan install.
3. `desktop/go.mod`: `go 1.25.0` dinaikkan ke `1.27.1`.
4. `desktop/frontend/src/main.ts`: memakai `mount()` (Svelte 5) dan tetap mengimpor `./style.css`.
5. `desktop/.gitignore` ditulis ulang (`frontend/dist/*` + `!frontend/dist/.gitkeep`); pola lama `frontend/dist` mengabaikan `.gitkeep`.
6. `.gitignore` root: ditambah `!desktop/build/` karena aturan `build/` mengabaikan aset build Wails (ikon, manifest, installer).
7. `sv create` mengabaikan `--no-install` (membuat `web/node_modules` dan `package-lock.json`, keduanya dihapus) dan tidak memasang Tailwind; Tailwind ditambah dengan `sv add tailwindcss="plugins:none"`.
8. CSS global web adalah `web/src/routes/layout.css` (bukan `app.css`); impor token: `../../../packages/ui/src/tokens.css`.
9. pnpm 12 menambah `allowBuilds` di `pnpm-workspace.yaml`; `svelte-preprocess: false` (skripnya hanya `echo`).
10. `Button` di `desktop/frontend/src/App.svelte` disisipkan tepat setelah `<main>`.
11. `vite build` dan `wails dev` menghapus `desktop/frontend/dist/.gitkeep`; buat ulang (`touch`) sebelum `git add`.
12. Peringatan yang diketahui, build tetap lulus: `@sveltejs/vite-plugin-svelte@6.2.4` (desktop) menyatakan dukungan `vite ^6.3 || ^7`, terpasang Vite 8.3.5 (eksperimental); `verbatimModuleSyntax` belum diset di `tsconfig.json` desktop. Kandidat perbaikan: samakan plugin desktop dengan web (`^7.2.0`).
13. Dipertahankan atas keputusan Owner: email di blok `author` `desktop/wails.json`, dan komentar `// replace ...` (path lokal) di `desktop/go.mod`.
14. `desktop/frontend/package.json.md5` ikut ter-commit (kemungkinan dibuat Wails saat `wails dev`; belum diverifikasi). Opsi: hapus dan tambahkan ke `.gitignore`.
15. Ruleset `main-protection` awalnya `bypass_actors: []`, sehingga `gh pr merge --admin` ditolak. Ditambahkan bypass `Repository admin` (mode `pull_request`). Ini berbeda dari `enforce_admins: false` di `docs/github-setup-steps.md`.
16. Ruleset: `required_status_checks` kosong (dokumen meminta `repo-hygiene` wajib dan branch up-to-date) dan `allowed_merge_methods` memuat merge, squash, dan rebase (master plan: hanya squash). Belum diubah.
17. Git Bash tidak punya `python3`; pakai `python` (3.12.10). Perintah yang mengubah repo dijalankan di Git Bash, bukan WSL.
18. Issue baru dibuat setelah merge 0.1 (sebelumnya tidak ada issue). PR #1 memakai nomor #1, jadi issue mulai dari #2 (G.1 = #2, 0.1 = #5). Pembuatan sempat terputus oleh error jaringan di issue ke-24; total akhir 60 issue.
19. Penugasan issue ke Kontributor ditunda sampai undangan diterima: `--contrib-login` membuat script berhenti bila akun belum menjadi collaborator.
20. BizzyReal sudah collaborator (role write, terkonfirmasi lewat API); CODEOWNERS valid (`codeowners/errors` kosong). 18 issue ber-PIC Kontributor ditugaskan ke BizzyReal.
21. 56 issue open: 27 ke master-abror, 18 ke BizzyReal, 11 tanpa assignee (PIC "Berdua": 7.4, 7.1, 4.7, C.8, C.4, C.3; PIC "Fleksibel": 1C.1 sampai 1C.5). Script hanya menugaskan PIC Owner dan Kontributor.
22. Project board belum ada saat G.3 ditutup. Dibuat lewat CLI setelah 0.1: `NizMeet Board` (project 1, private), ditautkan ke repo, 60 issue ditambahkan lewat `gh project item-add`. Empat issue selesai (G.1 sampai G.3, 0.1) berstatus Done. Opsi Status memakai bawaan GitHub (Todo, In Progress, Done) dan dipertahankan; lima kolom di dokumen tidak diterapkan.
23. Terbuka (keputusan Owner): required status checks dan squash-only di ruleset, `package.json.md5`, pin eksak versi, status LTS Node, peringatan Vite 8 di desktop, dan akses BizzyReal ke board (board private; izin project terpisah dari izin repo).

## Log Checkpoint (terbaru di atas)

### 2026-10-27 — 0.3 berkas selesai (batas parsial, menunggu PR Makefile)
- **Track / Milestone:** S / 0.3
- **Branch & commit terakhir:** `feat/0.3-dev-infra` (base `main` `a9824b4`); tag belum dibuat
- **Ringkasan:** compose, `.env.example`, dan `dev-up.sh` selesai dan tervalidasi pada Docker Engine 29.7.2 (bukti di Completed Features). `make dev-up` belum ada.
- **Berkas diubah:** `deploy/docker-compose.yml`, `deploy/.env.example`, `deploy/scripts/dev-up.sh`, `docs/handover/server.md`
- **Keputusan yang diambil:** opsi A untuk Makefile; 🚩 di 0.3 = PR biasa; tag Redis dipin ke versi yang dilaporkan `redis:latest` (8.10.3) setelah `docker pull redis:8.10.3` berhasil; lihat Catatan Milestone 0.3.
- **Langkah berikutnya (persis):** (1) approve dan squash merge PR 0.3; (2) branch `chore/` dari `main` terbaru: tambah target `dev-up` di `Makefile` yang memanggil `bash deploy/scripts/dev-up.sh`, PR ditinjau BizzyReal; (3) setelah merge, jalankan `make dev-up`, lalu Owner membuat tag `v0.0.3` pada commit hasil squash PR Makefile; (4) Owner memperbarui `HANDOVER.md` dan `README.md` (versi ter-pin); (5) lanjut 0.4.
- **Blocker / pertanyaan terbuka:** object storage di compose (lihat Catatan Milestone 0.3).
- **Pesan commit WIP:** tidak ada

### 2026-10-26 — C.1 selesai (batas milestone, bukan checkpoint)
- **Track / Milestone:** X (dicatat di S) / C.1
- **Branch & commit terakhir:** `main` / `6978e6d docs(contracts): add ERD and draft PostgreSQL DDL (#67)`, tag `v0.10.1`
- **Ringkasan:** `ddl.sql` dan `erd.md` selesai, tervalidasi pada PG 18.6 kosong (termasuk validasi ulang setelah review), di-approve BizzyReal, dan ter-merge. Isi `main` identik dengan commit yang disetujui.
- **Berkas diubah:** lihat Key Files Modified.
- **Keputusan yang diambil:** D1–D11 di `erd.md` bagian 7, termasuk transkrip di cloud (D7), arah sync per kind (D9), aturan chunk vs cursor (D10), dan segmen per `seq` (D11).
- **Langkah berikutnya (persis):** Owner perbarui `HANDOVER.md` dan ruleset; lalu 0.3 (#7), 0.4 (#8), C.2.
- **Blocker / pertanyaan terbuka:** OPEN QUESTION Q1–Q14 di `erd.md` bagian 8 (Q11 dan Q13 terjawab).
- **Pesan commit WIP:** tidak ada (milestone selesai tanpa checkpoint).
