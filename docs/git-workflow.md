# Git Workflow (tim 2 orang)

## Model
Trunk-based dengan branch pendek. `main` selalu bisa dibangun dan dites. Semua perubahan lewat PR, squash merge.

## Penamaan branch
`feat/<id>-<slug>` · `fix/<slug>` · `docs/<slug>` · `chore/<slug>`. Contoh: `feat/3.1-wasapi-loopback`.

## Perintah harian
```bash
git switch main && git pull --ff-only
git switch -c feat/2.1-wails-shell
# ... kerja, commit kecil ...
git fetch origin && git rebase origin/main      # sebelum minta review
git push -u origin feat/2.1-wails-shell
gh pr create --draft --fill                      # buka Draft PR sejak awal
```
Setelah force push karena rebase, gunakan `git push --force-with-lease` (hanya di branch milikmu sendiri, tidak pernah di `main`).

## Commit
- Conventional Commits. Commit `wip(...)` hanya untuk checkpoint di branch kerja; akan hilang saat squash.
- Pesan squash akhir = pesan commit milestone di Master Plan.

## Checkpoint
Lihat Master Plan Bagian 5. Ringkas: tulis CHECKPOINT STATEMENT di `docs/handover/<track>.md`, `git commit -m "wip(<id>): checkpoint"`, push, perbarui Draft PR.

## Tag dan rilis
- Hanya Owner. Setelah PR milestone di-merge:
  ```bash
  git switch main && git pull --ff-only
  git tag -a v0.2.1 -m "Milestone 2.1: Wails shell & UI dasar"
  git push origin v0.2.1
  ```
- Urutan tag boleh tidak kronologis karena track paralel; setiap tag unik.

## Berkas rawan konflik
`go.work`, `go.mod`, `pnpm-lock.yaml`, `pnpm-workspace.yaml`, `packages/ui`, registrasi modul server, `docs/contracts/openapi/` (dipecah per modul agar jarang bentrok).
- Ubah lewat PR kecil tersendiri dan merge cepat.
- Bila `pnpm-lock.yaml` konflik: `git checkout --theirs pnpm-lock.yaml` lalu `pnpm install` ulang; jangan edit manual.

## Perubahan kontrak
Issue "Perubahan Kontrak" → PR berlabel `contract` → approval kedua orang → merge → baru kode yang bergantung. Catat di `docs/contracts/CHANGELOG.md` dan naikkan versi.

## Hotfix
Branch `fix/<slug>` dari `main`, PR kecil, review cepat. Bypass admin hanya untuk keadaan darurat dan dicatat di deskripsi PR.

## Windows
- `.gitattributes` memaksa LF. Setel `git config core.autocrlf false`.
- Hindari path panjang: `git config core.longpaths true`.
