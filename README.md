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
| Go | 1.27.x (minimal 1.27.1) — isi: `____` |
| Node (LTS aktif) | `____` |
| pnpm | `____` |
| Wails CLI (v2) | `____` |
| PostgreSQL / Redis | 18.`__` / `____` (Milestone 0.3) |

## Memulai
Lihat `CONTRIBUTING.md` dan `docs/onboarding/CONTRIBUTOR_ONBOARDING.md`.
