# Panduan Kontribusi

Proyek ini dikerjakan oleh 2 orang (Owner + Kontributor) secara paralel. Panduan ini membuat kita tidak saling menunggu dan tidak saling menimpa. Sumber kebenaran rencana kerja: `docs/MASTER_PLAN_V2.2.md`.

## 1. Sebelum mulai
1. Baca `README.md`, lalu `docs/MASTER_PLAN_V2.2.md` Bagian 1 (arsitektur), Bagian 2 (konvensi), dan Peta Track di Bagian 3.
2. Pasang tool dengan versi yang tercatat di `README.md` (Go, Node, pnpm, Wails v2, Git, `gh`).
3. Baca `docs/git-workflow.md` dan `docs/onboarding/CONTRIBUTOR_ONBOARDING.md`.

## 2. Alur kerja satu milestone
1. Ambil issue berlabel `ready` di track kamu dan tugaskan ke dirimu. Jangan mengerjakan issue berlabel `blocked`.
2. Buat branch dari `main` terbaru: `feat/<id>-<slug>` (contoh `feat/2.1-wails-shell`). Jenis lain: `fix/`, `docs/`, `chore/`.
3. Commit kecil dan sering memakai Conventional Commits: `feat:`, `fix:`, `chore:`, `test:`, `docs:`, `refactor:` (boleh dengan scope: `feat(desktop): ...`).
4. Buka **Draft PR** sejak awal. Ubah menjadi *Ready for review* hanya bila semua AC terpenuhi.
5. Sebelum minta review: `git fetch origin && git rebase origin/main`, jalankan `make test` (setelah Milestone 0.2), perbarui `docs/handover/<track>.md`.
6. Setelah disetujui, **squash merge**. Pesan squash = pesan commit milestone di Master Plan.
7. Hanya Owner yang membuat tag setelah merge.

## 3. Aturan track dan area berkas
| Track | Area | PIC default |
| --- | --- | --- |
| S Server | `/server`, `/deploy` | Owner |
| D Desktop | `/desktop`, `/tools` | Kontributor |
| W Web | `/web` | Fleksibel |
| X Lintas | `/docs`, `/.github`, root config | Owner |

- Jangan mengubah area track lain dalam PR yang sama. Bila perlu, buat PR terpisah dan minta review pemilik area.
- Satu orang atau satu chat room AI = satu track.
- Berkas rawan konflik (`go.work`, `go.mod`, `pnpm-lock.yaml`, `pnpm-workspace.yaml`, `packages/ui`) diubah lewat PR kecil tersendiri.

## 4. Kontrak lintas track
Skema DB, OpenAPI, protokol WebSocket, spesifikasi sync, dan kontrak prompt ada di `docs/contracts/`.
- Butuh mengubah kontrak? Buka issue "Perubahan Kontrak", lalu PR berlabel `contract`. Harus disetujui kedua orang dan di-merge **sebelum** kode yang bergantung padanya.
- Jangan menyimpang diam-diam dari kontrak di kode.

## 5. Review
- Target waktu review: ≤ 2 hari kerja. Saat menunggu, ambil issue `ready` berikutnya.
- Reviewer memeriksa: AC terpenuhi, tes ada dan bermakna, tidak keluar area track, tidak ada rahasia, handover diperbarui.
- Komentar bersifat konstruktif; selesaikan semua percakapan sebelum merge.

## 6. Keamanan dan rahasia
- Jangan commit `.env`, kunci API, token, atau data pribadi. Hanya `.env.example`.
- Job `repo-hygiene` menolak berkas terlarang, tetapi pencegahan tetap tanggung jawab penulis.
- Temuan keamanan: kabari Owner secara langsung, jangan lewat issue publik.

## 7. Checkpoint dan handover
Bila konteks atau waktu kerjamu habis di tengah milestone:
1. Tulis CHECKPOINT STATEMENT (format di Master Plan Bagian 5.4) ke `docs/handover/<track>.md`.
2. `git add -A && git commit -m "wip(<id>): checkpoint"` lalu push; perbarui Draft PR.
3. Orang atau chat room AI berikutnya melanjutkan dari berkas itu.

## 8. Bekerja dengan chat room AI
- Satu room = satu track. Tempel kalimat pembuka dari Master Plan Bagian 5.6 beserta isi `docs/handover/<track>.md`.
- Minta room mengonfirmasi pemahamannya sebelum menulis kode.
- Kode dari AI diperlakukan sama seperti kode manusia: lewat PR, review, dan tes.
