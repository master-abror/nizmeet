# ADR-0003: Kontrak Ekstensibilitas dan Kerangka Modul

- **Status:** Diusulkan (menjadi Diterima setelah review PR ini; gate Milestone 0.4)
- **Tanggal:** 2026-10-28
- **Konteks:** Master Plan V2.2, Bagian 1 ("Kontrak Ekstensibilitas") dan Milestone 0.4.

## Konteks
Career Studio dan fitur lain direncanakan setelah MVP. Fitur baru harus masuk sebagai modul baru, tanpa mengubah kode inti, skema inti, atau kontrak yang sudah dirilis. ADR ini menetapkan aturan itu sebelum Fase 1 menulis kode yang bergantung padanya.

**Aturan emas:** bila fitur baru memaksa perubahan pada kode inti atau kontrak yang sudah rilis, itu dianggap cacat desain dan dicatat di ADR.

## Keputusan

### Server
1. **Modular monolith.** Setiap modul ada di `server/internal/<modul>` dan mengimplementasikan interface `Module` dengan empat tanggung jawab: route, migrasi, job, dan entitlement. Modul didaftarkan di satu registry. Menambah modul = folder baru + satu baris registrasi.
2. **Paket kerangka mandiri.** Paket `Module` dan registry tidak mengimpor `net/http` maupun router. Kait route dinyatakan lewat tipe abstrak milik paket itu dan diadaptasi ke `chi` di Milestone 1.1.
3. **Empat registry dibangun di 0.4 sebagai mekanisme:** tipe dokumen, resource kind sync, `LLMTask`, dan entitlement. Isi bawaannya didaftarkan oleh milestone yang memilikinya.

### Database
4. **Namespace = skema PostgreSQL per modul.** Tabel inti ada di `public`; tabel modul ada di skema bernama modul (seperti `hello` di `docs/contracts/db/ddl.sql`).
5. **Migrasi per modul**, dengan pelacakan versi sendiri per modul, sehingga berjalan, di-rollback, dan dihapus terpisah dari inti dan modul lain. Hanya aditif (expand/contract). Kolom spesifik tipe disimpan di `payload` jsonb. Setiap migrasi modul punya langkah down. Tabel versi migrasi modul berada di skema modul itu sendiri. Karena itu skema modul dibuat dan dihapus oleh runner migrasi, bukan oleh berkas migrasi modul: skema tidak boleh dihapus dari dalam migrasi down yang masih perlu mencatat versinya.
6. **Menghapus modul** = down migrasinya (skema modul ikut hilang), hapus folder, hapus satu baris registrasi. Tidak boleh ada sisa di inti.

### Data dan sinkronisasi
7. **Tipe dokumen** adalah string dari registry, bukan enum DB. `schema_version` ada per dokumen. Tipe baru = entri registry + handler parser.
8. **Sync berbasis resource kind.** Cursor per kind; envelope memuat `kind` dan `schema_version`; klien mengabaikan kind yang tidak dikenal. Master Plan menyebut kind MVP `documents` dan `session_profiles`; ERD draf C.1 menambah `live_sessions` (keputusan Owner). Daftar final ada di `docs/contracts/sync-spec.md` (C.4).
9. **Entitlement:** token lisensi memuat `features[]`. Kode memeriksa fitur, bukan nama paket. Fitur baru = kunci fitur + pemetaan paket.
10. **Usage metering generik:** `usage_events(kind, units, meta jsonb)`. Kind baru tanpa perubahan skema.

### AI dan pipeline
11. **Registry `LLMTask`** (`fast_pass`, `translate`, `main_pass`, dan tugas lain nanti) dengan prompt ber-versi di `prompts/<task>/vN`. Provider, model, dan timeout per tugas (dan override per bahasa) berasal dari `.env` server.
12. **Persona, gaya, dan bahasa berupa data** (registry preset + style pack prompt ber-versi per bahasa), bukan kode.
13. **Tahap pipeline server** (`Transcriber`, `Retriever`, `LLM`, `Stage`) berada di balik interface dan terdaftar di registry. Registry ini dibangun oleh milestone pemiliknya (Fase 3), bukan di 0.4.

### API, protokol, dan portal
14. **API:** prefiks `/v1`; perubahan hanya aditif; field tak dikenal diabaikan; `/v1/client-config` memuat versi minimum klien, daftar fitur, dan daftar bahasa.
15. **WebSocket:** pesan ber-versi; tipe pesan tak dikenal diabaikan; nama event ber-namespace (`answer:*`, `sync:*`, `session:*`).
16. **Web Portal:** route group per modul; navigasi dibentuk dari registry entitlement.

## Cakupan Milestone 0.4
- **Dibangun:** butir 1 sampai 7 dan 9 (kerangka `Module`, empat registry, konvensi migrasi, modul `hello` di server) serta butir 16 (struktur route group dan `hello` di portal).
- **Hanya aturan, belum kode:** butir 8, 10 sampai 15. Dibangun oleh milestone yang memilikinya dan wajib mengikuti ADR ini.

## Bukti (AC 0.4)
- Modul `hello` ditambahkan hanya dengan folder baru + satu baris registrasi, tanpa mengubah file inti lain (dibuktikan lewat diff PR).
- Migrasi `hello` berjalan terpisah dan dapat di-rollback.
- Modul `hello` dihapus tanpa meninggalkan sisa.
- ADR ini direview sebelum tag `v0.0.4`.

## Konsekuensi
- Milestone 1.1 dan seterusnya wajib memakai registry ini.
- Kontrak yang sudah dirilis hanya berubah secara aditif.
- Setiap fitur baru yang memaksa perubahan pada kode inti dicatat sebagai ADR baru.

## Hal yang belum diputuskan (OPEN QUESTION)
1. **Alat migrasi.** Master Plan Milestone 1.2 menyebut "golang-migrate/goose" tanpa memilih. AC 0.4 mewajibkan migrasi `hello` dapat dijalankan dan di-rollback, jadi alat harus dipilih sebelum PR kerangka server. Keputusan Owner; Milestone 1.2 mengikuti.
2. **Penyelarasan kontrak DB.** Skema `hello` di `docs/contracts/db/ddl.sql` dan Q7 di `docs/contracts/erd.md` bagian 8 harus selaras dengan ADR ini. Perubahannya lewat PR berlabel `contract` yang disetujui kedua orang.
