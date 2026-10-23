# ADR-0001: Stack Teknologi dan Batas Arsitektur

- **Status:** Diterima
- **Tanggal:** 2026-10-22
- **Konteks:** Master Plan V2.2, Bagian 0 dan 1.

## Keputusan
| Area | Pilihan |
| --- | --- |
| Server | Go 1.27.x (minimal 1.27.1), router `chi`, PostgreSQL 18, Redis, object storage S3-compatible |
| Desktop | **Wails v2**, frontend Svelte + TypeScript + Tailwind, SQLite lokal (driver tanpa CGO) |
| Web Portal | SvelteKit + Tailwind |
| UI bersama | `/packages/ui` (workspace pnpm) |
| Monorepo | Satu repo; `go.mod` per modul Go + `go.work`; workspace pnpm untuk JS |
| Platform | v1.0 hanya Windows untuk desktop |

## Mengapa Wails v2, bukan v3
Per pengecekan 22 Okt 2026: Wails v3 masih berstatus beta (`v3.0.0-beta.x`). Catatan rilis beta menyatakan API desktop stabil, tetapi tetap prarilis, dan menyebut Wails v2 sebagai rilis stabil saat ini. MVP yang dipasarkan ke pengguna akhir tidak dibangun di atas prarilis.

**Aturan isolasi:** hanya `desktop/main.go` dan satu package bridge (mis. `desktop/internal/wailsapp`) yang boleh mengimpor `github.com/wailsapp/wails/v2`. Logika bisnis ada di `desktop/internal/engine` tanpa dependensi Wails. Migrasi ke v3 ditinjau setelah rilis stabil (backlog).

## Mengapa Go 1.27.x
Go 1.27.0 dirilis 19 Agustus 2026; patch 1.27.1 tersedia sejak awal September 2026. Pin ke patch terbaru yang tersedia saat Milestone 0.1 dan catat di README.

## Konsekuensi
- Tidak ada model, endpoint model, atau API key provider di klien; semua pemrosesan AI di server.
- Versi tool dipin eksak (`go.mod`, `package.json` via `packageManager`, `docker-compose.yml`).
- WASAPI tanpa CGO adalah risiko; prototipe di Milestone 3.1 lebih awal, fallback CGO dicatat sebagai ADR baru bila dipakai.

## Alternatif yang ditolak
- Wails v3 (beta), Electron/Tauri (di luar keahlian stack Go yang dipilih), BYOK di desktop (bertentangan dengan desktop tipis).
