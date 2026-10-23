# Milestone 0.1 — Instruksi Eksekusi (Repo Scaffolding & ADR Awal)

Dikerjakan Owner di Windows (PowerShell atau Git Bash), **setelah** G.1–G.3. Hasil: monorepo yang dapat dibangun, dengan satu komponen dari `/packages/ui` tampil di desktop dan portal.

> Perintah scaffold (`wails init`, `sv create`) butuh jaringan dan interaktif, jadi dijalankan di mesinmu. Bila ada perbedaan dengan yang tertulis (nama flag, versi template), ikuti `--help` tool terkait dan **catat penyimpangan di `docs/handover/server.md`**.

## 0. Prasyarat dan pin versi
| Tool | Cara |
| --- | --- |
| Git, GitHub CLI | sudah dari Fase G |
| Go ≥ 1.27.1 | https://go.dev/dl — cek `go version`; gunakan patch terbaru |
| Node (LTS aktif) | https://nodejs.org — cek `node -v` |
| pnpm | `corepack enable` lalu `corepack use pnpm@latest` (menulis `packageManager` ke `package.json` root) |
| WebView2 Runtime | bawaan Windows 11; Windows 10 pasang manual |
| Wails CLI v2 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` lalu `wails doctor` |

Catat versi persis (`go version`, `node -v`, `pnpm -v`, `wails version`) ke tabel di `README.md` dan `HANDOVER.md`.

## 1. Branch
```bash
git switch main && git pull --ff-only
git switch -c feat/0.1-scaffold
```

## 2. Struktur direktori
```bash
mkdir -p server/cmd/api server/internal server/migrations deploy tools docs/adr
touch server/internal/.gitkeep server/migrations/.gitkeep deploy/.gitkeep tools/.gitkeep
```
(`packages/ui`, `pnpm-workspace.yaml`, root `package.json`, dan ADR sudah ada dari kit.)

## 3. Modul server
```bash
cd server
go mod init github.com/master-abror/nizmeet/server        # sesuaikan master-abror/nizmeet
cat > cmd/api/main.go <<'GO'
package main

import "log/slog"

func main() {
	slog.Info("copilot server: scaffold ok")
}
GO
go build ./... && cd ..
```

## 4. Scaffold desktop (Wails v2)
```bash
wails init -n copilot-desktop -t svelte-ts -d desktop
```
- Bila Wails membuat folder bersarang (`desktop/copilot-desktop/`), pindahkan isinya naik satu level ke `desktop/`.
- `cd desktop && go mod edit -module github.com/master-abror/nizmeet/desktop`
- Ganti npm dengan pnpm: hapus `frontend/package-lock.json`; di `wails.json` ubah `frontend:install` menjadi `pnpm install`, `frontend:build` menjadi `pnpm run build`, `frontend:dev:watcher` menjadi `pnpm run dev`.
- Di `desktop/frontend/package.json` ubah `"name"` menjadi `@copilot/desktop-frontend`.
- Buat placeholder embed: `touch frontend/dist/.gitkeep` (`.gitignore` sudah memberi pengecualian).

**Penting — versi Svelte harus sama (Svelte 5) di desktop dan web.** Cek `desktop/frontend/package.json`. Bila `svelte` < 5, naikkan Svelte beserta `@sveltejs/vite-plugin-svelte` dan `vite` ke versi kompatibel (ikuti panduan migrasi Svelte 5), lalu di `src/main.ts` ganti `new App({...})` dengan:
```ts
import { mount } from 'svelte';
import App from './App.svelte';
const app = mount(App, { target: document.getElementById('app')! });
export default app;
```
Jangan menyentuh kode Wails selain scaffold; aturan isolasi ada di ADR-0001.

## 5. Scaffold web (SvelteKit)
```bash
pnpm dlx sv create web
```
Pilih: template **SvelteKit minimal**, **TypeScript**, add-on: **tailwindcss**, **eslint**, **prettier**, package manager **pnpm**. Hapus `web/pnpm-lock.yaml` bila terbuat (lockfile ada di root workspace). Adapter produksi diputuskan di Milestone 1C.1.

## 6. Tailwind di desktop
```bash
pnpm --filter @copilot/desktop-frontend add -D tailwindcss @tailwindcss/vite
```
Tambahkan plugin `tailwindcss()` ke `desktop/frontend/vite.config.ts`, dan `@import "tailwindcss";` di CSS global desktop (`src/style.css`).

## 7. Hubungkan `/packages/ui`
```bash
pnpm --filter web add @copilot/ui@workspace:*
pnpm --filter @copilot/desktop-frontend add @copilot/ui@workspace:*
pnpm install
```
Impor token di CSS global masing-masing aplikasi:
- `web/src/app.css`: `@import "../../packages/ui/src/tokens.css";`
- `desktop/frontend/src/style.css`: `@import "../../../packages/ui/src/tokens.css";`

Bila kelak komponen UI bersama memakai kelas Tailwind, tambahkan `@source "../../packages/ui/src";` (web) atau `@source "../../../packages/ui/src";` (desktop) agar kelasnya ikut dibangkitkan.

Pakai komponen untuk memenuhi AC:
```svelte
<!-- web/src/routes/+page.svelte -->
<script lang="ts">
  import { Button } from '@copilot/ui';
</script>
<h1>Portal</h1>
<Button onclick={() => alert('UI bersama OK')}>Tes UI bersama</Button>
```
```svelte
<!-- desktop/frontend/src/App.svelte (sisipkan di markup) -->
<script lang="ts">
  import { Button } from '@copilot/ui';
</script>
<Button onclick={() => alert('UI bersama OK')}>Tes UI bersama</Button>
```

## 8. Go workspace
```bash
go work init ./server ./desktop
```
(`go.work` hanya dikelola Owner.)

## 9. Verifikasi AC
```bash
pnpm --filter @copilot/desktop-frontend build     # isi frontend/dist agar go:embed berhasil
go build ./server/... ./desktop/...               # semua modul Go terbangun
cd desktop && wails dev                           # halaman Svelte + tombol UI bersama muncul
# terminal lain:
pnpm --filter web dev                             # portal + tombol UI bersama muncul
```
- [ ] `wails dev` menampilkan halaman Svelte dan komponen dari `/packages/ui`
- [ ] `pnpm --filter web dev` menampilkan portal dan komponen yang sama
- [ ] `go build` sukses di semua modul Go
- [ ] Versi tool ter-pin dan tercatat di `README.md`
- [ ] ADR-0001 dan ADR-0002 ada di `docs/adr/`

## 10. Commit, PR, tag
```bash
git add -A
git commit -m "chore: scaffold monorepo with desktop, web portal, shared ui, ADRs"
git push -u origin feat/0.1-scaffold
gh pr create --fill
# Kontributor belum diundang: merge sendiri (bypass admin), lalu:
git switch main && git pull --ff-only
git tag -a v0.0.1 -m "Milestone 0.1: repo scaffolding & ADR awal" && git push origin v0.0.1
```
Perbarui `docs/handover/server.md` dan `HANDOVER.md` (milestone aktif, tag, versi ter-pin), lalu `python3 scripts/create_issues.py --apply --done G.1,G.2,G.3,0.1`, dan undang Kontributor (lihat `docs/github-setup-steps.md` bagian 5).

## Jebakan umum
| Gejala | Penyebab / solusi |
| --- | --- |
| `go build` desktop gagal soal `go:embed` | `frontend/dist` kosong → jalankan build frontend atau pastikan `.gitkeep` ada |
| Komponen `@copilot/ui` error di desktop | Svelte di desktop masih < 5 → lihat langkah 4 |
| Kelas Tailwind dari paket UI tidak muncul | Tambahkan `@source` ke paket UI (langkah 7) |
| `wails init` membuat folder bersarang | Pindahkan isinya ke `desktop/` |
| `pnpm install` membuat lockfile ganda | Hapus lockfile di `web/` dan `desktop/frontend/`; hanya satu di root |
| Path berspasi/panjang di Windows | Pindahkan repo ke path pendek, `git config core.longpaths true` |
