# Master Plan: Real-Time Meeting & Interview Co-Pilot

Dokumentasi Spesifikasi Teknis & Master Checklist (V2.2 — Oktober 2026). V2.2 menggantikan V2.1 dan menambahkan: Fase G (Git/GitHub), Fase C (Kontrak & Dokumentasi Teknis), pembagian track untuk tim 2 orang, serta mekanisme checkpoint/handover per track. V2.1 sebelumnya menggantikan V1 (desktop tipis, seluruh pemrosesan AI di server).

## 0. Saran Arsitektur (Mohon Konfirmasi Sebelum Milestone 0.1)

Produk ini bukan satu aplikasi desktop, melainkan platform dua permukaan (Web Portal + Desktop Co-Pilot) di atas satu Cloud Backend. V2 memuat klarifikasi itu. Berikut keputusan arsitektur dan saran saya; butir 1 sampai 3 paling menentukan dan perlu konfirmasi Anda sebelum Milestone 0.1.

1. **Cakupan rilis.** Career Studio (AI CV Generator, Portfolio Builder, Interview Simulator) masing-masing setara satu produk. Saran: **v1.0 = Co-Pilot desktop + Web Portal inti** (akun, billing, perangkat, Library/CV editor dasar, unduhan, sync). Career Studio masuk **Fase 8** secara berurutan: CV Generator, Interview Simulator, Portfolio Builder. Skema data dan API dirancang untuk ketiganya sejak awal agar tidak ada rombak ulang.
2. **Cloud = sumber kebenaran; SQLite = cache baca lokal (Library, riwayat sesi).** Dokumen yang diunggah dari desktop juga dikirim ke cloud. Aliran selalu desktop/portal → cloud → desktop; desktop tidak pernah menjadi master data.
3. **Sync berbasis revisi, bukan timestamp.** Tiap dokumen punya `revision` yang naik monoton, tiap device punya cursor, penghapusan memakai tombstone. Konflik v1.0: server menang per dokumen, versi lama tetap tersimpan dan dapat dipulihkan. Unggahan dari desktop saat offline masuk antrian (outbox) dan dikirim idempoten.
4. **File asli di object storage, teks di PostgreSQL.** PDF/DOCX asli disimpan di storage S3-compatible (mis. Cloudflare R2 atau MinIO). Teks hasil parse, chunk, tag, dan versi disimpan di PostgreSQL. Chunking dilakukan **server** agar indeks konsisten di semua device.
5. **Privasi data karier.** CV memuat data pribadi sensitif. Wajib: enkripsi at-rest di cloud, semua query ber-scope `user_id` (dengan test isolasi), hapus akun = hapus seluruh data, ekspor data pribadi, dan kebijakan privasi yang selaras dengan UU PDP. Catatan jujur: SQLCipher butuh CGO sehingga tidak cocok dengan `modernc.org/sqlite`; cache lokal disimpan di folder pengguna dengan ACL ketat, enkripsi penuh bergantung pada BitLocker, dan tersedia opsi hapus cache saat logout. Keputusan ini dicatat di ADR.
6. **Fast-Pass punya batas waktu.** Intent Router + Query Rewriter menambah satu panggilan LLM sebelum jawaban. Saran: pre-filter heuristik, model kecil-cepat, timeout 700 ms, dijalankan paralel dengan retrieval, dan fallback ke retrieval dari transkrip mentah. Target tetap: pertanyaan selesai diucapkan → token pertama ≤ 2,5 detik (P50).
7. **JSON terstruktur harus bisa di-stream.** JSON baru valid di akhir, sehingga UI akan menunggu penuh. Saran: schema berurutan (terjemahan dan intent lebih dulu, lalu jawaban) dengan **partial JSON parser** di server agar tiap field tampil begitu selesai, plus fallback ke teks mentah bila parse gagal.
8. **Model LLM.** Claude 3.5 Haiku sudah usang; default Cloud: **Claude Haiku 5.5** (`claude-haiku-5-5`). Provider, model, endpoint, dan key semuanya dibaca dari .env server. Verifikasi daftar model terbaru (Anthropic/OpenAI/Groq) di Milestone 4.3.
9. **Wails v2 (diputuskan 22 Okt 2026).** Hasil pengecekan: Wails v3 masih berstatus beta (`v3.0.0-beta.x`) dan catatan rilisnya menyatakan v2 tetap rilis stabil, sehingga MVP memakai **Wails v2**. Semua kode Wails diisolasi (hanya `main.go` dan satu package bridge yang boleh mengimpor Wails) agar migrasi ke v3 mudah; ditinjau ulang setelah v3 stabil (lihat Bagian 4).
10. **Pin versi.** Go 1.27.x (minimal 1.27.1; Go 1.27.0 dirilis 19 Agustus 2026), Svelte/SvelteKit, Tailwind, PostgreSQL 18, Redis dipin eksak di `go.mod`, `package.json`, dan `docker-compose.yml`.
11. **API key tidak ada di klien.** Mode SaaS lewat LLM/STT Proxy di server (metering + anti-abuse). Tidak ada mode BYOK dan tidak ada endpoint model di desktop: provider, model, endpoint, dan key semuanya diatur lewat .env server.
12. **Payment gateway.** Midtrans/Xendit (QRIS, VA, e-wallet) + Stripe atau Lemon Squeezy, di balik interface `PaymentProvider`. Checkout berlangsung di Web Portal.
13. **Monorepo dengan UI bersama.** `/web` (SvelteKit), `/desktop` (Wails + Svelte), `/packages/ui` (komponen + token Tailwind), `/server`. Logika bisnis tidak dibagi: desktop bicara ke Go engine lewat Wails bindings, portal bicara ke REST API.
14. **Portfolio Builder butuh keamanan khusus.** Situs publik buatan pengguna wajib di domain terpisah dari portal, dengan sanitasi konten dan batas ukuran. Karena itu ia ditempatkan terakhir di Fase 8.
15. **Audio dua arah dan Windows-only.** Tambahkan capture mikrofon (opsional) agar transkrip dua arah. v1.0 hanya Windows.

**Pembagian beban (arahan Anda):** desktop dibuat setipis mungkin karena spesifikasi laptop pengguna beragam. Desktop hanya menangkap audio, mengirimnya lewat satu koneksi WebSocket, dan menampilkan hasil. STT, Fast-Pass, retrieval, dan LLM seluruhnya berjalan di server sebagai pipeline paralel (goroutine + worker pool). Konsekuensinya: tidak ada BYOK, tidak ada endpoint model di desktop, Local Whisper dikeluarkan dari MVP, dan retrieval untuk sesi live dilakukan di server. Rinciannya ada di subbagian **Arsitektur Latensi & Beban**.

**Kebutuhan multibahasa & persona (arahan Anda):** aplikasi harus menangani banyak bahasa. Lawan bicara berbahasa Inggris menghasilkan transkrip Inggris, terjemahan Indonesia di bawahnya, saran jawaban Inggris, dan terjemahan Indonesia di bawah tiap saran. Jawaban mengacu pada CV/dokumen dan tag fokus pengguna, mengikuti peran (mis. Software Engineer atau Graphic Designer) dan gaya bahasa (formal, informal, santai, slang). Seluruhnya diatur lewat **Session Profile** yang dapat disimpan sebagai preset. Penerjemahan dikerjakan oleh LLM, bukan oleh STT. Rinciannya ada di subbagian **Persona, Bahasa & Grounding**, dan pilihan model final ditetapkan lewat evaluasi di Milestone 4.7.

**Status keputusan:** cakupan rilis (MVP = Co-Pilot desktop + Web Portal inti; Career Studio direncanakan terpisah), cloud sebagai sumber kebenaran, dan aturan konflik sync server-menang sudah **disetujui**. Butir lain berlaku sebagai default, termasuk catatan enkripsi cache lokal (butir 5), dan dapat diubah kapan saja. Syarat tambahan dari Anda: MVP harus siap menerima modul baru tanpa mengubah inti; lihat **Kontrak Ekstensibilitas** di Bagian 1 dan Milestone 0.4.

**Keputusan tambahan (V2.2):** tim terdiri dari 2 orang (pemilik repo + 1 kontributor), tanpa kontributor ketiga pada tahap awal; Wails v2; kerja paralel berbasis track dan kontrak (Fase G dan Fase C); checkpoint dan handover per track (Bagian 2 dan 5). Opsi menambah orang untuk Track W (Web Portal) ditinjau di akhir Gelombang 1.

## 1. Arsitektur Ringkas

```
Web Portal (SvelteKit)               Desktop Co-Pilot (Wails) = klien tipis
 Career Studio, akun, billing         Capture audio (WASAPI) + gate VAD ringan
          |                           Svelte UI + SQLite (Library, riwayat)
          | REST + JWT                      |  audio         ^  hasil per field
          |                                 v  WebSocket     |
  +-----------------------------------------------------------------+
  |  Cloud Server (Go + chi)                                        |
  |  Live Session Service:                                          |
  |    STT -> [Fast-Pass || Retrieval] -> Main-Pass LLM -> stream   |
  |  Auth, licensing, payment, documents + parser, sync,            |
  |  provider gateway (LLM/STT, dari .env), metering, admin         |
  +-----------------------------------------------------------------+
        |                 |                  |
   PostgreSQL 18        Redis          Object storage
   (data master)   (rate limit, sesi)  (file asli)
```

**Struktur monorepo:**

```
/web            (SvelteKit: portal, Career Studio, admin)
/desktop        (Wails: /app, /engine, /frontend)
/packages/ui    (komponen Svelte + token Tailwind bersama)
/server         (chi: /cmd, /internal, /migrations)
/deploy         (docker-compose, Dockerfile, CI)
/docs           (ADR, API spec, runbook)
HANDOVER.md
```

**Prinsip desain:**

- Satu Cloud API dipakai dua klien (portal dan desktop); kontrak didokumentasikan di OpenAPI.
- **Kontrak dulu, kode kemudian:** hal yang dipakai lintas track (skema DB, OpenAPI, protokol WebSocket, spesifikasi sync, kontrak prompt/JSON) ditulis di `docs/contracts/` pada Fase C. Setelah kontrak v0 dibekukan (C.8), perubahan hanya lewat PR berlabel `contract`. Milestone implementasi (mis. 1.2, 1B.1, 1B.3, 3.3, 3.4) **mengimplementasikan** kontrak, bukan merancang ulang.
- Cloud adalah sumber kebenaran data pengguna; SQLite desktop hanya cache baca (Library, riwayat sesi) yang bisa dibangun ulang dari cloud.
- **Desktop tipis:** tidak ada model, endpoint model, atau API key provider di desktop; semua pemrosesan AI di server. Desktop punya anggaran sumber daya yang diuji.
- Semua provider (STT, LLM, Payment, Retriever) di balik **interface Go** dan dikonfigurasi lewat `.env` server, bukan di kode atau klien.
- Streaming di mana pun: provider → server → WebSocket → UI.
- Desktop tanpa CGO (`modernc.org/sqlite`); `context.Context` di semua I/O; logging terstruktur `log/slog`.
- Setiap query data pengguna di server wajib ber-scope `user_id`.

### Peran Tiap Surface

| Surface | Teknologi | Pengguna | Fungsi utama | Data |
| --- | --- | --- | --- | --- |
| Web Portal | SvelteKit + Tailwind | Pelanggan, admin | Akun, billing, perangkat, unduhan, Library/CV editor dasar, admin (Career Studio menyusul sebagai modul) | Tidak menyimpan data sendiri; semua lewat API |
| Desktop Co-Pilot | Wails + Svelte SPA + Go (ringan) | Pengguna saat meeting atau interview | Capture audio, gate VAD ringan, streaming audio ke server, menampilkan transkrip dan jawaban, overlay, Library | SQLite: cache baca Library, riwayat sesi, settings. Tanpa model, tanpa key provider |
| Cloud Server | Go + chi, PostgreSQL 18, Redis, object storage | Portal dan desktop | Auth, lisensi, payment, data master, parser, sync, Live Session Service (STT, intent, retrieval, LLM), provider gateway, metering | Sumber kebenaran |

### Model Data & Alur Dokumen (Cloud ke Lokal)

Cloud menyimpan data master; desktop hanya menyimpan salinan baca untuk tampilan Library dan akses offline. Alurnya:

1. CV/dokumen dibuat atau diedit di Web Portal, atau diunggah dari Library desktop (dikirim ke cloud lewat API, bukan hanya disimpan lokal).
2. Server menyimpan file asli di object storage, mengurai teks, memecah menjadi chunk, memberi tag (skill, peran, perusahaan, periode), lalu menaikkan `revision`.
3. Desktop menarik perubahan saat aplikasi dibuka, saat tombol Sync Now ditekan, dan berkala saat online, memakai cursor per device.
4. Desktop menerapkan upsert dan tombstone ke SQLite dalam satu transaksi, untuk tampilan Library dan pencarian lokal yang ringan.
5. Saat sesi live dimulai, server memuat chunk dokumen aktif milik pengguna ke cache sesi. Retrieval selama sesi dijalankan di server; desktop tidak ikut dalam retrieval.

Tabel cloud utama (dirinci di Fase 1B): `profiles`, `documents`, `document_versions`, `document_chunks`, `tombstones`, `sync_cursors`. Konflik pada v1.0: server menang per dokumen; versi lama tetap tersimpan dan dapat dipulihkan.

### Pipeline Smart RAG (Agentic RAG Lite)

```
DESKTOP (ringan)                  SERVER (paralel, satu pipeline per sesi)
WASAPI -> resample 16 kHz mono    WebSocket ingest
 -> gate VAD (energi)  ---------> VAD/endpointing -> STT worker pool (hint bahasa)
                                    -> transkrip final (bahasa asli) -> UI
                                    -> paralel:  Fast-Pass: terjemahan + intent + rewrite (700 ms)
                                                 Retrieval (transkrip mentah + tag fokus)
                                    -> retrieval ulang dengan rewrite (bahasa dokumen)
                                       bila tiba tepat waktu (gabung RRF)
                                    -> Main-Pass (peran, gaya, tag, bahasa jawaban, grounding)
                                    -> tiap jawaban selesai -> task translate (paralel)
Svelte merender field  <--------- partial JSON parser -> event per field
```

Dua keluaran terstruktur. **Fast-Pass** (satu panggilan, field `translation` di-stream lebih dulu; `rewritten_queries` ditulis dalam bahasa dokumen aktif, mis. "memimpin tim" menjadi "mengelola", "lead", "engineer", "team"):

```json
{
  "translation": "...",
  "detected_lang": "en",
  "is_question": true,
  "intent": {"type": "behavioral | technical | clarification | other", "topic": "...", "confidence": 0.0},
  "rewritten_queries": ["..."],
  "tags": ["postgresql"]
}
```

**Main-Pass** (jawaban dalam bahasa jawaban; terjemahannya datang terpisah dari task `translate` per jawaban sebagai event `answer:translation`):

```json
{
  "suggested_answers": [
    {"length": "singkat", "text": "...", "grounded": true, "sources": ["chunk_id"]},
    {"length": "detail", "text": "...", "grounded": false, "sources": []}
  ]
}
```

Pencarian berada di balik interface `Retriever`. Implementasi MVP berjalan di server: PostgreSQL full-text search (GIN index; konfigurasi bahasa Indonesia dan Inggris diverifikasi di Milestone 4.2) dengan chunk dokumen aktif dipanaskan saat sesi dimulai. `HybridRetriever` (FTS + embedding) dapat ditambahkan kemudian tanpa mengubah pipeline, cukup dengan migrasi tabel `chunk_embeddings`.

### Persona, Bahasa & Grounding (Sesi Multibahasa)

Setiap sesi berjalan dengan **Session Profile** pilihan pengguna. Profil disimpan di cloud dan tersinkron seperti dokumen, dan dapat disimpan sebagai preset bernama (mis. "Interview Backend EN"). Pengaturan dapat diubah di tengah sesi dan berlaku mulai ucapan berikutnya.

| Pengaturan | Nilai | Dipakai oleh |
| --- | --- | --- |
| Peran | Preset (Software Engineer, Backend Engineer, Graphic Designer, Product Manager, Data Analyst, dst.) atau teks bebas | Main-Pass: sudut pandang, istilah, contoh |
| Gaya bahasa | `formal`, `informal`, `santai`, `slang` (preset bawaan, aturannya per bahasa) | Main-Pass |
| Panjang jawaban | singkat atau detail | Main-Pass |
| Bahasa lawan bicara | `auto` (deteksi per ucapan) atau satu bahasa tetap | STT (hint), Fast-Pass |
| Bahasa jawaban | Ikuti lawan bicara (default) atau satu bahasa tetap | Main-Pass |
| Bahasa terjemahan | Bahasa pengguna, mis. Indonesia | Task `translate` |
| Tag fokus | mis. postgresql, redis, golang; dari tag dokumen atau buatan sendiri, dinormalisasi dengan alias (go = golang) | Retrieval (boost/filter) dan Main-Pass |
| Dokumen aktif | CV, JD, catatan | Retrieval |
| Mode | Interview atau Meeting | Main-Pass |

**Alur bahasa.** Contoh: lawan bicara berbahasa Inggris, pengguna berbahasa Indonesia. (1) Ucapan ditranskripsi dalam bahasa aslinya. (2) Terjemahan Indonesia tampil tepat di bawahnya. (3) Saran jawaban ditulis dalam bahasa Inggris. (4) Tiap saran punya terjemahan Indonesia di bawahnya agar pengguna paham yang akan ia ucapkan. Bila bahasa ucapan sama dengan bahasa terjemahan, terjemahan dilewati.

**Aturan grounding.** Jawaban mengacu pada dokumen pengguna dan tag fokus:

- Pengalaman pribadi (perusahaan, proyek, angka, tahun) hanya boleh berasal dari chunk dokumen yang diambil; tidak boleh dikarang.
- Pengetahuan teknis umum untuk tag fokus boleh dipakai, bergaya sesuai peran, tetapi ditandai `grounded: false` dan tampil dengan badge "umum" di UI.
- Setiap saran membawa `sources[]` (id chunk) agar UI dapat menunjukkan asal jawaban.

**Retrieval lintas bahasa.** CV bisa berbahasa Indonesia sementara pertanyaan berbahasa Inggris, padahal FTS bersifat leksikal. Karena itu bahasa tiap dokumen dideteksi saat parsing, Fast-Pass menulis `rewritten_queries` dalam bahasa dokumen aktif, dan tag dinormalisasi lintas bahasa. Bila evaluasi menunjukkan recall lintas bahasa kurang, `HybridRetriever` dengan embedding multibahasa dinaikkan prioritasnya dari backlog.

**Pilihan model (semua dari `.env` server, per tugas, dapat dioverride per bahasa):**

| Tugas | Kebutuhan | Kandidat awal |
| --- | --- | --- |
| STT | Multibahasa, hint bahasa, vocabulary hint dari tag dokumen | Groq `whisper-large-v3-turbo` (varian turbo hanya untuk transkripsi, jadi terjemahan dikerjakan LLM; verifikasi di dokumentasi Groq) |
| Fast-Pass + translate | Cepat, multibahasa kuat, keluaran JSON patuh | Claude Haiku 5.5; model cepat di Groq hanya bila lolos evaluasi per bahasa |
| Main-Pass | Multibahasa kuat, patuh instruksi peran dan gaya, hemat halusinasi | Claude Haiku 5.5 default; Claude Sonnet 5.5 untuk tier kualitas atau bahasa yang gagal evaluasi |

Pilihan final ditetapkan dari hasil evaluasi di Milestone 4.7, bukan diasumsikan.

### Arsitektur Latensi & Beban (Desktop Tipis)

Desktop dibatasi anggaran sumber daya karena laptop pengguna beragam; server menjalankan seluruh pemrosesan AI sebagai pipeline paralel.

|  | Desktop (klien tipis) | Server (Live Session Service) |
| --- | --- | --- |
| Tugas | Capture WASAPI, downmix + resample 16 kHz mono, gate VAD berbasis energi, kirim audio, terima event, render UI | VAD/endpointing akurat, STT, Fast-Pass, retrieval, Main-Pass LLM, partial JSON parser, metering |
| Tidak dilakukan | Inferensi model, embedding, parsing dokumen, chunking, pemanggilan provider | Rendering UI |
| Anggaran (target) | CPU rata-rata < 5%, RAM < 150 MB, tanpa GPU, unggah ≈ 256 kbps (PCM 16-bit 16 kHz) hanya saat ada suara | Diukur per sesi; kapasitas per instance ditetapkan di load test (Milestone 7.1) |

**Model konkurensi server (goroutine + worker pool).** Satu sesi live = satu WebSocket persisten (tanpa handshake per permintaan) dan satu pipeline tahap-tahap goroutine yang dihubungkan channel ber-buffer terbatas:

1. `ingest`: membaca frame audio; backpressure bila tahap berikutnya penuh.
2. `endpointer`: VAD + pemotongan ucapan; mengirim potongan interim sekitar tiap 2 detik agar transkrip tampil cepat.
3. `stt`: worker pool bersama dengan ukuran dibatasi, memanggil provider STT dengan hint bahasa dari Session Profile dan deteksi bahasa per ucapan; urutan hasil dijaga per sesi.
4. `think`: untuk transkrip final, jalankan **paralel** (`errgroup`) Fast-Pass (terjemahan + intent + rewrite dalam bahasa dokumen) dan retrieval dengan transkrip mentah + tag fokus. Terjemahan ucapan langsung dikirim ke UI tanpa menunggu Main-Pass. Bila rewrite tiba tepat waktu, retrieval kedua dijalankan lalu digabung (RRF); bila tidak, lanjut dengan hasil mentah.
5. `answer`: Main-Pass streaming dengan Session Profile; setiap jawaban yang selesai langsung memicu task `translate` paralel; partial JSON parser mengirim event per field.

Setiap ucapan membawa `context.Context` sendiri: ucapan baru membatalkan jawaban yang masih berjalan.

**Model konkurensi desktop.** Goroutine terpisah untuk capture (callback WASAPI ke ring buffer, prioritas rendah), sender (WebSocket, batch frame 20 sampai 40 ms), dan receiver (event ke Wails). Pembaruan UI di-throttle maksimal satu render per 50 ms.

**Teknik menjaga latensi:**

- Pool koneksi HTTP/2 ke provider dihangatkan saat sesi dimulai.
- Prompt statis (persona, ringkasan profil, chunk dokumen aktif) disusun sekali saat sesi dimulai; prompt caching provider dipakai bila tersedia.
- Fast-Pass paralel dengan retrieval, bukan berurutan; pre-filter heuristik membuang ucapan non-pertanyaan sebelum memanggil LLM.
- Timeout per tahap dengan fallback eksplisit; tidak ada tahap yang boleh menahan seluruh pipeline.
- Metrik latensi per tahap (Prometheus) dan trace per ucapan sejak Milestone 3.4.
- Ketahanan koneksi: reconnect dengan exponential backoff, ring buffer audio di desktop, dan resume sesi lewat `session_id`.

**Anggaran latensi (target P50, belum terukur; divalidasi di Milestone 4.7):**

| Tahap | Target |
| --- | --- |
| Endpointing (akhir ucapan terdeteksi) | ≤ 400 ms |
| Transmisi audio + STT | ≤ 700 ms |
| Fast-Pass paralel dengan retrieval | ≤ 500 ms |
| Main-Pass: field pertama | ≤ 600 ms |
| **Total sampai saran jawaban pertama tampil** | **≤ 2.200 ms (batas keras 2.500 ms)** |
| Terjemahan ucapan (paralel, tidak memblokir jawaban) | ≤ 700 ms setelah transkrip final |
| Terjemahan tiap jawaban (paralel) | ≤ 500 ms setelah teks jawaban selesai |

### Kontrak Ekstensibilitas (Siap Modul Baru)

Fitur baru, mis. Career Studio, ditambahkan sebagai modul baru tanpa mengubah kode inti, skema inti, atau kontrak yang sudah dirilis. Aturan di bawah berlaku sejak Milestone 0.4.

| Titik ekstensi | Aturan di MVP | Fitur baru cukup menambah |
| --- | --- | --- |
| Modul server | Modular monolith: `internal/<modul>` mengimplementasikan interface `Module` (routes, migrasi, job, entitlement) dan didaftarkan di satu registry | Folder modul + satu baris registrasi |
| Skema database | Migrasi per modul (namespace), hanya aditif (expand/contract); kolom spesifik tipe disimpan di `payload` jsonb | Tabel dan migrasi modul baru |
| Tipe dokumen | `documents.type` berupa string dari registry, bukan enum DB; `schema_version` per dokumen | Tipe baru di registry + handler parser |
| Sync | Berbasis `resource kind` (MVP: `documents`, `session_profiles`); cursor per kind; envelope memuat `kind` dan `schema_version`; klien mengabaikan kind yang tidak dikenal | Kind baru di registry server dan desktop |
| Entitlement | Token lisensi memuat `features[]`; kode memeriksa fitur, bukan nama paket | Kunci fitur baru + pemetaan paket |
| Usage metering | `usage_events(kind, units, meta jsonb)` generik | Kind baru, tanpa perubahan skema |
| Tugas LLM | Registry `LLMTask` (`fast_pass`, `translate`, `main_pass`, ...) + prompt ber-versi di `prompts/<task>/vN`; provider, model, dan timeout per tugas (dan override per bahasa) dari `.env` server | Tugas baru + file prompt + variabel env |
| Persona dan bahasa | Peran, gaya, dan bahasa berupa data (registry preset + style pack prompt ber-versi per bahasa), bukan kode; daftar bahasa dari konfigurasi + hasil evaluasi | Preset, gaya, atau bahasa baru = entri data + file prompt + test set |
| Tahap pipeline server | `Transcriber`, `Retriever`, `LLM`, dan `Stage` di balik interface dan terdaftar di registry | Implementasi atau stage baru |
| API | Prefiks `/v1`, perubahan hanya aditif, field tak dikenal diabaikan; `/v1/client-config` memuat versi minimum klien, daftar fitur, dan daftar bahasa | Endpoint baru di modul baru |
| Protokol desktop | Pesan WebSocket ber-versi; tipe pesan tak dikenal diabaikan; nama event ber-namespace (`answer:*`, `sync:*`, `session:*`) | Tipe pesan atau event baru |
| Web Portal | Route group per modul; navigasi dibentuk dari registry entitlement | Route group baru |

Aturan emas: bila fitur baru memaksa perubahan pada kode inti atau kontrak yang sudah rilis, itu dianggap cacat desain dan dicatat di ADR. Milestone 0.4 membuktikan kontrak ini dengan modul contoh.

## 2. Konvensi Kerja

**Format commit (Conventional Commits):** `feat:`, `fix:`, `chore:`, `test:`, `docs:`, `refactor:`.

**Tag:** `v0.<fase>.<sub>` selama pengembangan (mis. Milestone 1.3 → `v0.1.3`). Rilis publik pertama: `v1.0.0`.

**Penomoran tag V2:** Fase 1B memakai `v0.11.x`, Fase 1C memakai `v0.12.x`, Fase 8 memakai `v0.8.x`; fase lain mengikuti nomor fasenya. Fase G memakai `v0.9.x` dan Fase C memakai `v0.10.x`. Karena track berjalan paralel, urutan tag tidak selalu kronologis; setiap tag tetap unik dan hanya dibuat oleh pemilik repo di `main`.

**Definition of Done tiap milestone:**

1. Semua Acceptance Criteria (AC) terpenuhi; `make test` hijau (CI hijau setelah Milestone 0.2).
2. `docs/handover/<track>.md` diperbarui **di PR yang sama** (Bagian 5).
3. PR ditinjau dan disetujui orang lain (kecuali PR pemilik repo sebelum kontributor diundang), lalu di-*squash merge* ke `main` dengan pesan Conventional Commit milestone.
4. Pemilik repo membuat tag pada commit hasil merge (`git tag -a vX.Y.Z -m "..." && git push origin vX.Y.Z`) dan memperbarui baris milestone di `HANDOVER.md` (indeks).
5. Issue milestone ditutup dan kartu di Project board dipindah ke Done.
6. ZIP source tidak lagi wajib. `make zip` (dibuat di 0.2) hanya dipakai bila perlu membawa source ke chat room AI (tanpa `.git`, `node_modules`, `build/`, `*.exe`, `.env`).

**Legenda penanda:** 📦 = wajib handover track diperbarui + tag. 🚩 = Phase Gate (review keseluruhan fase sebelum lanjut).

**Checkpoint (mid-task):** bila konteks chat atau waktu kerja hampir habis, keluarkan **CHECKPOINT STATEMENT** (format di Bagian 5), simpan ke `docs/handover/<track>.md`, commit `wip(<id>): checkpoint` di branch kerja, push, dan buka *draft PR*. Room atau orang baru melanjutkan dari file itu.

### Kolaborasi Tim (2 orang: pemilik repo + 1 kontributor)

Detail ada di `docs/git-workflow.md` dan `CONTRIBUTING.md`. Ringkasnya:

- **Satu milestone = satu issue = satu branch `feat/<id>-<slug>` = satu PR.** Milestone besar boleh dipecah menjadi beberapa PR kecil.
- **`main` terproteksi:** tanpa push langsung; wajib PR, 1 approval dari orang lain, dan status check hijau; hanya squash merge.
- **Kontrak dulu, kode kemudian.** Perubahan yang memengaruhi track lain dimulai dari PR berlabel `contract` pada `docs/contracts/`, disetujui kedua orang, dan di-merge sebelum kode yang bergantung padanya.
- **Satu orang atau satu chat room AI = satu track.** Hanya mengubah file di area track-nya (Peta Track di Bagian 3). Perubahan di luar area dibuat lewat PR terpisah dan ditinjau pemilik area.
- **File rawan konflik** dikelola pemilik repo atau lewat PR kecil tersendiri: `go.work`, `go.mod`, `pnpm-lock.yaml`, `pnpm-workspace.yaml`, `docs/contracts/openapi/` (dipecah per modul), registrasi modul, dan `packages/ui`.
- **Jangan menganggur saat menunggu review.** Ambil issue berlabel `ready` berikutnya di track sendiri. Target waktu review: ≤ 2 hari kerja.
- **Tag hanya dibuat pemilik repo** di `main`.
- **Rahasia tidak pernah masuk repo** (`.env`, kunci, token); hanya `.env.example`.

## 3. Master Checklist

### Peta Track, Gelombang & Dependensi

Tim: **Owner** (pemilik repo) dan **Kontributor**. **Track:** `S` = Server (`/server`, `/deploy`) · `D` = Desktop (`/desktop`, `/tools`) · `W` = Web Portal (`/web`) · `X` = Lintas/tata kelola (repo, `/docs/contracts`, evaluasi, rilis). `/packages/ui` dimiliki Owner dan diubah lewat PR yang ditinjau kedua orang.

**PIC:** Owner, Kontributor, Berdua, atau Fleksibel (siapa yang lebih dulu longgar; dicatat di issue). Asumsi: Kontributor memegang Track D dan memiliki mesin Windows untuk uji WASAPI. Bila tidak, milestone 3.1 sampai 3.3 ditukar ke Owner.

**Gelombang:**

- **W0 (Owner, solo):** G.1–G.3, 0.1. Setelah 0.1 selesai dan di-tag, Kontributor diundang.
- **W1 (paralel penuh):** Owner mengerjakan 0.3, 0.4, 1.1, dan kontrak C.1–C.6. Kontributor mengerjakan 0.2, 2.1–2.3, 3.1–3.2 serta ikut menulis C.3 dan C.4. Tidak ada yang saling menunggu.
- **Gerbang C.8:** Kontrak v0 dibekukan (akhir W1/awal W2).
- **W2:** Owner: 1.2–1.5, 1.7, 1B.1. Kontributor: C.7 (mock), 2.4, 2.5, 3.3.
- **W3:** Owner: 1.6, 1.8, 1B.2–1B.4, 3.4. Kontributor: 2.6, 3.5.
- **W4:** Owner: 4.2–4.5. Kontributor: 4.1, 4.6.
- **W5:** 4.7 (berdua), 5.1 (S), 6.1–6.2 (D), 1C.1–1C.3 (W, fleksibel).
- **W6:** 1C.4–1C.5, 5.2 (opsional), 6.3–6.4.
- **W7:** Fase 7 (berdua/Owner).

**Titik sinkron (diuji bersama terhadap server asli, bukan mock):**

| # | Titik | Setelah | Yang diuji |
| --- | --- | --- | --- |
| 1 | Kontrak v0 beku | C.8 | Kedua orang meng-approve PR pembekuan |
| 2 | Auth & lisensi | 1.4 dan 2.5 | Login, aktivasi, heartbeat dari desktop ke server asli |
| 3 | Sync | 1B.3 dan 2.6 | Edit di desktop/portal ↔ cloud, outbox offline, konflik `409` |
| 4 | Audio end-to-end | 3.4 dan 3.5 | WASAPI → WebSocket → STT → transkrip live |
| 5 | Jawaban live | 4.5 dan 4.6 | Saran jawaban streaming + terjemahan, ukur latensi |
| 6 | Hardening | Fase 7 | E2E, security review, beta |

Kolom "Bergantung pada" berarti milestone tersebut harus sudah di-merge sebelum milestone ini dimulai. Tabel ini dibaca oleh `scripts/create_issues.py`; jaga formatnya.

<!-- TRACK-MAP:START -->

| ID | Judul singkat | Track | PIC | Gel. | Bergantung pada |
| --- | --- | --- | --- | --- | --- |
| G.1 | Repo GitHub & proteksi branch | X | Owner | W0 | – |
| G.2 | Dokumen tata kelola & template | X | Owner | W0 | G.1 |
| G.3 | Issue, label & Project board | X | Owner | W0 | G.2 |
| 0.1 | Repo scaffolding & ADR | X | Owner | W0 | G.3 |
| 0.2 | Tooling & CI | D | Kontributor | W1 | 0.1 |
| 0.3 | Dev infra lokal | S | Owner | W1 | 0.1 |
| 0.4 | Kontrak ekstensibilitas | S | Owner | W1 | 0.1 |
| C.1 | ERD & DDL draf | X | Owner | W1 | 0.1 |
| C.2 | OpenAPI v1 draf | X | Owner | W1 | C.1 |
| C.3 | Protokol WebSocket & event | X | Berdua | W1 | C.1 |
| C.4 | Spesifikasi sync | X | Berdua | W1 | C.1 |
| C.5 | Kontrak prompt & JSON | X | Owner | W1 | C.1 |
| C.6 | Threat model & privasi | X | Owner | W1 | C.1 |
| 1.1 | Server skeleton | S | Owner | W1 | 0.3, 0.4 |
| 2.1 | Wails shell & UI dasar | D | Kontributor | W1 | 0.1 |
| 2.2 | Go engine & event bus | D | Kontributor | W1 | 2.1 |
| 2.3 | SQLite cache + FTS5 | D | Kontributor | W1 | 2.2 |
| 3.1 | WASAPI loopback | D | Kontributor | W1 | 0.1 |
| 3.2 | Mikrofon & mixer | D | Kontributor | W1 | 3.1 |
| C.7 | Mock server dari kontrak | D | Kontributor | W2 | C.2, C.3, C.4 |
| C.8 | Pembekuan kontrak v0 | X | Berdua | W2 | C.1, C.2, C.3, C.4, C.5, C.6 |
| 1.2 | DB & migrasi | S | Owner | W2 | 1.1, C.8 |
| 1.3 | Auth | S | Owner | W2 | 1.2 |
| 1.4 | Licensing | S | Owner | W2 | 1.3 |
| 1.5 | Redis cache & limiter | S | Owner | W2 | 1.1 |
| 1.7 | Provider gateway & metering | S | Owner | W2 | 1.2 |
| 1B.1 | Skema data master | S | Owner | W2 | 1.2 |
| 2.4 | Settings & token aman | D | Kontributor | W2 | 2.2 |
| 2.5 | Client auth & license | D | Kontributor | W2 | 2.4, C.7, C.8 |
| 3.3 | Audio transport (desktop) | D | Kontributor | W2 | 3.1, C.7, C.8 |
| 1.6 | Payment gateway | S | Owner | W3 | 1.4 |
| 1.8 | Admin & observability | S | Owner | W3 | 1.3, 1.4 |
| 1B.2 | Documents API & parser | S | Owner | W3 | 1B.1 |
| 1B.3 | Sync API | S | Owner | W3 | 1B.2 |
| 1B.4 | Session profile, preset & tag | S | Owner | W3 | 1B.1 |
| 3.4 | Live Session Service | S | Owner | W3 | 1.4, 1.7 |
| 2.6 | Sync module desktop | D | Kontributor | W3 | 2.3, C.7 |
| 3.5 | Live transcript UI | D | Kontributor | W3 | 3.3, C.7 |
| 4.1 | Library & session setup | D | Kontributor | W4 | 2.6, C.7 |
| 4.2 | Retriever server | S | Owner | W4 | 1B.1, 1B.4 |
| 4.3 | Task router LLM | S | Owner | W4 | 1.7 |
| 4.4 | Fast-Pass | S | Owner | W4 | 4.2, 4.3, C.5 |
| 4.5 | Main-Pass | S | Owner | W4 | 4.4 |
| 4.6 | Mini window & streaming UI | D | Kontributor | W4 | 3.5, C.7 |
| 4.7 | Evaluasi multibahasa & latensi | X | Berdua | W5 | 4.5, 4.6 |
| 5.1 | Endpoint inferensi kustom | S | Owner | W5 | 1.7, 4.3 |
| 6.1 | Overlay & hotkey | D | Kontributor | W5 | 4.6 |
| 6.2 | Privasi & consent | D | Kontributor | W5 | 3.5 |
| 1C.1 | Scaffold portal & auth UI | W | Fleksibel | W5 | 1.3, C.2 |
| 1C.2 | Pricing & checkout | W | Fleksibel | W5 | 1.6, 1C.1 |
| 1C.3 | Akun, lisensi & perangkat | W | Fleksibel | W5 | 1.4, 1C.1 |
| 1C.4 | Library & CV editor | W | Fleksibel | W6 | 1B.3, 1C.1 |
| 1C.5 | Admin panel | W | Fleksibel | W6 | 1.8, 1C.1 |
| 5.2 | GPU worker (opsional) | S | Owner | W6 | 5.1 |
| 6.3 | Installer, signing, auto-update | D | Kontributor | W6 | 2.5 |
| 6.4 | Crash reporting & telemetri | D | Kontributor | W6 | 2.2 |
| 7.1 | E2E & benchmark | X | Berdua | W7 | 4.7, 6.1, 6.2, 6.3, 6.4, 1C.3 |
| 7.2 | Security review | X | Owner | W7 | 7.1 |
| 7.3 | Deploy produksi & beta | X | Owner | W7 | 7.2, 1C.3 |
| 7.4 | Rilis v1.0.0 | X | Berdua | W7 | 7.3 |

<!-- TRACK-MAP:END -->

**Catatan kapasitas:** dengan 2 orang, Web Portal adalah titik tersempit. Portal beta dipersempit ke 1C.1–1C.3 (akun, bayar, lisensi/unduh); 1C.4–1C.5 menyusul. Bila jadwal terancam, tambahkan kontributor ketiga (idealnya kuat SvelteKit) pada akhir Gelombang 1, saat OpenAPI sudah beku dan `/packages/ui` ada.

### FASE G — Git, GitHub & Tata Kelola Kolaborasi

**\[ \] G.1 Repo GitHub & Proteksi Branch**

- Tugas: buat repo GitHub (private), default branch `main`; aktifkan proteksi `main` (PR wajib, 1 approval, review Code Owners, status check wajib, hanya squash merge, tanpa force-push dan penghapusan, percakapan harus selesai); auto-hapus branch setelah merge; aktifkan Dependabot alerts dan secret scanning bila paket GitHub mendukung; `.gitignore` dan `.gitattributes` (`eol=lf`). Langkah rinci: `docs/github-setup-steps.md`.
- AC: push langsung ke `main` ditolak; hanya opsi squash merge yang tersedia; branch uji terhapus otomatis setelah merge; ketersediaan proteksi branch pada paket GitHub yang dipakai **diverifikasi**, bukan diasumsikan.
- Commit: `chore: initial repository setup`
- Tag: `v0.9.1` — 📦

**\[ \] G.2 Dokumen Tata Kelola & Template**

- Tugas: `CONTRIBUTING.md`, `.github/CODEOWNERS`, template PR dan issue (bug, perubahan kontrak), workflow `repo-hygiene` (menolak `.env`, `*.exe`, `node_modules` yang ter-commit), `docs/git-workflow.md`, `HANDOVER.md` (indeks) dan `docs/handover/{server,desktop,web}.md`, `README.md` awal, `docs/onboarding/`.
- AC: PR uji otomatis memakai template; CODEOWNERS meminta reviewer yang benar; job `repo-hygiene` gagal pada commit uji yang memuat `.env` dan lulus setelah file dihapus; semua placeholder (`@OWNER`, `@CONTRIBUTOR`, `master-abror/nizmeet`) sudah diganti.
- Commit: `docs: add contribution guide, templates, and handover structure`
- Tag: `v0.9.2` — 📦

**\[ \] G.3 Issue, Label & Project Board**

- Tugas: label (`track:*`, `ready`, `blocked`, `contract`, `gate`), GitHub Milestones per Gelombang (W0–W7), satu issue per milestone dibuat otomatis oleh `scripts/create_issues.py` dari dokumen ini (Tugas, AC sebagai checklist, track, PIC, dependensi), Project board (Backlog, Ready, In progress, In review, Done).
- AC: setiap milestone di Bagian 3 punya tepat satu issue; issue yang tidak terblokir berlabel `ready`; skrip aman dijalankan ulang (tidak menggandakan issue).
- Commit: `chore: add issue generation script and project board conventions`
- Tag: `v0.9.3` — 📦 🚩 (Gate: repo siap menerima kontributor setelah Milestone 0.1 selesai)

### FASE 0 — Fondasi

**\[ \] 0.1 Repo Scaffolding & ADR Awal**

- Tugas: init monorepo (`/server`, `/desktop`, `/web`, `/packages/ui`, `/deploy`, `/docs`), `go.mod` per modul Go, scaffold **Wails v2** + Svelte + Tailwind, scaffold SvelteKit + Tailwind, workspace pnpm untuk package UI bersama, `docs/adr/0001-stack.md` dan `docs/adr/0002-cloud-source-of-truth-and-sync.md` (keputusan Bagian 0). Panduan langkah demi langkah: `docs/milestone-0.1-instructions.md`.
- AC: `wails dev` menampilkan halaman Svelte; `pnpm --filter web dev` menampilkan portal; keduanya mengimpor satu komponen dari `/packages/ui`; `go build ./...` sukses di semua modul Go; versi tool ter-pin dan tercatat di README.
- Commit: `chore: scaffold monorepo with desktop, web portal, shared ui, ADRs`
- Tag: `v0.0.1` — 📦

**\[ \] 0.2 Tooling & CI**

- Tugas: `Makefile` (`lint`, `test`, `build`), golangci-lint, `svelte-check`, GitHub Actions (lint+test server dan desktop di runner Windows), lalu job `ci` dijadikan required status check di branch protection (menggantikan `repo-hygiene`), serta `make zip` (opsional, untuk dibawa ke chat room AI).
- AC: pipeline CI hijau pada PR kosong; `make lint test` berjalan lokal.
- Commit: `chore: add makefile, linting, and CI pipeline`
- Tag: `v0.0.2` — 📦

**\[ \] 0.3 Dev Infra Lokal**

- Tugas: `docker-compose.yml` (PostgreSQL 18 + Redis), `.env.example`, skrip `make dev-up`.
- AC: `pg_isready` dan `redis-cli ping` sukses; tidak ada secret ter-commit.
- Commit: `chore: add docker-compose for postgres18 and redis`
- Tag: `v0.0.3` — 📦 🚩

**\[ \] 0.4 Kontrak Ekstensibilitas & Kerangka Modul**

- Tugas: tulis `docs/adr/0003-extension-contract.md`; kerangka `Module` + registry di server (paket mandiri, tanpa HTTP); registry tipe dokumen, resource kind sync, `LLMTask`, dan entitlement; struktur route group di portal; konvensi migrasi per modul; modul contoh `hello` (server + portal) sebagai bukti. Milestone 1.1 dan seterusnya wajib memakai registry ini.
- AC: modul `hello` ditambahkan hanya dengan folder baru + satu baris registrasi, tanpa mengubah file inti lain (dibuktikan lewat diff PR); migrasi modulnya berjalan terpisah dan dapat di-rollback; modul `hello` dihapus tanpa meninggalkan sisa; ADR-0003 direview.
- Commit: `feat: extension contract, module registry, and sample module`
- Tag: `v0.0.4` — 📦 🚩 (Gate: kontrak ekstensibilitas disetujui sebelum Fase 1)

### FASE C — Kontrak & Dokumentasi Teknis (paralel dengan Gelombang 1)

Tujuan: kedua track memegang kontrak tertulis yang sama sebelum menulis kode yang saling bergantung. Semua berkas ada di `docs/contracts/`: `erd.md`, `db/ddl.sql`, `openapi/`, `ws/`, `sync-spec.md`, `prompts/`, `threat-model.md`, `CHANGELOG.md`. Draf boleh dibantu AI tetapi wajib ditinjau manusia. Setelah **C.8**, perubahan hanya lewat PR berlabel `contract`.

**\[ \] C.1 ERD & DDL Draf**

- Tugas: ERD (Mermaid `erDiagram`) dan `db/ddl.sql` PostgreSQL 18 untuk semua tabel Fase 1, 1B, dan modul contoh `hello` (0.4): akun/sesi/perangkat, lisensi/langganan/pembayaran, `usage_events`, dokumen/versi/chunk/tag, session profile/preset, cursor sync, tombstone, audit log. Tetapkan konvensi penamaan, tipe ID (UUID v7; PostgreSQL 18 menyediakan `uuidv7()`, verifikasi saat implementasi), kolom `revision` dan penanda hapus, indeks FTS, serta kebijakan retensi. Nama tabel final ditetapkan di sini.
- AC: `ddl.sql` berjalan bersih pada PostgreSQL 18 kosong (`psql -v ON_ERROR_STOP=1`); setiap tabel milik pengguna memiliki `user_id` dan indeksnya; ERD mencakup semua tabel; Kontributor meninjau dari sudut pandang klien sync.
- Commit: `docs(contracts): add ERD and draft PostgreSQL DDL`
- Tag: `v0.10.1` — 📦

**\[ \] C.2 OpenAPI v1 Draf**

- Tugas: `openapi/` dipecah per modul (`auth`, `licensing`, `billing`, `documents`, `sync`, `presets`, `client-config`, `admin`, `hello`); skema error seragam, prefiks `/v1`, mekanisme auth (access + refresh token; token lisensi Ed25519 untuk perangkat), pagination, idempotency key, header rate-limit, contoh request/response.
- AC: lulus lint OpenAPI (mis. Spectral atau Redocly); semua endpoint yang dibutuhkan Fase 1, 1B, 1C, serta 2.5 dan 2.6 terwakili; contoh dapat dipakai mock server (C.7).
- Commit: `docs(contracts): add OpenAPI v1 draft`
- Tag: `v0.10.2` — 📦

**\[ \] C.3 Protokol WebSocket & Event**

- Tugas: `ws/protocol.md` dan JSON Schema: handshake dan auth, `session.start` (Session Profile), frame audio (format, ukuran, urutan, timestamp), event server→klien (transkrip parsial/final, jawaban per field, terjemahan, status, error), heartbeat, reconnect + resume (sequence number, ring buffer), backpressure, kode close, versi protokol. Ditulis **bersama Kontributor** karena Track D adalah klien utama.
- AC: setiap event punya JSON Schema dan contoh; skenario reconnect/resume tertulis dengan diagram urutan; tidak ada field `TBD`; ditinjau Kontributor.
- Commit: `docs(contracts): add WebSocket protocol and event schemas`
- Tag: `v0.10.3` — 📦

**\[ \] C.4 Spesifikasi Sync**

- Tugas: `sync-spec.md`: model `revision`, cursor per device, tombstone, outbox (`client_mutation_id`, idempoten), `If-Match` dan `409` (server menang, versi lama dapat dipulihkan), urutan pull/push, ukuran halaman, retry/backoff, perilaku offline, tabel keputusan konflik, contoh pesan end-to-end, dan skema SQLite lokal (cache). Ditulis bersama Kontributor.
- AC: ada contoh bernilai konkret untuk 5 skenario (edit normal, edit offline lalu konflik, hapus lalu pulihkan, dua device, retry duplikat); skema SQLite selaras dengan DDL C.1; ditinjau Kontributor.
- Commit: `docs(contracts): add sync specification and local cache schema`
- Tag: `v0.10.4` — 📦

**\[ \] C.5 Kontrak Prompt & JSON**

- Tugas: skema JSON output Fast-Pass (terjemahan, intent, rewrite, tag) dan Main-Pass (`suggestions[]` berisi `text`, `translation`, `grounded`, `sources[]`); struktur style pack per bahasa dan gaya (formal/informal/santai/slang); aturan grounding sebagai teks kontrak; versioning prompt (`prompts/<task>/vN`); kasus uji acuan (*golden cases*) untuk 4.7.
- AC: setiap JSON Schema tervalidasi pada ≥ 10 contoh valid dan ≥ 5 contoh tidak valid; aturan grounding punya contoh benar dan salah; struktur versi prompt terdokumentasi.
- Commit: `docs(contracts): add prompt and structured-output contracts`
- Tag: `v0.10.5` — 📦

**\[ \] C.6 Threat Model & Peta Data Pribadi**

- Tugas: `threat-model.md` (aset, aktor, alur data audio/CV/pembayaran/lisensi, ancaman STRIDE, mitigasi, risiko residual); peta data pribadi terhadap UU PDP (retensi, hapus akun, ekspor); daftar kontrol terukur yang diuji di 7.2. Dokumen ini bukan nasihat hukum; teks hukum direview ahli sebelum rilis (lihat 7.2).
- AC: setiap alur data di Bagian 1 punya baris ancaman dan mitigasi; ada daftar kontrol terukur yang dirujuk Milestone 7.2; ditinjau pemilik repo.
- Commit: `docs(contracts): add threat model and privacy data map`
- Tag: `v0.10.6` — 📦

**\[ \] C.7 Mock Server dari Kontrak (Track D)**

- Tugas: `/tools/mockserver` (modul Go terpisah) yang melayani REST dari OpenAPI C.2 dan WebSocket dari C.3 dengan data skrip (transkrip dan jawaban palsu yang di-stream, simulasi putus koneksi, respons `409`); `make mock`. Direktori `/tools` ditambahkan di milestone ini dan tidak mengubah struktur inti.
- AC: desktop dapat login, sync, dan menerima transkrip serta jawaban streaming dari mock; contract test memeriksa respons mock terhadap skema; skenario putus/resume dan `409` dapat dipicu.
- Commit: `feat(tools): add contract-driven mock server`
- Tag: `v0.10.7` — 📦

**\[ \] C.8 Pembekuan Kontrak v0**

- Tugas: review silang oleh kedua orang untuk C.1–C.6; selaraskan DDL ↔ OpenAPI ↔ sync ↔ WebSocket; tulis `docs/contracts/CHANGELOG.md` dengan versi `0.1.0`; tetapkan aturan perubahan sesudahnya (PR berlabel `contract`, approval keduanya, naikkan versi kontrak, catat di changelog).
- AC: checklist konsistensi silang lulus (setiap field OpenAPI ada di DDL atau diberi alasan; setiap event WebSocket merujuk skema yang ada; skema SQLite lokal selaras dengan DDL); kedua orang meng-approve PR pembekuan; tag dibuat.
- Commit: `docs(contracts): freeze contract v0`
- Tag: `v0.10.8` — 📦 🚩 (Gate: kontrak v0 beku sebelum kode yang bergantung padanya)

### FASE 1 — Cloud SaaS Server

**\[ \] 1.1 Server Skeleton**

- Tugas: chi router, config via env, `slog`, middleware (request-id, recover, CORS, timeout), `/healthz` dan `/readyz`, graceful shutdown.
- AC: `/readyz` gagal bila DB mati; graceful shutdown menyelesaikan request berjalan; test handler lulus.
- Commit: `feat(server): http skeleton with chi, config, logging, health`
- Tag: `v0.1.1` — 📦

**\[ \] 1.2 Database & Migrasi**

- Tugas: `pgxpool`, migrasi (golang-migrate/goose) untuk `users`, `devices`, `licenses`, `plans`, `subscriptions`, `payments`, `usage_events`; repository layer.
- AC: migrasi up/down bersih; integration test (testcontainers) lulus; index pada kolom lookup utama.
- Commit: `feat(server): postgres schema, migrations, repositories`
- Tag: `v0.1.2` — 📦

**\[ \] 1.3 Auth**

- Tugas: register/login, password **argon2id**, JWT access (pendek) + refresh (rotasi, disimpan hash), logout, reset password (stub email interface).
- AC: refresh token reuse terdeteksi dan mencabut sesi; password tidak pernah di-log; test unit + integration lulus; rate limit login belum wajib (di 1.5).
- Commit: `feat(server): auth with argon2id and rotating refresh tokens`
- Tag: `v0.1.3` — 📦

**\[ \] 1.4 Licensing**

- Tugas: model lisensi (plan, masa berlaku, batas device), aktivasi device (fingerprint ter-hash), heartbeat, revoke, grace period offline (mis. 72 jam), token lisensi bertanda tangan (Ed25519) agar klien dapat verifikasi offline.
- AC: device ke-N+1 ditolak; lisensi kedaluwarsa menolak aktivasi; klien dapat memverifikasi token tanpa jaringan; test edge case (clock skew) lulus.
- Commit: `feat(server): licensing, device activation, signed license tokens`
- Tag: `v0.1.4` — 📦

**\[ \] 1.5 Redis: Cache & Rate Limiter**

- Tugas: rate limiter (sliding window/token bucket) per IP, user, dan device; cache plan/lisensi; blokir brute force login.
- AC: melebihi limit mengembalikan `429` + `Retry-After`; limiter tetap aman saat Redis down (fail-open/closed sesuai kebijakan terdokumentasi); load test sederhana lulus.
- Commit: `feat(server): redis rate limiting and caching`
- Tag: `v0.1.5` — 📦

**\[ \] 1.6 Payment Gateway**

- Tugas: interface `PaymentProvider`, implementasi gateway pilihan, checkout session, **webhook dengan verifikasi signature + idempotency key**, aktivasi/perpanjangan lisensi otomatis, refund/cancel.
- AC: webhook ganda tidak menggandakan lisensi; signature palsu ditolak; mode sandbox end-to-end berhasil (bayar → lisensi aktif).
- Commit: `feat(server): payment provider abstraction and webhook handling`
- Tag: `v0.1.6` — 📦

**\[ \] 1.7 Provider Gateway (LLM/STT) & Metering**

- Tugas: lapisan provider di server (interface `LLM` dan `Transcriber`) dengan adapter Claude, OpenAI, Groq, dan adapter generik OpenAI-compatible (siap diarahkan ke vLLM/Ollama lewat env; diuji dengan mock server, uji nyata di Milestone 5.1); **provider, model, endpoint, dan key dibaca dari `.env`/env var per tugas** (Fast-Pass, Main-Pass, STT), tanpa hardcode; streaming; potong kuota per plan (menit audio, token); catat `usage_events`; timeout dan pembatalan.
- AC: mengganti provider, model, atau endpoint cukup dengan mengubah env dan restart (diuji untuk minimal dua provider); `.env.example` memuat semua variabel; key tidak pernah muncul di log atau respons; kuota habis menghasilkan `402/429` yang jelas; streaming tidak di-buffer penuh; usage tercatat akurat.
- Commit: `feat(server): provider gateway for llm and stt with env-driven config and metering`
- Tag: `v0.1.7` — 📦

**\[ \] 1.8 Admin & Observability**

- Tugas: endpoint admin (cari user, cabut lisensi, lihat usage) dengan auth terpisah, metrik Prometheus, dokumen OpenAPI.
- AC: endpoint admin tidak bisa diakses token user biasa; `/metrics` tersedia; `docs/openapi.yaml` sesuai implementasi.
- Commit: `feat(server): admin endpoints, metrics, openapi spec`
- Tag: `v0.1.8` — 📦 🚩 (Gate: pentest ringan + review kontrak API)

### FASE 1B — Data Master, Dokumen & Sync API

**\[ \] 1B.1 Skema Data Master (Profil, CV, Dokumen)**

- Tugas: migrasi `profiles` (headline, ringkasan, skills jsonb, bahasa), `documents` (type: cv/jd/note/portfolio/presentation, title, status, `revision`, `blob_key`, `user_id`), `document_versions`, `document_chunks` (teks, posisi, `tags` jsonb), `tombstones`, `sync_cursors` (per device); index `(user_id, revision)`; enkripsi at-rest; repository dengan scope `user_id` wajib.
- AC: migrasi up/down bersih; test isolasi: user A tidak bisa membaca atau menulis data user B (negative test); `revision` naik monoton; penghapusan membuat tombstone.
- Commit: `feat(server): master data schema for profile, documents, versions, chunks`
- Tag: `v0.11.1` — 📦

**\[ \] 1B.2 Documents API, Object Storage & Parser**

- Tugas: CRUD dokumen; upload multipart (batas ukuran, tipe PDF/DOCX/TXT/MD) ke object storage; job async untuk parse teks; chunker (300 sampai 500 token, overlap) + tagging metadata (skill, perusahaan, peran, periode); versioning di setiap edit; endpoint re-chunk.
- AC: PDF 50 halaman terproses < 10 detik (async); file berbahaya ditolak (MIME sniffing, batas ukuran); tiap edit membuat versi baru dan menaikkan `revision`; chunk deterministik untuk input yang sama.
- Commit: `feat(server): documents api, object storage, parser and chunker`
- Tag: `v0.11.2` — 📦

**\[ \] 1B.3 Sync API (Cloud ke Lokal, dua arah)**

- Tugas: `GET /sync/changes?cursor=` (upsert + tombstone, paginasi, gzip), `POST /sync/ack` (cursor per device), mutasi dari desktop dengan `client_mutation_id` (idempoten) dan `If-Match` (revision) untuk deteksi konflik; rate limit khusus sync.
- AC: device baru menarik seluruh data dengan cursor kosong; mutasi yang sama dikirim ulang tidak menggandakan data; versi basi ditolak `409` beserta data terbaru; penghapusan ikut tersinkron; kontrak tertulis di OpenAPI.
- Commit: `feat(server): revision-based sync api with tombstones and idempotent mutations`
- Tag: `v0.11.3` — 📦 🚩 (Gate: review kontrak sync bersama desktop)

**\[ \] 1B.4 Session Profile, Preset & Tag**

- Tugas: migrasi `session_profiles` (nama, peran, gaya, panjang jawaban, bahasa lawan bicara/jawaban/terjemahan, `focus_tags[]`, dokumen aktif, mode); registry preset bawaan (peran, dan gaya `formal`/`informal`/`santai`/`slang`) sebagai data, bukan kode; tabel `tags` + `tag_aliases` (normalisasi, mis. go = golang) dan tag per chunk hasil ekstraksi; deteksi bahasa dokumen saat parse (`documents.lang`); API CRUD profil; daftar bahasa dari konfigurasi lewat `/v1/client-config`; resource kind sync `session_profiles`.
- AC: profil tersimpan dan tersinkron ke desktop; alias tag menghasilkan tag kanonik yang sama; setiap dokumen memiliki `lang`; preset baru dapat ditambah hanya dengan entri data; isolasi `user_id` lulus negative test.
- Commit: `feat(server): session profiles, presets, tags with aliases, document language`
- Tag: `v0.11.4` — 📦

### FASE 1C — Web Portal (SvelteKit)

**\[ \] 1C.1 Scaffold Portal & Auth UI**

- Tugas: layout, register/login/verifikasi email, penanganan sesi aman (cookie httpOnly), route terlindungi, i18n ID/EN dasar, komponen dari `/packages/ui`, header CSP.
- AC: login/logout dan refresh sesi berfungsi; route terlindungi menolak tamu; skor accessibility Lighthouse ≥ 90 pada halaman auth.
- Commit: `feat(web): sveltekit portal scaffold, auth ui, shared ui package`
- Tag: `v0.12.1` — 📦

**\[ \] 1C.2 Pricing, Checkout & Billing**

- Tugas: landing + pricing, checkout lewat `PaymentProvider` (Milestone 1.6), halaman langganan, invoice, batal/ubah paket.
- AC: pembayaran sandbox end-to-end membuat lisensi aktif dan tampil di portal < 30 detik; invoice dapat diunduh; pembatalan berlaku sesuai kebijakan.
- Commit: `feat(web): pricing, checkout and billing pages`
- Tag: `v0.12.2` — 📦

**\[ \] 1C.3 Akun, Lisensi, Perangkat & Unduhan**

- Tugas: dashboard lisensi, daftar device + cabut aktivasi, kuota dan pemakaian, halaman unduh installer + changelog.
- AC: cabut device efektif pada heartbeat berikutnya; angka kuota sama dengan data server; tautan unduh hanya untuk akun berlisensi aktif.
- Commit: `feat(web): account, license, device and download pages`
- Tag: `v0.12.3` — 📦

**\[ \] 1C.4 Library & CV Editor Dasar**

- Tugas: daftar dokumen, upload, editor teks/Markdown untuk CV, riwayat versi + pulihkan, hapus, indikator status sync per dokumen.
- AC: dokumen yang diedit di portal tiba di desktop setelah sync (diuji bersama Milestone 2.6); upload di portal terparse menjadi chunk; konflik tampil jelas dan tidak menghilangkan data.
- Commit: `feat(web): document library and cv editor with version history`
- Tag: `v0.12.4` — 📦

**\[ \] 1C.5 Admin Panel**

- Tugas: UI admin (cari user, lisensi, usage, cabut akses) memakai endpoint Milestone 1.8, auth admin terpisah + 2FA, audit log.
- AC: akun user biasa mendapat `403` di semua route admin; setiap aksi admin tercatat di audit log.
- Commit: `feat(web): admin panel with audit log`
- Tag: `v0.12.5` — 📦 🚩 (Gate: alur beli sampai lisensi aktif diuji penuh di portal)

### FASE 2 — Desktop Core

**\[ \] 2.1 Wails Shell & UI Foundation**

- Tugas: layout Svelte (Setup, Session, Library, Settings), Tailwind theme (dark default), routing sederhana, state store.
- AC: navigasi antar layar berfungsi; build Windows menghasilkan `.exe` yang berjalan; `svelte-check` bersih.
- Commit: `feat(desktop): wails shell and svelte ui foundation`
- Tag: `v0.2.1` — 📦

**\[ \] 2.2 Go Engine & Event Bus**

- Tugas: modul engine (lifecycle sesi, event bus internal), jembatan Wails events untuk streaming ke UI; `net/http` lokal hanya bila dibutuhkan (bind `127.0.0.1`, token acak per proses).
- AC: event dummy mengalir engine → UI < 50 ms; tidak ada port terbuka ke jaringan luar.
- Commit: `feat(desktop): engine lifecycle and event bridge`
- Tag: `v0.2.2` — 📦

**\[ \] 2.3 SQLite (modernc) + FTS5**

- Tugas: koneksi WAL, migrasi lokal, tabel cache `documents`, `chunks`, `chunk_tags`, `chunks_fts` (FTS5), tabel lokal `sync_state` (cursor, waktu sync terakhir), `outbox` (mutasi tertunda), `sessions`, `transcripts`, `settings`; migrasi dirancang agar `chunk_embeddings` dapat ditambah kemudian.
- AC: build tanpa CGO sukses; query FTS5 BM25 berfungsi; migrasi idempoten; benchmark: pencarian 10 ribu chunk < 30 ms; cache dapat dihapus dan dibangun ulang penuh dari cloud.
- Commit: `feat(desktop): sqlite pure-go cache schema with fts5 and sync tables`
- Tag: `v0.2.3` — 📦

**\[ \] 2.4 Settings & Penyimpanan Token Aman**

- Tugas: simpan token akses/refresh lewat Windows Credential Manager (atau DPAPI); UI Settings (perangkat audio, bahasa, hotkey, tampilan). **Tidak ada API key provider dan tidak ada endpoint model di desktop.**
- AC: tidak ada secret di SQLite atau log; logout menghapus token; tidak ada field konfigurasi provider di UI.
- Commit: `feat(desktop): secure token storage and settings ui`
- Tag: `v0.2.4` — 📦

**\[ \] 2.5 Client Auth & License**

- Tugas: login ke server, simpan refresh token aman, aktivasi device, verifikasi token lisensi offline, heartbeat, UI status plan/kuota, grace period offline.
- AC: tanpa lisensi valid sesi tidak bisa dimulai; offline dalam masa grace tetap jalan; device terbalik/clone terdeteksi.
- Commit: `feat(desktop): client auth and license enforcement`
- Tag: `v0.2.5` — 📦 🚩

**\[ \] 2.6 Sync Module (Cloud ke Lokal + Outbox)**

- Tugas: sync engine di goroutine terpisah (tarik saat aplikasi dibuka, tombol Sync Now, berkala saat online); terapkan upsert + tombstone dalam satu transaksi; perbarui indeks FTS5 secara inkremental; `outbox` untuk unggahan dari desktop (retry dengan backoff, idempoten via `client_mutation_id`); status di UI (terakhir sync, jumlah perubahan, error); mode offline hanya membaca cache.
- AC: perubahan di portal tampil di desktop < 5 detik setelah Sync Now; 1.000 chunk tersinkron < 3 detik; proses dimatikan di tengah sync tidak merusak data dan dilanjutkan dari cursor terakhir; unggahan offline terkirim saat online tanpa duplikat; sesi live tidak terblokir saat sync berjalan.
- Commit: `feat(desktop): cloud-to-local sync module with outbox and incremental fts5 index`
- Tag: `v0.2.6` — 📦 🚩 (Gate: uji end-to-end portal → cloud → desktop)

### FASE 3 — Audio, Streaming & STT

**\[ \] 3.1 WASAPI Loopback Capture**

- Tugas: capture output default device (PCM float/16-bit), resample ke 16 kHz mono, tangani ganti device & sample rate, start/stop.
- AC: audio terekam dari YouTube/Zoom uji; tidak ada glitch > 1% pada 30 menit; CPU < 5% rata-rata; tanpa CGO bila memungkinkan (syscall/`go-ole`), catat keputusan di ADR.
- Commit: `feat(audio): wasapi loopback capture with resampling`
- Tag: `v0.3.1` — 📦

**\[ \] 3.2 Mikrofon & Mixer (Opsional per Setting)**

- Tugas: capture mikrofon, label sumber ("Lawan bicara" / "Saya"), jalur terpisah ke STT.
- AC: kedua sumber terpisah dan berlabel; mute mikrofon berfungsi; tidak ada echo di transkrip.
- Commit: `feat(audio): microphone capture with source labeling`
- Tag: `v0.3.2` — 📦

**\[ \] 3.3 Audio Transport: Gate VAD Ringan & Streaming WebSocket (Desktop)**

- Tugas: gate VAD berbasis energi (murah, pure Go) agar audio hening tidak dikirim; frame 20 sampai 40 ms PCM 16-bit 16 kHz mono; klien WebSocket persisten (JWT, `session_id`, heartbeat); ring buffer + reconnect dengan backoff dan resume; goroutine terpisah untuk capture, sender, dan receiver; protokol ber-versi (tipe pesan tak dikenal diabaikan).
- AC: CPU rata-rata < 5% dan RAM < 150 MB selama sesi 30 menit pada laptop spesifikasi rendah; audio hening tidak dikirim; putus koneksi 10 detik tidak menghilangkan ucapan (dilanjutkan setelah reconnect); bandwidth unggah tercatat dan sesuai estimasi.
- Commit: `feat(desktop): light vad gate and resilient websocket audio streaming`
- Tag: `v0.3.3` — 📦

**\[ \] 3.4 Live Session Service (Server): Ingest, Endpointing & STT**

- Tugas: endpoint WebSocket server dengan autentikasi, lisensi, dan kuota per sesi; pipeline goroutine (`ingest` → `endpointer` → `stt` worker pool) dengan channel ber-buffer dan backpressure; VAD/endpointing akurat + potongan interim; `Transcriber` lewat Provider Gateway (Groq `whisper-large-v3-turbo`); hint bahasa dari Session Profile atau deteksi bahasa per ucapan (`detected_lang`); vocabulary hint (tag dan istilah dari dokumen aktif) lewat parameter prompt STT; penanganan campuran bahasa (mis. ID/EN); retry dengan backoff; metrik latensi per tahap + trace per ucapan. Local Whisper dikeluarkan dari MVP.
- AC: STT P50 ≤ 700 ms per ucapan pendek (termasuk jaringan); endpointing ≤ 400 ms; deteksi bahasa benar ≥ 95% pada set uji Tier 1; WER set uji Tier 1 ≤ 15% (target awal, disesuaikan setelah baseline); mengganti bahasa di tengah sesi berlaku pada ucapan berikutnya; provider gagal diatasi retry atau provider cadangan tanpa memutus sesi; tidak ada goroutine leak setelah sesi ditutup.
- Commit: `feat(server): live session service with multilingual stt worker pool`
- Tag: `v0.3.4` — 📦

**\[ \] 3.5 Live Transcript UI**

- Tugas: panel transkrip real-time (partial → final), auto-scroll, penanda pembicara, simpan ke `transcripts`.
- AC: transkrip muncul < 1,5 detik setelah ucapan selesai; sesi tersimpan dan dapat dibuka ulang.
- Commit: `feat(ui): live transcript panel with persistence`
- Tag: `v0.3.5` — 📦 🚩 (Gate: uji 1 jam rekaman tanpa crash/memory leak)

### FASE 4 — Smart RAG Pipeline (Agentic RAG Lite) & LLM

**\[ \] 4.1 Library, Session Setup & Konteks Aktif (Desktop)**

- Tugas: layar Library (cache SQLite, pencarian lokal ringan, unggah lewat `outbox`); layar **Session Setup**: pilih atau simpan preset, peran, gaya (formal/informal/santai/slang), panjang jawaban, bahasa lawan bicara (auto atau tetap), bahasa jawaban (ikuti lawan bicara atau tetap), bahasa terjemahan, tag fokus (pilih dari tag dokumen atau tambah sendiri), dan dokumen aktif; seluruh pilihan dikirim ke server saat sesi dimulai dan dapat diubah di tengah sesi lewat pesan `session:update`.
- AC: preset tersimpan dan muncul di perangkat lain setelah sync; mengubah gaya, bahasa, atau tag di tengah sesi berlaku pada ucapan berikutnya tanpa memulai ulang; tidak ada parsing atau chunking di desktop.
- Commit: `feat(desktop): library and session setup with role, style, language and tag presets`
- Tag: `v0.4.1` — 📦

**\[ \] 4.2 Retriever di Server (PostgreSQL FTS + Pemanasan Sesi)**

- Tugas: interface `Retriever` + `PgRetriever` (full-text search dengan GIN index; konfigurasi bahasa dipilih dari `documents.lang`); tag fokus pengguna (kanonik) menjadi boost dan filter opsional; pemanasan chunk dokumen aktif saat sesi dimulai; multi-query dalam bahasa dokumen + RRF; filter `user_id` dan koleksi wajib; sanitasi query.
- AC: 30 pertanyaan uji per pasangan bahasa Tier 1 (termasuk pertanyaan Inggris terhadap CV Indonesia dan sebaliknya): relevan di top-5 ≥ 80% dengan rewrite lintas bahasa; memilih tag fokus menaikkan peringkat chunk bertag sama; latensi retrieval P95 ≤ 50 ms; tidak ada hasil lintas user (negative test); implementasi dapat diganti lewat interface.
- Commit: `feat(server): postgres fts retriever with session warm-up, tag boosting, cross-language queries`
- Tag: `v0.4.2` — 📦

**\[ \] 4.3 Task Router LLM (Fast-Pass & Main-Pass)**

- Tugas: registry `LLMTask` (`fast_pass`, `translate`, `main_pass`, tugas lain nanti) dengan provider, model, dan timeout per tugas dari env, plus override per bahasa; prompt ber-versi di `prompts/<task>/vN`; streaming, pembatalan, penghitungan usage; pool HTTP/2 dihangatkan saat sesi dimulai; prompt caching bila provider menyediakan.
- AC: semua adapter lulus suite kontrak yang sama (mock server); ganti model per tugas atau per bahasa hanya lewat env; pembatalan menghentikan stream < 200 ms; error upstream dipetakan ke error domain; daftar model terbaru diverifikasi di dokumentasi provider.
- Commit: `feat(server): llm task router with per-task and per-language env config`
- Tag: `v0.4.3` — 📦

**\[ \] 4.4 Fast-Pass: Terjemahan, Intent & Query Rewriter (Server)**

- Tugas: pre-filter heuristik; satu panggilan Fast-Pass (model kecil-cepat multibahasa, timeout 700 ms) dengan keluaran berurutan `{translation, detected_lang, is_question, intent, rewritten_queries[], tags[]}`; `translation` di-stream lebih dulu dan langsung dikirim ke UI; `rewritten_queries` dalam bahasa dokumen aktif; terjemahan dilewati bila bahasa ucapan sama dengan bahasa terjemahan; opsi "terjemahkan semua ucapan" atau "hanya pertanyaan"; berjalan paralel dengan retrieval; fallback ke hasil mentah bila timeout; jendela riwayat percakapan pendek.
- AC: terjemahan ucapan tampil ≤ 700 ms setelah transkrip final (P50); akurasi intent ≥ 85% pada 50 ucapan uji campuran bahasa; rewrite lintas bahasa memenuhi AC Milestone 4.2; Fast-Pass tidak pernah menahan jalur utama; prompt injection di transkrip tidak mengubah schema keluaran.
- Commit: `feat(server): fast-pass translation, intent and cross-language query rewrite`
- Tag: `v0.4.4` — 📦

**\[ \] 4.5 Main-Pass: Persona, Grounding & Terjemahan Jawaban**

- Tugas: prompt builder dari Session Profile (peran, gaya, panjang, bahasa jawaban, tag fokus, mode) dengan style pack per bahasa (`prompts/answer/vN/style/<preset>`); aturan grounding (pengalaman pribadi hanya dari chunk, pengetahuan umum untuk tag fokus ditandai `grounded: false`); keluaran `suggested_answers[{length, text, grounded, sources[]}]` di-stream per jawaban; setiap jawaban selesai langsung memicu task `translate` paralel ke bahasa terjemahan; event `answer:field`, `answer:translation`, `answer:done`, `answer:error`; pembatalan saat ucapan baru.
- AC: jawaban memakai bahasa lawan bicara (atau bahasa jawaban yang ditetapkan) dan terjemahannya tampil ≤ 500 ms setelah teks jawaban selesai; peran dan gaya (formal/informal/santai/slang) konsisten pada set uji; tidak ada perusahaan, proyek, angka, atau tahun yang tidak ada di chunk (test halusinasi); jawaban tanpa dukungan dokumen ditandai `grounded: false`; P50 field pertama ≤ 600 ms; JSON tidak valid jatuh ke teks mentah tanpa crash.
- Commit: `feat(server): persona-aware main-pass with grounding and parallel answer translation`
- Tag: `v0.4.5` — 📦

**\[ \] 4.6 Mini Window: Transkrip, Terjemahan & Saran Jawaban**

- Tugas: mini window (overlay dasar; perilaku lanjutan di Milestone 6.1) yang menampilkan ucapan lawan bicara dalam bahasa asli dengan terjemahan di bawahnya, badge intent, dan saran jawaban (bahasa jawaban) dengan terjemahan di bawah tiap jawaban; badge "dari dokumen" atau "umum" beserta sumber chunk; kontrol cepat untuk gaya, panjang, bahasa, tag fokus, regenerate, dan salin; indikator status koneksi.
- AC: terjemahan dan saran tampil progresif tanpa flicker; mengubah gaya, bahasa, atau tag lewat kontrol cepat berlaku pada ucapan berikutnya; teks panjang tidak keluar dari window; CPU desktop tetap dalam anggaran.
- Commit: `feat(ui): mini window with translations, grounded answers and quick controls`
- Tag: `v0.4.6` — 📦

**\[ \] 4.7 Evaluasi Multibahasa & Kualitas Jawaban**

- Tugas: harness evaluasi per bahasa untuk STT (WER), kualitas terjemahan (rubrik + sampel manual), kepatuhan peran dan gaya, groundedness (tanpa fakta karangan), retrieval lintas bahasa, dan latensi end-to-end; tetapkan tier bahasa (Tier 1: Indonesia dan Inggris; Tier 2: bahasa besar lain, mis. Mandarin, Jepang, Korea, Spanyol, Prancis, Jerman, Arab, Portugis; sisanya beta); pilih model final per tugas dan per bahasa berdasarkan hasil.
- AC: Tier 1 memenuhi target kualitas dan latensi (WER ≤ 15%, skor terjemahan ≥ 4 dari 5, tanpa fakta karangan pada set uji); setiap bahasa yang dirilis lolos ambang minimal atau diberi label beta di UI; keputusan model dicatat di ADR; laporan latensi end-to-end terdokumentasi.
- Commit: `test(eval): multilingual quality, grounding and latency evaluation harness`
- Tag: `v0.4.7` — 📦 🚩 (Gate: latensi 2,5 detik dan kualitas Tier 1 tercapai, atau ada rencana perbaikan)

### FASE 5 — Opsi Self-Hosted GPU

**\[ \] 5.1 Endpoint Inferensi Kustom (Server, via .env)**

- Tugas: memakai adapter OpenAI-compatible dari Milestone 1.7 untuk vLLM/Ollama di GPU server milik Anda, dipilih per tugas lewat env (URL, model, key, timeout); milestone ini menambahkan: health check berkala + circuit breaker; panduan `docs/self-hosted.md`; kebijakan TLS dan jaringan privat.
- AC: memindahkan Fast-Pass atau Main-Pass ke GPU server cukup dengan mengubah env; berjalan dengan vLLM dan Ollama (uji manual); endpoint mati membuka circuit breaker dan jatuh ke provider cloud tanpa memutus sesi; tidak ada konfigurasi endpoint di desktop.
- Commit: `feat(server): self-hosted inference endpoint via env with circuit breaker`
- Tag: `v0.5.1` — 📦

**\[ \] 5.2 (Opsional) GPU Worker Gateway di Server**

- Tugas: route SaaS ke worker GPU milik sendiri, antrian, autentikasi worker, metering.
- AC: failover ke cloud bila worker mati; beban uji 20 sesi paralel stabil.
- Commit: `feat(server): gpu worker routing and failover`
- Tag: `v0.5.2` — 📦 🚩

### FASE 6 — Polish Produk

**\[ \] 6.1 Overlay & Hotkey**

- Tugas: jendela overlay ringan (always-on-top, transparan, click-through opsional), hotkey global (tampil/sembunyi, regenerate, mute), posisi dan ukuran tersimpan.
- AC: hotkey bekerja saat aplikasi lain fokus; overlay tidak mencuri fokus; perilaku pada multi-monitor benar.
- Commit: `feat(desktop): overlay window and global hotkeys`
- Tag: `v0.6.1` — 📦

**\[ \] 6.2 Privasi, Consent & Data Control**

- Tugas: layar consent & disclaimer saat pertama dan per sesi, opsi tidak menyimpan audio/transkrip, tombol hapus semua data lokal, ekspor sesi (MD/TXT).
- AC: mode "tanpa simpan" tidak meninggalkan data di disk; hapus data terverifikasi; teks hukum direview.
- Commit: `feat(desktop): privacy controls, consent flow, export`
- Tag: `v0.6.2` — 📦

**\[ \] 6.3 Installer, Code Signing & Auto-Update**

- Tugas: installer (NSIS/MSI via Wails), code signing, auto-update dengan verifikasi tanda tangan, kanal stable/beta.
- AC: instalasi bersih di Windows 10/11 bersih; update tidak bisa dipalsukan; SmartScreen tidak memblokir pada build signed.
- Commit: `feat(desktop): installer, signing, secure auto-update`
- Tag: `v0.6.3` — 📦

**\[ \] 6.4 Crash Reporting & Telemetri Minimal**

- Tugas: laporan crash opt-in, telemetri anonim opt-in, scrubbing PII, `slog` ke file dengan rotasi.
- AC: tidak ada transkrip/dokumen/key dalam laporan; opt-out efektif.
- Commit: `feat(desktop): opt-in crash reporting and log rotation`
- Tag: `v0.6.4` — 📦 🚩

### FASE 7 — Hardening & Rilis

**\[ \] 7.1 Test E2E & Benchmark**

- Tugas: skenario E2E (login → lisensi → sesi → jawaban), benchmark latensi dan memori, soak test 2 jam, load test sesi live paralel (kapasitas per instance server).
- AC: target latensi (Bagian 1, *Anggaran latensi*) tercapai; memori stabil; tidak ada goroutine leak.
- Commit: `test: e2e scenarios, latency and soak benchmarks`
- Tag: `v0.7.1` — 📦

**\[ \] 7.2 Security Review**

- Tugas: review threat model, dependency audit (`govulncheck`, `npm audit`), secret scanning, hardening header/CORS, backup & restore PostgreSQL.
- AC: tidak ada temuan High/Critical terbuka; restore backup teruji.
- Commit: `chore(security): audit fixes, hardening, backup runbook`
- Tag: `v0.7.2` — 📦

**\[ \] 7.3 Deploy Produksi & Beta**

- Tugas: deployment server (container + reverse proxy TLS), monitoring/alert, deploy Web Portal, beta tertutup, runbook insiden.
- AC: uptime check aktif; 10 beta user menyelesaikan alur bayar → pakai; backlog bug terpetakan.
- Commit: `chore(release): production deploy and beta rollout`
- Tag: `v0.7.3` — 📦

**\[ \] 7.4 Rilis v1.0.0**

- Tugas: perbaikan bug beta, dokumentasi pengguna, changelog, rilis.
- AC: tidak ada bug Critical terbuka; semua checklist di atas `[x]`.
- Commit: `chore(release): v1.0.0`
- Tag: `v1.0.0` — 📦 🚩

### FASE 8 — Career Studio (Web Portal, Pasca-v1.0)

**Dicadangkan, tidak dirinci di dokumen ini.** Fase ini akan direncanakan di dokumen terpisah setelah MVP. Kandidat modul: AI CV Generator, Interview Simulator, Portfolio Web Builder. Ketiganya harus bisa ditambahkan lewat Kontrak Ekstensibilitas (Bagian 1) tanpa mengubah milestone MVP; prasyaratnya Milestone 0.4 lulus.

## 4. Backlog Pasca-v1.0

Migrasi ke Wails v3 setelah rilis stabil, Local Whisper (mode offline opsional), BYOK untuk pengguna tingkat lanjut, retrieval lokal sebagai cadangan offline, `HybridRetriever` (FTS + embedding) di atas interface `Retriever` yang sama, dukungan macOS/Linux, integrasi kalender (konteks meeting), ringkasan dan action items otomatis setelah meeting, tim/enterprise plan, multi-bahasa UI, merge sync tingkat-field (menggantikan server-menang).

## 5. Checkpoint & Handover (per Track)

**Tujuan.** Chat room AI atau orang baru tidak punya ingatan; ia hanya tahu apa yang tertulis di repo. Karena itu semua state kerja disimpan sebagai berkas, di-commit, lalu diserahkan ke room baru.

**Beda keduanya.** *Checkpoint* menyelamatkan pekerjaan di tengah milestone (konteks atau waktu habis). *Handover* adalah serah terima di batas milestone. Keduanya memakai berkas yang sama.

### 5.1 Struktur berkas

- `HANDOVER.md` (root) = **indeks**. Hanya pemilik repo yang mengedit, saat membuat tag.
- `docs/handover/server.md`, `desktop.md`, `web.md` = status tiap track. Hanya pemilik track yang mengedit, di PR milestone masing-masing (sehingga tidak ada konflik merge).
- Log checkpoint ada di berkas track (entri terbaru di paling atas). *Draft PR* berisi salinan checkpoint terakhir.

### 5.2 Template `HANDOVER.md` (indeks)

```markdown
# PROJECT HANDOVER — INDEKS

- **Versi master plan:** V2.2
- **Tag terakhir di `main`:** [mis. v0.1.2]
- **Versi ter-pin:** Go [..] · Node [..] · pnpm [..] · Wails v2 [..] · PostgreSQL 18.[..] · Redis [..]
- **Kontrak:** `docs/contracts/` versi [..] (dibekukan di C.8: ya/tidak)

| Track | PIC | Milestone aktif | Berkas status | Diperbarui |
| --- | --- | --- | --- | --- |
| S Server | Owner | [..] | docs/handover/server.md | [tanggal] |
| D Desktop | Kontributor | [..] | docs/handover/desktop.md | [tanggal] |
| W Web | [..] | [..] | docs/handover/web.md | [tanggal] |

**Titik sinkron berikutnya:** [..]
**Blocker lintas-track:** [..]
```

### 5.3 Template berkas track

```markdown
# HANDOVER — TRACK [S|D|W]

- **PIC:** [..]
- **Current Milestone:** [mis. 1.2 - Database & Migrasi]
- **Branch / Tag terakhir:** [mis. feat/1.2-db-migrations / v0.1.1]
- **Completed Features:**
  - [x] Feature A
- **In-Progress / Next Task:** [persis: mulai dari file dan fungsi mana]
- **Area berkas milik track ini:** [path]
- **Key Files Modified:**
  - `path/to/file1.go`
- **Dependensi lintas track:** [versi kontrak; menunggu apa dari track lain]
- **Context / Special Notes:** [keputusan, ADR terkait, pinned versions, jebakan yang ditemukan]

## Log Checkpoint (terbaru di atas)

### [YYYY-MM-DD HH:MM] — [milestone]
[CHECKPOINT STATEMENT]
```

### 5.4 CHECKPOINT STATEMENT (mid-task)

```markdown
# CHECKPOINT STATEMENT
- **Track / Milestone:** [..]
- **Branch & commit terakhir:** [..]
- **Ringkasan:** [apa yang sudah jalan, apa yang belum]
- **Berkas diubah:** [path + baris/fungsi logika terakhir yang ditulis]
- **Keputusan yang diambil:** [dan alasannya]
- **Langkah berikutnya (persis):** [..]
- **Blocker / pertanyaan terbuka:** [..]
- **Pesan commit WIP:** `wip(<id>): checkpoint — <ringkas>`
```

### 5.5 Alur menyimpan dan melanjutkan

**Menyimpan (room lama):** (1) keluarkan CHECKPOINT STATEMENT sebelum konteks benar-benar habis; (2) salin ke log di `docs/handover/<track>.md`; (3) `git add -A && git commit -m "wip(<id>): checkpoint"` di branch `feat/<id>-<slug>` lalu push; (4) buka atau perbarui *draft PR* yang memuat checkpoint terakhir.

**Melanjutkan (room baru):** (1) `git fetch && git checkout <branch>`; (2) buka room baru, lampirkan berkas handover track, berkas yang disebut di checkpoint (atau hasil `make zip`), dan kontrak yang relevan; (3) tempel kalimat pembuka di 5.6; (4) room baru **mengonfirmasi pemahamannya terlebih dahulu**, baru melanjutkan dari "In-Progress / Next Task".

### 5.6 Kalimat pembuka room baru

> Saya melanjutkan **Track [S|D|W]** dari Checkpoint/Handover **[milestone]**. Aturan: (1) hanya ubah berkas di area track ini: [path]; (2) jangan mengubah `docs/contracts/` kecuali lewat PR berlabel `contract`; (3) ikuti `CONTRIBUTING.md`; (4) konfirmasi pemahamanmu sebelum menulis kode. Berikut isi `docs/handover/[track].md`: [tempel isi]

## 6. Risiko Utama

| Risiko | Mitigasi |
| --- | --- |
| Latensi total > 3 detik | Pipeline paralel di server (Fast-Pass ∥ Retrieval), streaming per field, timeout per tahap dengan fallback, pre-filter heuristik, dan pengukuran latensi per tahap (benchmark di 7.1) |
| Laptop pengguna berspesifikasi rendah | Desktop tipis: tanpa inferensi lokal, anggaran CPU < 5% dan RAM < 150 MB, diuji di perangkat minimum |
| Beban dan biaya server naik seiring sesi paralel | Worker pool terbatas, backpressure, load test di 7.1, autoscaling, metering + kuota per plan |
| Kualitas bahasa tidak merata | Tier bahasa, evaluasi per bahasa di 4.7, label beta di UI, override model per bahasa |
| Jawaban mengarang pengalaman | Aturan grounding, `grounded: false` + `sources[]`, test halusinasi di 4.5 dan 4.7 |
| Retrieval lintas bahasa gagal | Rewrite dalam bahasa dokumen, tag kanonik, `HybridRetriever` dinaikkan prioritasnya bila perlu |
| Gaya slang atau santai menghasilkan jawaban tidak pantas | Style pack per bahasa dengan batas kepantasan, pengguna dapat berganti gaya seketika |
| Audio pengguna melewati server | Consent eksplisit, TLS, audio mentah tidak disimpan secara default, kebijakan privasi yang jelas |
| Jaringan pengguna lambat atau putus | Gate VAD hemat bandwidth, ring buffer + reconnect + resume sesi, indikator status koneksi |
| WASAPI tanpa CGO rumit | Prototipe 3.1 lebih awal; fallback library CGO dicatat di ADR |
| Biaya API membengkak | Metering + kuota per plan; pre-filter agar ucapan non-pertanyaan tidak memicu LLM; terjemahan dapat dibatasi ke pertanyaan saja |
| Lisensi dibajak | Token Ed25519, device binding, heartbeat, grace period terbatas |
| Risiko hukum/ToS perekaman | Consent flow, disclaimer, mode tanpa simpan, teks hukum direview |
| Kebocoran data CV pengguna | Scope `user_id` di semua query + test isolasi, enkripsi at-rest, hapus akun menyeluruh, pentest di Fase 7 |
| Data korup atau konflik saat sync | Revision + cursor + tombstone, mutasi idempoten, transaksi, versi dokumen dapat dipulihkan |
| Cakupan melebar (Career Studio) | Dipisah ke Fase 8 setelah v1.0; Kontrak Ekstensibilitas menjaga inti tetap stabil |
| JSON streaming gagal di-parse | Partial JSON parser di server + fallback ke teks mentah |
| Konteks chat habis | Protokol Checkpoint dan handover per track (Bagian 5), disimpan di repo |
| Konflik merge dan bottleneck review (tim 2 orang) | Track dan area berkas terpisah (CODEOWNERS), handover per track, berkas rawan konflik dikelola pemilik, target review ≤ 2 hari, ambil issue `ready` lain saat menunggu |
| Kontrak menyimpang antar track | Fase C, pembekuan v0 (C.8), PR berlabel `contract`, contract test pada mock server dan server asli |
| Mock server berbeda dari server asli | Titik sinkron 2 sampai 5 diuji terhadap server asli; contract test dua arah |
| Kapasitas terbatas untuk Web Portal | PIC fleksibel; portal beta dipersempit ke 1C.1–1C.3; opsi kontributor ketiga ditinjau di akhir Gelombang 1 |
| Wails v3 masih beta | Pakai v2; kode Wails terisolasi; migrasi masuk backlog |
| Proteksi branch tidak tersedia di paket GitHub yang dipakai | Diverifikasi di G.1 sebelum kontributor diundang |

## 7. Langkah Berikutnya

1. ✅ Cakupan v1.0, cloud sebagai sumber kebenaran, dan aturan sync sudah disetujui. ✅ Wails v2 diputuskan (v3 masih beta).
2. **Owner:** jalankan Fase G (G.1–G.3) memakai `docs/github-setup-steps.md`; ganti semua placeholder.
3. **Owner:** jalankan Milestone 0.1 memakai `docs/milestone-0.1-instructions.md`; merge dan beri tag `v0.0.1`.
4. **Owner:** undang Kontributor (role Write) dan kirim `docs/onboarding/CONTRIBUTOR_ONBOARDING.md`.
5. **Gelombang 1 paralel:** Kontributor memulai 0.2, 2.1–2.3, 3.1–3.2; Owner mengerjakan 0.3, 0.4, 1.1 dan menyusun kontrak C.1–C.6 (bersama Kontributor di C.3 dan C.4).
6. Setelah C.8 (kontrak v0 beku), mulai Gelombang 2.

**Hal yang masih terbuka:** (a) konfirmasi bahwa Kontributor memiliki mesin Windows untuk Track D; (b) keputusan menambah kontributor ketiga untuk Track W di akhir Gelombang 1.
