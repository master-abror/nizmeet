# Real-Time Meeting & Interview Co-Pilot

Platform dengan dua permukaan: **Desktop Co-Pilot** (Wails v2, Windows) yang menangkap audio dan menampilkan saran, serta **Web Portal** (SvelteKit) untuk akun, billing, lisensi, dan Library. Seluruh pemrosesan AI (STT, Fast-Pass, retrieval, LLM, terjemahan) berjalan di **Cloud Backend** (Go).

Status: tahap perencanaan dan scaffolding. Rencana lengkap: [`docs/MASTER_PLAN_V2.2.md`](docs/MASTER_PLAN_V2.2.md).

## Struktur repo
```
/server        Cloud API + pipeline sesi (Go)
/desktop       Co-Pilot desktop (Wails v2 + Svelte)
/web           Web Portal (SvelteKit)
/packages/ui   Komponen UI bersama (Svelte)
/tools         Alat bantu (mock server, dll.)
/deploy        Docker/infra
/docs          Master plan, kontrak, ADR, handover, onboarding
```

## Dokumen penting
| Dokumen | Isi |
| --- | --- |
| `docs/MASTER_PLAN_V2.2.md` | Spesifikasi, checklist milestone, peta track |
| `CONTRIBUTING.md` | Aturan kerja tim |
| `docs/git-workflow.md` | Git, branch, tag, konflik |
| `docs/contracts/` | Kontrak lintas track (diisi di Fase C) |
| `docs/adr/` | Keputusan arsitektur |
| `HANDOVER.md` + `docs/handover/` | Status per track dan checkpoint |
| `docs/onboarding/` | Panduan kontributor baru |

## Versi tool (isi dan pin saat Milestone 0.1)
| Tool | Versi |
| --- | --- |
| Go | 1.27.1 |
| Node (LTS aktif) | v24.21.0 |
| pnpm | 12.15.0 (dipin lewat `packageManager` di `package.json` root) |
| Wails CLI (v2) | v2.16.0 |
| Svelte | 5.57.2 (web dan desktop) |
| SvelteKit | 3.1.0 (web) |
| Vite | 8.3.5 (web dan desktop) |
| Tailwind CSS | 4.3.3 (web dan desktop) |
| Svelte CLI (`sv`) | 1.1.1 |
| PostgreSQL / Redis | 18.`__` / `____` (Milestone 0.3) |

## Memulai
Baca `CONTRIBUTING.md` dan `docs/onboarding/CONTRIBUTOR_ONBOARDING.md`.
