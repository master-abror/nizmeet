# HANDOVER — TRACK S (Server)

- **PIC:** Owner
- **Current Milestone:** 0.1 selesai (tag v0.0.1) → berikutnya 0.3 / 0.4 / C.1
- **Branch / Tag terakhir:** feat/0.1-scaffold / v0.0.1
- **Completed Features:**
  - [x] G.1–G.3 (Fase G)
  - [x] 0.1 Repo scaffolding & ADR awal: `/server` (Go, `slog` hello), `/desktop` (Wails v2 + Svelte 5 + Tailwind), `/web` (SvelteKit + Tailwind), `/packages/ui` (komponen `Button` dipakai di web dan desktop), `go.work`
- **In-Progress / Next Task:** Pastikan Kontributor (BizzyReal, sudah collaborator) menerima onboarding dan membuat PR uji (AC G.1); lalu mulai 0.3 (dev infra lokal), 0.4 (kontrak ekstensibilitas), C.1 (ERD dan DDL).
- **Area berkas milik track ini:** /server, /deploy
- **Key Files Modified:** `go.work`, `package.json`, `pnpm-lock.yaml`, `pnpm-workspace.yaml`, `.gitignore`, `README.md`, `server/`, `desktop/`, `web/`
- **Dependensi lintas track:** kontrak v0 belum dibekukan (C.8). Gunakan mock server (C.7) untuk pekerjaan yang butuh server.
- **Context / Special Notes:** Wails v2 (ADR-0001). Versi: Go 1.27.1, Node v24.21.0 (status "LTS aktif" belum diverifikasi), pnpm 12.15.0, Wails v2.16.0, Svelte 5.57.2, SvelteKit 3.1.0, Vite 8.3.5, Tailwind 4.3.3, sv 1.1.1. `package.json` masih memakai rentang `^`; versi persis dikunci `pnpm-lock.yaml` (pin eksak per Bagian 0 butir 10 belum diterapkan; usul: Milestone 0.2).

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

(belum ada)
