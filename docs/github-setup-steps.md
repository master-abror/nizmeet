# Fase G — Langkah Setup Git & GitHub (untuk Owner)

Dikerjakan **sebelum** Kontributor diundang. Ganti `OWNER`, `CONTRIBUTOR`, `REPO` dengan nilai asli. Perintah memakai GitHub CLI (`gh`); setiap langkah punya padanan di web UI.

## 0. Cek paket GitHub (penting)
Proteksi branch dan "Require review from Code Owners" pada repo **private** bergantung pada paket akun/organisasi. Verifikasi di halaman Pricing GitHub dan di Settings repo sebelum lanjut. Bila tidak tersedia di paket Free, pilihannya: upgrade (mis. Pro/Team), atau jadikan repo public. **Jangan mengundang Kontributor sebelum proteksi terbukti aktif** (G.1 AC).

## 1. Prasyarat lokal
```bash
git --version
gh --version && gh auth login
git config --global user.name "Nama Anda"
git config --global user.email "email@anda"
git config --global init.defaultBranch main
git config --global core.autocrlf false
```

## 2. G.1 — Buat repo dan proteksi `main`
```bash
mkdir REPO && cd REPO && git init
# salin isi folder "copilot-repo-kit" ke sini, lalu:
# - ganti @OWNER dan @CONTRIBUTOR di .github/CODEOWNERS
# - ganti master-abror/nizmeet bila ada di dokumen
git add -A && git commit -m "chore: initial repository setup"
gh repo create OWNER/REPO --private --source=. --remote=origin --push
```
Pengaturan repo (Settings → General):
- Pull Requests: **Allow squash merging** saja; matikan merge commit dan rebase merge.
- Centang **Automatically delete head branches**.
- Settings → Code security: aktifkan Dependabot alerts; aktifkan secret scanning/push protection bila tersedia di paketmu.

Proteksi branch (pilih salah satu):
```bash
# A. via API (branch protection klasik)
gh api -X PUT repos/OWNER/REPO/branches/main/protection --input scripts/branch-protection.json
```
atau Web UI: Settings → Branches (atau Rules → Rulesets) → `main`: Require a pull request (1 approval, dismiss stale approvals, **Require review from Code Owners**), Require status checks (`repo-hygiene`, branch harus up-to-date), Require conversation resolution, Require linear history, blokir force push dan deletion.

Catatan: `enforce_admins: false` membuat Owner dapat bypass saat darurat dan saat milestone G/0.1 (belum ada reviewer). Setelah Kontributor aktif, bypass hanya untuk keadaan darurat dan dicatat di PR.

Verifikasi AC G.1:
```bash
git switch -c uji/proteksi && echo x > x.txt && git add x.txt && git commit -m "test: uji"
git push origin HEAD:main        # HARUS ditolak (bila admin bypass aktif, uji dengan akun non-admin)
```

## 3. G.2 — Dokumen tata kelola
Berkas sudah ada di kit: `CONTRIBUTING.md`, `.github/`, `docs/git-workflow.md`, `HANDOVER.md`, `docs/handover/`. Lakukan:
1. Pastikan placeholder sudah diganti (`grep -rn "@OWNER\|@CONTRIBUTOR\|master-abror/nizmeet" .`).
2. Buka PR uji (mis. ubah satu baris README) → pastikan template PR muncul dan `repo-hygiene` berjalan hijau.
3. Uji negatif: commit `.env` kosong di branch uji → `repo-hygiene` harus **gagal**; hapus → hijau. Tutup PR uji tanpa merge.
4. Commit/tag: `git tag -a v0.9.2 -m "G.2: governance docs"` (setelah merge).

## 4. G.3 — Label, Milestone Gelombang, Issue, Board
```bash
python3 scripts/create_issues.py --dry-run          # lihat apa yang akan dibuat
python3 scripts/create_issues.py --apply --done G.1,G.2,G.3
```
- Skrip membuat label, GitHub Milestones `W0`–`W7`, dan satu issue per milestone dari Master Plan (aman dijalankan ulang).
- `--done` berisi ID milestone yang sudah selesai agar issue yang tidak terblokir berlabel `ready`.
- Project board: Projects → New project → Board, kolom `Backlog, Ready, In progress, In review, Done`; tambahkan semua issue repo (Workflows → Auto-add).
- Setelah 0.1 selesai: `python3 scripts/create_issues.py --apply --done G.1,G.2,G.3,0.1` memperbarui label `ready`.

## 5. Setelah Milestone 0.1 di-merge dan di-tag
```bash
gh api -X PUT repos/OWNER/REPO/collaborators/CONTRIBUTOR -f permission=push   # role Write, bukan Admin
```
Kirim `docs/onboarding/CONTRIBUTOR_ONBOARDING.md`. Minta Kontributor membuat PR uji kecil untuk memastikan alurnya berjalan (AC G.1: Kontributor dapat membuat branch dan PR).

## Checklist ringkas
- [ ] Paket GitHub mendukung proteksi di repo private (terverifikasi)
- [ ] Squash-only, auto-delete branch, proteksi `main` aktif
- [ ] Placeholder CODEOWNERS terganti
- [ ] `repo-hygiene` lulus dan gagal pada uji negatif
- [ ] Label, Milestone W0–W7, issue, dan board siap
- [ ] 0.1 di-merge dan di-tag `v0.0.1`, baru Kontributor diundang
