# Onboarding Kontributor (Track D — Desktop)

Selamat datang. Dokumen ini cukup untuk mulai bekerja tanpa menunggu siapa pun.

## 1. Proyek dalam 5 baris
- Platform asisten rapat/wawancara real-time: **Desktop Co-Pilot** (Wails v2, Windows) + **Web Portal** (SvelteKit) + **Cloud Backend** (Go).
- Desktop **tipis**: hanya menangkap audio (WASAPI), mengirim lewat satu WebSocket, menampilkan hasil. Tidak ada model atau API key di desktop.
- Semua AI (STT, retrieval, LLM, terjemahan) berjalan di server.
- Cloud adalah sumber kebenaran; SQLite lokal hanya cache baca dengan sync berbasis revisi.
- Kamu memegang **Track D**: semua di `/desktop` dan `/tools`.

## 2. Bacaan (urut, ±1 jam)
1. `README.md`
2. `docs/MASTER_PLAN_V2.2.md`: Bagian 1 (Arsitektur), Bagian 2 (Konvensi), Peta Track di Bagian 3, Fase 2 dan 3
3. `CONTRIBUTING.md` dan `docs/git-workflow.md`
4. `docs/adr/0001-stack.md` (khususnya aturan isolasi Wails) dan `0002-...sync.md`
5. `docs/handover/desktop.md` (status track kamu)

## 3. Setup mesin (Windows)
Pasang sesuai versi di `README.md`: Git, GitHub CLI, Go ≥ 1.27.1, Node LTS, pnpm (`corepack enable`), WebView2 Runtime, Wails CLI v2 (`wails doctor` harus bersih).
```bash
git config --global core.autocrlf false
git config --global core.longpaths true
gh auth login
gh repo clone master-abror/nizmeet && cd nizmeet
pnpm install
```
Verifikasi: `cd desktop && wails dev` menampilkan aplikasi dasar.

## 4. Hari pertama (checklist)
- [ ] Menerima undangan repo (role Write) dan berhasil `clone`
- [ ] `wails dev` jalan di mesinmu
- [ ] Membuat branch + Draft PR uji (mis. perbaiki typo di README) dan mendapat review dari Owner
- [ ] Membaca issue `ready` di track D dan menugaskan satu ke dirimu

## 5. Rencana Gelombang 1 (semua tidak butuh server)
Urutan yang disarankan, masing-masing satu issue/PR:

| Urutan | Milestone | Catatan |
| --- | --- | --- |
| 1 | **0.2 Tooling & CI** | Kecil dan membuka CI untuk semua orang; jadikan job `ci` required check setelah merge (minta Owner) |
| 2 | **3.1 WASAPI loopback** | **Risiko terbesar proyek.** Kerjakan prototipe lebih awal; tanpa CGO bila memungkinkan, fallback dicatat sebagai ADR |
| 3 | **2.1 → 2.2 → 2.3** | Shell Wails + UI, engine/event bus (satu-satunya tempat logika), SQLite cache + FTS5 |
| 4 | **3.2 Mikrofon & mixer** | Setelah 3.1 stabil |
| Paralel | **C.3 dan C.4** | Ikut menulis protokol WebSocket dan spesifikasi sync bersama Owner (kamu klien utamanya) |

Aturan isolasi Wails (ADR-0001): hanya `desktop/main.go` dan satu package bridge yang mengimpor Wails; logika di `desktop/internal/engine` tanpa dependensi Wails.

## 6. Setelah kontrak v0 dibekukan (C.8)
Kamu membuat **C.7 mock server** di `/tools/mockserver`, lalu 2.4, 2.5, dan 3.3 memakai mock itu. Integrasi dengan server asli diuji di titik sinkron (lihat Master Plan Bagian 3).

## 7. Bekerja dengan chat room AI
Satu room = satu track. Kalimat pembuka untuk Track D:

> Saya melanjutkan **Track D (Desktop)** dari Handover **[milestone]**. Aturan: (1) hanya ubah berkas di `/desktop` dan `/tools`; (2) jangan mengubah `docs/contracts/` kecuali lewat PR berlabel `contract`; (3) ikuti `CONTRIBUTING.md` dan ADR-0001 (aturan isolasi Wails v2); (4) konfirmasi pemahamanmu sebelum menulis kode. Berikut isi `docs/handover/desktop.md`: [tempel isi]

Bila konteks room hampir habis: minta CHECKPOINT STATEMENT (Master Plan Bagian 5.4), simpan ke `docs/handover/desktop.md`, commit `wip(<id>): checkpoint`, push, lalu buka room baru dan ulangi kalimat pembuka dengan isi berkas terbaru.

## 8. Cara minta bantuan
- Pertanyaan teknis/keputusan: komentar di issue atau PR terkait.
- Blocker: beri label `blocked` dan tulis penyebabnya di issue; ambil issue `ready` lain.
- Mendesak: hubungi Owner langsung.

## 9. Yang tidak boleh
Push ke `main`; commit `.env`/kunci/token; mengubah `docs/contracts/`, `go.work`, `pnpm-lock.yaml` tanpa PR tersendiri; mengerjakan issue `blocked`; mengubah area track lain dalam PR yang sama.
