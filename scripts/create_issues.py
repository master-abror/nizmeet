#!/usr/bin/env python3
"""Membuat label, GitHub Milestones (W0-W7), dan satu issue per milestone dari Master Plan.

Sumber kebenaran: docs/MASTER_PLAN_V2.2.md (tabel TRACK-MAP + blok milestone).
Aman dijalankan ulang: issue dengan judul yang sama dilewati.

Pemakaian:
  python3 scripts/create_issues.py --dry-run
  python3 scripts/create_issues.py --apply --done G.1,G.2,G.3
Opsi: --owner-login USER --contrib-login USER  (otomatis menugaskan issue sesuai PIC)
Butuh: GitHub CLI (`gh`) yang sudah login, dijalankan dari dalam clone repo.
"""
import argparse, json, re, subprocess, sys
from pathlib import Path

PLAN = Path(__file__).resolve().parent.parent / "docs" / "MASTER_PLAN_V2.2.md"
TRACK_LABEL = {"S": "track:server", "D": "track:desktop", "W": "track:web", "X": "track:lintas"}
TRACK_NAME = {"S": "Server", "D": "Desktop", "W": "Web Portal", "X": "Lintas/tata kelola"}
WAVE_DESC = {
    "W0": "Owner solo: Git/GitHub dan scaffolding", "W1": "Paralel penuh: kontrak + desktop dasar",
    "W2": "Server inti + mock + transport", "W3": "Sync, sesi live, UI transkrip",
    "W4": "Smart RAG + jawaban live", "W5": "Evaluasi, portal beta, overlay",
    "W6": "Portal lanjutan, installer", "W7": "Hardening dan rilis v1.0.0",
}
LABELS = [
    ("track:server", "1d76db", "Track S: server"), ("track:desktop", "0e8a16", "Track D: desktop"),
    ("track:web", "fbca04", "Track W: web portal"), ("track:lintas", "5319e7", "Track X: lintas/tata kelola"),
    ("ready", "2ea44f", "Dependensi terpenuhi, siap dikerjakan"), ("blocked", "d73a4a", "Menunggu dependensi"),
    ("contract", "b60205", "Perubahan docs/contracts"), ("gate", "000000", "Phase Gate (review sebelum lanjut)"),
]
HEAD = re.compile(r"^\*\*\\\[ \\\] (\S+) (.+?)\*\*\s*$")

def parse_map(text):
    block = text.split("<!-- TRACK-MAP:START -->")[1].split("<!-- TRACK-MAP:END -->")[0]
    rows = {}
    for ln in block.splitlines():
        cells = [c.strip() for c in ln.strip().strip("|").split("|")]
        if len(cells) == 6 and cells[0] not in ("ID", "---") and not set(cells[0]) <= {"-"}:
            mid, title, track, pic, wave, deps = cells
            rows[mid] = dict(title=title, track=track, pic=pic, wave=wave,
                             deps=[] if deps in ("–", "-", "") else [d.strip() for d in deps.split(",")])
    return rows

def parse_milestones(text):
    ms, cur = {}, None
    for ln in text.splitlines():
        m = HEAD.match(ln)
        if m:
            cur = m.group(1); ms[cur] = dict(title=m.group(2), tugas="", ac="", commit="", tag=""); continue
        if ln.startswith("#"):
            cur = None
        if cur:
            for key, pref in (("tugas", "- Tugas:"), ("ac", "- AC:"), ("commit", "- Commit:"), ("tag", "- Tag:")):
                if ln.startswith(pref):
                    ms[cur][key] = ln[len(pref):].strip()
    return ms

def topo(rows):
    order, seen = [], set()
    def visit(i):
        if i in seen: return
        seen.add(i)
        for d in rows[i]["deps"]: visit(d)
        order.append(i)
    for i in rows: visit(i)
    return order

def gh(args, inp=None, check=True):
    r = subprocess.run(["gh"] + args, input=inp, capture_output=True, text=True)
    if check and r.returncode != 0:
        sys.exit(f"gh {' '.join(args[:3])} gagal: {r.stderr.strip()}")
    return r

def build_body(mid, row, ms, num):
    ac = [a.strip().rstrip(".") for a in ms["ac"].split(";") if a.strip()]
    deps = ", ".join(f"#{num[d]} ({d})" if d in num else d for d in row["deps"]) or "tidak ada"
    lines = [
        f"**Track:** {row['track']} ({TRACK_NAME[row['track']]}) · **PIC:** {row['pic']} · **Gelombang:** {row['wave']}",
        f"**Bergantung pada:** {deps}",
        f"**Sumber:** `docs/MASTER_PLAN_V2.2.md` — milestone {mid}", "",
        "## Tugas", ms["tugas"], "", "## Acceptance Criteria",
        *[f"- [ ] {a}" for a in ac], "",
        "## Definition of Done",
        "- [ ] `make test` hijau (CI hijau setelah 0.2)",
        "- [ ] `docs/handover/<track>.md` diperbarui di PR yang sama",
        "- [ ] PR disetujui dan di-squash merge", "- [ ] Owner membuat tag", "",
        f"**Commit:** {ms['commit']}  \n**Tag:** {ms['tag']}",
    ]
    return "\n".join(lines)

def main():
    ap = argparse.ArgumentParser()
    g = ap.add_mutually_exclusive_group(required=True)
    g.add_argument("--dry-run", action="store_true"); g.add_argument("--apply", action="store_true")
    ap.add_argument("--done", default="", help="ID milestone yang sudah selesai, dipisah koma")
    ap.add_argument("--owner-login", default=""); ap.add_argument("--contrib-login", default="")
    a = ap.parse_args()
    text = PLAN.read_text(encoding="utf-8")
    rows, ms = parse_map(text), parse_milestones(text)
    missing = sorted(set(rows) ^ set(ms))
    if missing: sys.exit(f"Tabel TRACK-MAP dan blok milestone tidak cocok untuk ID: {missing}")
    for i, r in rows.items():
        for d in r["deps"]:
            if d not in rows: sys.exit(f"Dependensi tidak dikenal: {i} -> {d}")
    done = {x.strip() for x in a.done.split(",") if x.strip()}
    assignee = {"Owner": a.owner_login, "Kontributor": a.contrib_login}

    existing = {}
    if a.apply:
        for n, c, d in LABELS:
            gh(["label", "create", n, "--color", c, "--description", d, "--force"])
        for w, d in WAVE_DESC.items():
            gh(["api", "repos/{owner}/{repo}/milestones", "-f", f"title={w}", "-f", f"description={d}"], check=False)
        lst = json.loads(gh(["issue", "list", "--state", "all", "--limit", "500", "--json", "number,title"]).stdout)
        existing = {i["title"]: i["number"] for i in lst}

    num, made, skipped = {}, 0, 0
    for mid in topo(rows):
        row, m = rows[mid], ms[mid]
        title = f"[{mid}] {m['title']}"
        if title in existing:
            num[mid] = existing[title]; skipped += 1; continue
        is_done = mid in done
        ready = all(d in done for d in row["deps"]) and not is_done
        labels = [TRACK_LABEL[row["track"]]] + ([] if is_done else ["ready" if ready else "blocked"])
        if "🚩" in m["tag"]: labels.append("gate")
        body = build_body(mid, row, m, num)
        if a.dry_run:
            num[mid] = len(num) + 1
            print(f"--- #{num[mid]} {title}\n    labels={labels} milestone={row['wave']} deps={row['deps']}")
            continue
        cmd = ["issue", "create", "--title", title, "--body-file", "-", "--milestone", row["wave"]]
        for l in labels: cmd += ["--label", l]
        if assignee.get(row["pic"]): cmd += ["--assignee", assignee[row["pic"]]]
        url = gh(cmd, inp=body).stdout.strip()
        num[mid] = int(url.rsplit("/", 1)[1]); made += 1
        if is_done:
            gh(["issue", "close", str(num[mid]), "--comment", "Sudah selesai sebelum issue dibuat."])
        print(f"dibuat #{num[mid]} {title}")
    print(f"\nTotal milestone: {len(rows)} | dibuat: {made} | dilewati (sudah ada): {skipped}" if a.apply
          else f"\nDRY-RUN OK: {len(rows)} issue akan dibuat. Contoh body ada di bawah.\n\n" + build_body("0.1", rows["0.1"], ms["0.1"], {"G.3": 3}))

if __name__ == "__main__":
    main()
