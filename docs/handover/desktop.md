# HANDOVER — TRACK D (Desktop)

- **PIC:** Kontributor (BizzyReal)
- **Current Milestone:** 0.2 Tooling & CI (issue #6)
- **Branch / Tag terakhir:** belum ada branch kerja / tag terakhir di `main`: v0.0.1
- **Completed Features:**
  - (belum ada)
- **In-Progress / Next Task:** Kerjakan 0.2 di branch `feat/0.2-tooling-ci`: `Makefile` (`lint`, `test`, `build`), golangci-lint, `svelte-check`, GitHub Actions (lint dan test server serta desktop di runner Windows). Lalu 2.1 → 2.2 → 2.3, dan prototipe 3.1 → 3.2 lebih awal (risiko terbesar).
- **Area berkas milik track ini:** /desktop, /tools. **Pengecualian untuk 0.2:** boleh mengubah `Makefile`, `.github/workflows/`, dan konfigurasi lint di root; PR ditinjau Owner (Master Plan Bagian 2). Menjadikan job `ci` sebagai required status check di ruleset dilakukan Owner.
- **Key Files Modified:**
  - (belum ada)
- **Dependensi lintas track:** kontrak v0 belum dibekukan (C.8). Gunakan mock server (C.7) untuk pekerjaan yang butuh server.
- **Context / Special Notes:** Wails v2 (ADR-0001). Versi ter-pin ada di `README.md` (Go 1.27.1, Node v24.21.0, pnpm 12.15.0, Wails v2.16.0). Baca `docs/MASTER_PLAN_V2.2.md` Bagian 2 dan 5. Jebakan yang sudah ditemukan di 0.1 ada di `docs/handover/server.md` (penyimpangan 1–23), terutama: `vite build` dan `wails dev` menghapus `desktop/frontend/dist/.gitkeep`; `desktop/frontend/package.json.md5` ikut ter-commit; repo jangan di OneDrive; di Git Bash pakai `python`, bukan `python3`; build lulus walau ada peringatan plugin Svelte vs Vite 8.

## Log Checkpoint (terbaru di atas)

(belum ada)