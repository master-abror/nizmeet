#!/usr/bin/env python
"""Buat ZIP source dari daftar berkas (dipanggil oleh `make zip`).

Masukan  : stdin, path relatif dipisah byte NUL (keluaran `git ls-files -z`).
Argumen  : satu, path berkas ZIP keluaran.
Dikecualikan (Master Plan Bagian 2, DoD no. 6):
  - folder .git, node_modules, build (di kedalaman mana pun)
  - berkas *.exe
  - .env dan .env.* (kecuali .env.example)
  - berkas ZIP keluaran itu sendiri
Berkas yang terdaftar tetapi sudah tidak ada di disk dilewati.
"""
import os
import sys
import zipfile

EXCLUDED_DIRS = {".git", "node_modules", "build"}


def is_excluded(rel_path: str) -> bool:
    parts = rel_path.split("/")
    if any(p in EXCLUDED_DIRS for p in parts[:-1]):
        return True
    name = parts[-1]
    if name.lower().endswith(".exe"):
        return True
    if name == ".env" or (name.startswith(".env.") and name != ".env.example"):
        return True
    return False


def main() -> int:
    if len(sys.argv) != 2:
        print("pemakaian: git ls-files -z ... | python zip_source.py <keluaran.zip>", file=sys.stderr)
        return 2

    out_path = os.path.abspath(sys.argv[1])
    raw = sys.stdin.buffer.read()
    entries = [e.decode("utf-8") for e in raw.split(b"\0") if e]

    added = 0
    skipped = 0
    with zipfile.ZipFile(out_path, "w", zipfile.ZIP_DEFLATED) as zf:
        for rel in entries:
            rel = rel.replace("\\", "/")
            if is_excluded(rel) or os.path.abspath(rel) == out_path or not os.path.isfile(rel):
                skipped += 1
                continue
            zf.write(rel, arcname=rel)
            added += 1

    print(f"ZIP dibuat: {out_path} ({added} berkas, {skipped} dilewati)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
