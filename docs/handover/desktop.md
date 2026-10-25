# HANDOVER — TRACK D (Desktop)

- **PIC:** Kontributor (BizzyReal)
- **Current Milestone:** 0.2 Tooling & CI (issue #6). Selesai di sisi Kontributor, menunggu review Owner (Draft PR #66).
- **Branch / Tag terakhir:** branch `feat/0.2-tooling-ci` (PR #66). Tag terakhir di `main`: v0.0.1. Tag `v0.0.2` dibuat Owner setelah merge.
- **Completed Features:**
  - 0.2 Tooling & CI: `Makefile` (`lint`, `test`, `build`, `deps`, `tools`, `doctor`, `zip`), konfigurasi golangci-lint v2 (set `standard`), `svelte-check` lewat `make lint-frontend`, workflow GitHub Actions job `ci` (runner `windows-2025`, `shell: bash`), dan `make zip` (opsional).
  - Acceptance Criteria 0.2 terpenuhi: job `ci` dan `repo-hygiene` hijau di PR #66, dan `make lint test` berjalan lokal dengan 0 issues.
- **In-Progress / Next Task:** Setelah PR 0.2 ter-merge: 2.1 → 2.2 → 2.3, dan prototipe 3.1 → 3.2 lebih awal (risiko terbesar). Backlog dari 0.2:
  - Milestone 2.2: tambahkan `depguard` di `.golangci.yml` untuk aturan isolasi Wails (ADR-0001) saat package `desktop/internal/engine` dibuat.
  - `svelte-check` menyarankan opsi `verbatimModuleSyntax` di `desktop/frontend/tsconfig.json`. Bisa mengubah impor tipe di kode Svelte, jadi dikerjakan terpisah.
  - `make build` (`wails build -s`) belum diuji dan tidak masuk CI: cek apakah `-s` mencegah `package.json.md5` muncul dan apakah Wails mengubah `desktop/frontend/wailsjs/`.
  - Kedua modul Go masih `[no test files]`. Cek kebutuhan PostgreSQL/Redis di CI saat tes pertama ditambahkan.
- **Area berkas milik track ini:** /desktop, /tools. **Pengecualian untuk 0.2:** boleh mengubah `Makefile`, `.github/workflows/`, dan konfigurasi lint di root; PR ditinjau Owner (Master Plan Bagian 2). Menjadikan job `ci` sebagai required status check di ruleset dilakukan Owner.
- **Key Files Modified:**
  - `Makefile`, `tools/zip_source.py`
  - `.golangci.yml`
  - `.github/workflows/ci.yml`
  - `docs/handover/desktop.md`
- **Dependensi lintas track:** kontrak v0 belum dibekukan (C.8). Gunakan mock server (C.7) untuk pekerjaan yang butuh server.
- **Context / Special Notes:** Wails v2 (ADR-0001). Versi ter-pin ada di `README.md` (Go 1.27.1, Node v24.21.0, pnpm 12.15.0, Wails v2.16.0). Baca `docs/MASTER_PLAN_V2.2.md` Bagian 2 dan 5. Jebakan yang sudah ditemukan di 0.1 ada di `docs/handover/server.md` (penyimpangan 1–23), terutama: `vite build` dan `wails dev` menghapus `desktop/frontend/dist/.gitkeep`; `desktop/frontend/package.json.md5` ikut ter-commit; repo jangan di OneDrive; di Git Bash pakai `python`, bukan `python3`; build lulus walau ada peringatan plugin Svelte vs Vite 8. Tambahan dari 0.2: golangci-lint di-pin v2.14.0 hanya di `Makefile` (variabel `GOLANGCI_LINT_VERSION`), dipasang lewat `make tools`, dan CI membacanya lewat `make -s print-golangci-lint-version`. `make` tidak ada di image runner, jadi CI memasangnya lewat Chocolatey. `make ensure-dist` menjaga `desktop/frontend/dist/.gitkeep`. Tanpa `-race` di `go test` karena butuh CGO. `repo-hygiene` tidak dihapus.

## Log Checkpoint (terbaru di atas)

### 2026-10-25 — 0.2 Tooling & CI (tahap d, siap review)

- Tahap (a) sampai (d) selesai di branch `feat/0.2-tooling-ci` (PR #66). Hash commit lama di entri checkpoint di bawah sudah usang karena branch di-rebase ke `main` setelah PR #65 ter-merge. Rujuk PR #66, bukan hash.
- Verifikasi: job `ci` dan `repo-hygiene` hijau di PR. `make lint test` lokal: 0 issues (server dan desktop), svelte-check 0 errors dan 0 warnings.
- Untuk Owner: tambahkan baris `golangci-lint v2.14.0 (di-pin di Makefile)` ke tabel versi `README.md`. Setelah merge: ganti required status check dari `repo-hygiene` ke `ci`, buat tag `v0.0.2`, perbarui `HANDOVER.md`, tutup issue #6.

### 2026-10-24 17:34 — 0.2 Tooling & CI

**CHECKPOINT STATEMENT**
- **Track / Milestone:** D (Desktop) / 0.2 Tooling & CI (issue #6)
- **Branch & commit terakhir:** `feat/0.2-tooling-ci`, commit `14fb361` (tahap a). Draft PR #66.
- **Selesai:** tahap (a): `Makefile` dan `tools/zip_source.py`. Sudah diuji di mesin Kontributor: `make help`, `make doctor`, `make -n lint test build`, `make tools`, `make test`, `make build-server`, `make zip`. Semuanya lolos.
- **Belum dikerjakan:** (b) `.golangci.yml` dan svelte-check, (c) workflow GitHub Actions, (d) update handover ini.
- **Lingkungan Kontributor:** Windows, Git Bash, Go 1.27.3, Node v24.22.1, pnpm 12.15.0, Wails v2.16.0, GNU Make 4.4.1 (via Scoop), golangci-lint 2.14.0 (built with go1.27.3, via `make tools`), Python 3.10.
- **Struktur repo (terkonfirmasi):** `server/go.mod`, `desktop/go.mod`, `go.work` ada. `desktop/frontend/package.json` sudah punya script `build` dan `check` serta devDependency `svelte-check ^4.0.0`, jadi `pnpm-lock.yaml` tidak perlu berubah. `desktop/frontend/dist/.gitkeep` terlacak git.
- **Berkas diubah (di commit 14fb361):**
  - `Makefile` (root): `GOLANGCI_LINT_VERSION ?= v2.14.0`; target `lint` (= `lint-go` + `lint-frontend`, memakai `--config $(CURDIR)/.golangci.yml`), `test`, `build` (= `build-server` + `build-desktop`), `deps`, `tools`, `doctor`, `zip`, `ensure-dist`, `check-golangci-lint`, `print-golangci-lint-version`.
  - `tools/zip_source.py`: mengecualikan `.git`, `node_modules`, `build/`, `*.exe`, `.env` dan `.env.*` (kecuali `.env.example`).
  - **Belum disentuh:** `.golangci.yml`, `.github/workflows/`, `README.md`.
- **Keputusan:**
  1. golangci-lint di-pin v2.14.0 hanya di Makefile. Workflow membaca lewat `make -s print-golangci-lint-version`. Dukungan Go 1.27 mulai v2.13.0.
  2. Dipasang lewat `make tools` (`go install`), bukan `install.sh`. Cara pasang di CI diputuskan di tahap (c).
  3. `repo-hygiene` tidak dihapus. Hanya menambah job `ci`. Mengganti required check adalah tugas Owner setelah merge.
  4. `make build` (`wails build -s`) tidak masuk CI. CI hanya lint dan test.
  5. Tanpa `-race` pada `go test` (butuh CGO; ADR-0001 mensyaratkan SQLite tanpa CGO).
  6. `make zip` mengecualikan semua folder `build/` sesuai Master Plan, termasuk `desktop/build/` milik Wails. Bila perlu, lampirkan terpisah.
  7. Aturan Kontributor: kode `/server` tidak diubah (bila lint atau test server gagal atau butuh PostgreSQL/Redis, berhenti dan laporkan); `pnpm-lock.yaml` dan `go.work` tidak diubah tanpa memberi tahu dulu; kerja per tahap; tiap commit pakai `git add` path eksplisit dan Kontributor menempel `git status --short` dulu.
- **Langkah berikutnya:**
  1. **Tahap (b):** buat `.golangci.yml` di root (format v2, mulai dengan `version: "2"`). Verifikasi `make lint-go` dan `make lint-frontend`. Peringatan plugin Svelte vs Vite 8 bukan kegagalan. Bila temuan lint menumpuk di kode yang bukan milik Track D, longgarkan konfigurasi, jangan memperbaiki kode itu.
  2. **Tahap (c):** buat `.github/workflows/ci.yml` dengan job bernama persis `ci` di runner Windows, `shell: bash`. Pakai versi pin README (Go 1.27.1, Node v24.21.0, pnpm 12.15.0), `pnpm install --frozen-lockfile`, dan versi golangci-lint dari Makefile. Verifikasi dulu apakah `make` tersedia di runner Windows. Jangan ubah `repo-hygiene`.
  3. **Tahap (d):** perbarui berkas ini (Current Milestone, Branch/Tag, Completed Features, Key Files Modified, In-Progress/Next Task, Context).
  4. Sebelum Ready for review: `git fetch origin && git rebase origin/main`, lalu `make test`. Ubah Draft PR #66 menjadi Ready hanya bila semua AC terpenuhi.
- **Belum teruji / pertanyaan terbuka:**
  - `make build` / `wails build -s`: apakah `-s` mencegah `package.json.md5` muncul, dan apakah Wails mengubah `desktop/frontend/wailsjs/`. Cek `git status --short` setelah menjalankannya.
  - `make lint`: baru bisa jalan setelah tahap (b).
  - `make test` lolos lokal, tetapi kedua modul masih `[no test files]`, jadi belum membuktikan apa pun soal kebutuhan PostgreSQL/Redis.
- **Catatan untuk Owner:**
  - Tambahkan baris `golangci-lint v2.14.0 (di-pin di Makefile)` ke tabel versi `README.md` (tidak diubah di PR ini).
  - Setelah merge: ganti required status check dari `repo-hygiene` ke `ci`, buat tag `v0.0.2`, perbarui `HANDOVER.md`, tutup issue #6.

(belum ada)