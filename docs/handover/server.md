# HANDOVER — TRACK S (Server)

- **PIC:** Owner
- **Current Milestone:** 0.1 selesai (tag v0.0.1). C.1 (ERD & DDL draf, Track X) selesai di tingkat draf di branch `feat/c.1-erd-ddl`, menunggu review Kontributor dan merge → berikutnya 0.3 / 0.4 / C.2
- **Branch / Tag terakhir:** feat/c.1-erd-ddl / v0.0.1 (tag `v0.10.1` dibuat Owner di `main` setelah squash merge C.1)
- **Completed Features:**
  - [x] G.1–G.3 (Fase G)
  - [x] 0.1 Repo scaffolding & ADR awal: `/server` (Go, `slog` hello), `/desktop` (Wails v2 + Svelte 5 + Tailwind), `/web` (SvelteKit + Tailwind), `/packages/ui` (komponen `Button` dipakai di web dan desktop), `go.work`
  - [x] C.1 (draf, belum merge) ERD & DDL: `docs/contracts/db/ddl.sql` (27 tabel: 26 di `public`, 1 di `hello`; berjalan bersih pada PostgreSQL 18.6 kosong dengan `psql -v ON_ERROR_STOP=1`, exit code 0) dan `docs/contracts/erd.md` (diagram Mermaid, penjelasan tabel, konvensi, mekanisme sync, retensi usulan, keputusan D1–D8, OPEN QUESTION Q1–Q13). Terbukti: setiap tabel ber-`user_id` punya indeks berawalan `user_id` (0 pelanggaran), semua tabel berkomentar, 27 tabel DB = 27 entitas ERD, `pg_dump --schema-only` lalu pulih berhasil, smoke test perilaku (`docs/contracts/db/smoke.sql`) lolos.
  - [ ] C.1: review Kontributor dari sudut pandang klien sync (AC C.1; menunggu PR) dan konfirmasi diagram Mermaid tampil benar di pratinjau PR.
- **In-Progress / Next Task:**
  1. Pastikan Kontributor (BizzyReal, sudah collaborator) menerima onboarding dan membuat PR uji (AC G.1).
  2. Buka PR C.1 dari `feat/c.1-erd-ddl` (label `contract` + `track:X`, mencantumkan `Closes #<nomor issue C.1>`); minta BizzyReal meninjau `docs/contracts/erd.md` bagian 4 (mekanisme sync) dan keputusan D1, D2, D7. Merge dengan squash secara manual (ruleset belum memaksa squash-only, butir 16) dan tanpa bypass admin, karena AC C.1 mensyaratkan review Kontributor.
  3. Setelah merge: Owner membuat tag `v0.10.1` di `main`, memperbarui baris di `HANDOVER.md` (indeks), menutup issue C.1 dan memindahkan kartu ke Done.
  4. Lanjut 0.3 (dev infra lokal), 0.4 (kontrak ekstensibilitas; menyelaraskan schema `hello` dan Q7 dengan ADR-0003), lalu C.2 (OpenAPI v1 draf: mulai dari `docs/contracts/erd.md` bagian 3, satu berkas per modul di `docs/contracts/openapi/`; setiap field harus ada di DDL atau diberi alasan, checklist C.8).
  5. OPEN QUESTION yang menjadi prasyarat: Q1 dan Q10 sebelum C.3; Q11 dan Q13 sebelum C.4; Q3, Q4, Q9 di C.6; Q2 di awal 1.8.
- **Area berkas milik track ini:** /server, /deploy
- **Key Files Modified:** `go.work`, `package.json`, `pnpm-lock.yaml`, `pnpm-workspace.yaml`, `.gitignore`, `README.md`, `server/`, `desktop/`, `web/`; C.1: `docs/contracts/db/ddl.sql`, `docs/contracts/erd.md`, `docs/contracts/db/smoke.sql` (alat bantu validasi, bukan bagian kontrak), `docs/handover/server.md`
- **Dependensi lintas track:** kontrak v0 belum dibekukan (C.8). Gunakan mock server (C.7) untuk pekerjaan yang butuh server. Skema DB draf C.1 ada di `docs/contracts/` (belum merge ke `main`). Track D menyelaraskan skema SQLite lokal (2.3) dengan DDL ini di C.4. Milestone 1.2 memecah `ddl.sql` menjadi migrasi per modul.
- **Context / Special Notes:** Wails v2 (ADR-0001). Versi: Go 1.27.1, Node v24.21.0 (status "LTS aktif" belum diverifikasi), pnpm 12.15.0, Wails v2.16.0, Svelte 5.57.2, SvelteKit 3.1.0, Vite 8.3.5, Tailwind 4.3.3, sv 1.1.1. `package.json` masih memakai rentang `^`; versi persis dikunci `pnpm-lock.yaml` (pin eksak per Bagian 0 butir 10 belum diterapkan; usul: Milestone 0.2).

### Catatan Milestone C.1
- **Lingkungan validasi:** PostgreSQL 18.6 (image `postgres:18`), `pg_dump` 18.6. `uuidv7()` bawaan PG 18 terverifikasi; tidak ada ekstensi yang dipakai. Konfigurasi FTS `simple`, `english`, `indonesian` tersedia.
- **Keputusan Owner:** transkrip sesi live disimpan di cloud sebagai riwayat. Konsekuensinya `live_sessions` dan `transcript_segments` ditambahkan dan ada kind sync ketiga `live_sessions`. Mode "tanpa simpan" (6.2) berarti tidak ada baris di tabel itu. Ini memperluas cakupan di luar daftar tabel eksplisit Master Plan dan memengaruhi Fase 1B (kind sync), 3 (Live Session Service), 2.3 (cache lokal), dan C.6 (peta privasi: ucapan pihak ketiga). ADR-0002 dan Master Plan 2.3/Bagian 6 masih menyebut riwayat sesi hanya sebagai cache lokal; perlu ADR baru atau addendum (keputusan Owner).
- **Revisi:** satu counter per pengguna (`sync_counters`), diisi trigger `assign_revision`; tombstone memakai counter yang sama. Hapus akun melewati trigger sync lewat guard `EXISTS (SELECT 1 FROM users ...)`; terbukti oleh smoke test T5.
- **Retensi:** mekanisme (indeks waktu untuk job purge) sudah ada di DDL; nilainya hanya usulan di `erd.md` bagian 5 sampai C.6.
- **Jebakan:** psql membuka pager untuk keluaran lebar dan menahan perintah berikutnya; set `PAGER=cat PSQL_PAGER=cat`. Variabel `DB_URL` tidak terbawa ke terminal baru. Menempel skrip panjang ke heredoc terminal dapat terpotong; jalankan dari berkas. Menyalin berkas unduhan ke repo dapat menimpa berkas yang sudah ada (handover ini sempat tertimpa lalu dipulihkan) atau mengambil unduhan lama bila namanya bertambah `(1)`; periksa dengan `ls` dan `wc -c` sebelum `git add`.
- **Proses git:** commit C.1 dibuat dari WSL, bukan Git Bash (butir 17 di bawah); identitas git diset lokal di repo (`.git/config`). Samakan dengan identitas commit sebelumnya bila berbeda.
- **Perubahan skema sesudahnya:** aditif saja; setiap perubahan DDL wajib mengulang `ddl.sql` pada DB kosong dan `smoke.sql`.

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

### 2026-10-25 — C.1 (selesai di tingkat draf, bukan checkpoint)
- **Track / Milestone:** X (dicatat di S) / C.1
- **Branch & commit terakhir:** `feat/c.1-erd-ddl` / `docs(contracts): add ERD and draft PostgreSQL DDL` dan susulan `docs(contracts): record FTS config availability in ERD open question Q6`
- **Ringkasan:** `ddl.sql` dan `erd.md` selesai dan tervalidasi pada PG 18.6 kosong; menunggu review Kontributor, konfirmasi render diagram, dan merge.
- **Berkas diubah:** lihat Key Files Modified.
- **Keputusan yang diambil:** D1–D8 di `erd.md` bagian 7, termasuk transkrip di cloud (D7).
- **Langkah berikutnya (persis):** buka PR berlabel `contract`; setelah merge tag `v0.10.1`; lalu 0.3 / 0.4 / C.2.
- **Blocker / pertanyaan terbuka:** OPEN QUESTION Q1–Q13 di `erd.md` bagian 8.
- **Pesan commit WIP:** tidak ada (milestone selesai tanpa checkpoint).
