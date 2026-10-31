#!/usr/bin/env python3
"""Verifikasi tabel Go paket wasapi terhadap header mingw-w64.

Memeriksa, terhadap header yang diberikan:
  1. indeks slot vtable (komentar "Antarmuka::Metode | berkas:baris" di consts.go),
  2. nilai konstanta bertanda "// NAMA | berkas:baris" di consts.go,
  3. tabel AudclntCodes (AUDCLNT_E_*/AUDCLNT_S_*), termasuk kelengkapannya,
  4. literal GUID di guid.go ("// berkas:baris NAMA").

Pemakaian (Git Bash, dari root repo):
  python tools/check_vtbl_slots.py --headers <folder header> \
      --go desktop/internal/audio/wasapi

Keluar dengan kode 1 bila ada ketidakcocokan. Hanya memakai pustaka standar.
"""
import argparse
import os
import re
import sys

errors = []
checked = {"slot": 0, "const": 0, "hresult": 0, "guid": 0}


def err(msg):
    errors.append(msg)


def read_lines(path):
    with open(path, encoding="utf-8", errors="replace") as fh:
        return fh.read().splitlines()


def to_int(s):
    return int(s, 16) if s.lower().startswith("0x") else int(s)


def vtbl_methods(lines, iface):
    """[(nama_metode, nomor_baris_1based)] urut dari struct IxxxVtbl."""
    start = None
    for i, ln in enumerate(lines):
        if re.match(r"\s*typedef struct %sVtbl\s*\{" % re.escape(iface), ln):
            start = i
            break
    if start is None:
        return None
    out = []
    for j in range(start + 1, len(lines)):
        if re.match(r"\s*\}\s*%sVtbl\s*;" % re.escape(iface), lines[j]):
            break
        m = re.search(r"\(STDMETHODCALLTYPE\s*\*\s*(\w+)\)\s*\(", lines[j])
        if m:
            out.append((m.group(1), j + 1))
    return out


def header_symbols(lines):
    """Peta nama -> [(nilai, baris)] dari #define angka dan anggota enum."""
    syms = {}
    in_enum = False
    counter = 0
    for i, ln in enumerate(lines, 1):
        m = re.match(r"\s*#define\s+(\w+)\s+(0x[0-9A-Fa-f]+|\d+)\b", ln)
        if m:
            syms.setdefault(m.group(1), []).append((to_int(m.group(2)), i))
        if re.search(r"\benum\b[^;]*\{", ln):
            in_enum = True
            counter = 0
            continue
        if in_enum:
            if "}" in ln:
                in_enum = False
                continue
            m = re.match(r"\s*([A-Za-z_]\w*)\s*(?:=\s*(0x[0-9A-Fa-f]+|\d+))?\s*,?\s*(?:/[/*].*)?$", ln)
            if m:
                if m.group(2) is not None:
                    counter = to_int(m.group(2))
                syms.setdefault(m.group(1), []).append((counter, i))
                counter += 1
    return syms


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--headers", required=True, help="folder berisi audioclient.h, mmdeviceapi.h, audiosessiontypes.h, mmreg.h")
    ap.add_argument("--go", required=True, help="folder paket wasapi (berisi consts.go dan guid.go)")
    a = ap.parse_args()

    hdr = {}
    for name in os.listdir(a.headers):
        if name.endswith(".h"):
            hdr[name] = read_lines(os.path.join(a.headers, name))
    for need in ("audioclient.h", "mmdeviceapi.h", "audiosessiontypes.h", "mmreg.h"):
        if need not in hdr:
            err("header tidak ditemukan: %s" % need)
    if errors:
        return finish()

    consts = "\n".join(read_lines(os.path.join(a.go, "consts.go")))
    guids = "\n".join(read_lines(os.path.join(a.go, "guid.go")))

    # 1. slot vtable
    seen_iface = set()
    for m in re.finditer(r"Slot(\w+)\s*=\s*(\d+)\s*//\s*(\w+)::(\w+)\s*\|\s*([\w.]+):(\d+)", consts):
        _, slot, iface, meth, fname, line = m.groups()
        slot, line = int(slot), int(line)
        checked["slot"] += 1
        if fname not in hdr:
            err("slot %s::%s: berkas %s tidak ada" % (iface, meth, fname))
            continue
        methods = vtbl_methods(hdr[fname], iface)
        if methods is None:
            err("slot %s::%s: struct %sVtbl tidak ditemukan di %s" % (iface, meth, iface, fname))
            continue
        if iface not in seen_iface:
            seen_iface.add(iface)
            first3 = [n for n, _ in methods[:3]]
            if first3 != ["QueryInterface", "AddRef", "Release"]:
                err("%s: tiga slot pertama bukan IUnknown: %s" % (iface, first3))
        names = [n for n, _ in methods]
        if meth not in names:
            err("slot %s::%s: metode tidak ada di %sVtbl (%s)" % (iface, meth, iface, names))
            continue
        idx = names.index(meth)
        if idx != slot:
            err("slot %s::%s: Go=%d header=%d" % (iface, meth, slot, idx))
        hline = methods[idx][1]
        if hline != line:
            err("slot %s::%s: baris Go=%d header=%d" % (iface, meth, line, hline))

    # 2. konstanta bernama
    all_syms = {}
    for fname, lines in hdr.items():
        for k, v in header_symbols(lines).items():
            for val, ln in v:
                all_syms.setdefault((fname, k), []).append((val, ln))
    for m in re.finditer(r"=\s*(0x[0-9A-Fa-f]+|\d+)\s*//\s*(\w+)\s*\|\s*([\w.]+):(\d+)", consts):
        val, name, fname, line = to_int(m.group(1)), m.group(2), m.group(3), int(m.group(4))
        checked["const"] += 1
        cands = all_syms.get((fname, name))
        if not cands:
            err("konstanta %s: tidak ditemukan di %s" % (name, fname))
            continue
        if (val, line) not in cands:
            err("konstanta %s: Go=(%#x, baris %d) header=%s" % (name, val, line, cands))

    # 3. tabel HRESULT
    go_codes = {}
    for m in re.finditer(r'Name:\s*"(\w+)",\s*N:\s*(0x[0-9A-Fa-f]+),\s*Failure:\s*(true|false),\s*Line:\s*(\d+)', consts):
        go_codes[m.group(1)] = (to_int(m.group(2)), m.group(3) == "true", int(m.group(4)))
    hdr_codes = {}
    for i, ln in enumerate(hdr["audioclient.h"], 1):
        m = re.match(r"\s*#define\s+(AUDCLNT_[ES]_\w+)\s+AUDCLNT_(ERR|SUCCESS)\((0x[0-9A-Fa-f]+|\d+)\)", ln)
        if m:
            hdr_codes[m.group(1)] = (to_int(m.group(3)), m.group(2) == "ERR", i)
    for name, val in go_codes.items():
        checked["hresult"] += 1
        if name not in hdr_codes:
            err("hresult %s: tidak ada di audioclient.h" % name)
        elif hdr_codes[name] != val:
            err("hresult %s: Go=%s header=%s" % (name, val, hdr_codes[name]))
    for name in sorted(set(hdr_codes) - set(go_codes)):
        err("hresult %s ada di header tetapi tidak di AudclntCodes" % name)

    # 4. GUID
    hdr_guids = {}
    for fname in ("audioclient.h", "mmdeviceapi.h"):
        for i, ln in enumerate(hdr[fname], 1):
            m = re.match(r"\s*DEFINE_GUID\(\s*(\w+)\s*,(.*)\)\s*;", ln)
            if m:
                nums = [to_int(x) for x in re.findall(r"0x[0-9A-Fa-f]+|\b\d+\b", m.group(2))]
                hdr_guids[(fname, m.group(1))] = (nums, i)
    for m in re.finditer(
            r"GUID\{(0x[0-9a-f]+),\s*(0x[0-9a-f]+),\s*(0x[0-9a-f]+),\s*\[8\]byte\{([^}]*)\}\}\s*//\s*([\w.]+):(\d+)\s+(\w+)",
            guids):
        d1, d2, d3 = to_int(m.group(1)), to_int(m.group(2)), to_int(m.group(3))
        d4 = [to_int(x.strip()) for x in m.group(4).split(",") if x.strip()]
        fname, line, name = m.group(5), int(m.group(6)), m.group(7)
        checked["guid"] += 1
        got = hdr_guids.get((fname, name))
        if got is None:
            err("guid %s: DEFINE_GUID tidak ditemukan di %s" % (name, fname))
            continue
        if got[0] != [d1, d2, d3] + d4:
            err("guid %s: Go=%s header=%s" % (name, [d1, d2, d3] + d4, got[0]))
        if got[1] != line:
            err("guid %s: baris Go=%d header=%d" % (name, line, got[1]))
    return finish()


def finish():
    print("diperiksa: %s" % ", ".join("%s=%d" % kv for kv in checked.items()))
    if errors:
        print("TIDAK COCOK (%d):" % len(errors))
        for e in errors:
            print("  - " + e)
        return 1
    print("OK: semua tabel cocok dengan header")
    return 0


if __name__ == "__main__":
    sys.exit(main())
