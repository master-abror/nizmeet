-- =============================================================================
-- docs/contracts/db/smoke.sql
-- Smoke test perilaku skema C.1 (trigger revisi, tombstone, FK komposit, cascade).
-- Alat bantu validasi; bukan bagian kontrak. Seluruhnya di-ROLLBACK.
--
-- Jalankan pada DB yang SUDAH berisi skema dari ddl.sql:
--   psql -P pager=off -v ON_ERROR_STOP=1 "$DB_URL/ac_c1" -f smoke.sql
--
-- Hasil yang diharapkan:
--   * semua kolom ok bernilai t (T1a, T1b, T1c, T3a, T4a, T4b, T4c, T5)
--   * tiga NOTICE: T2 OK, T2b OK, T6 OK
--   * baris terakhir: ROLLBACK, tanpa ERROR
-- =============================================================================
BEGIN;
INSERT INTO users (email,password_hash) VALUES ('a@x.test','h') RETURNING id AS ua \gset
INSERT INTO users (email,password_hash) VALUES ('b@x.test','h') RETURNING id AS ub \gset

-- T1: revisi monoton, soft delete -> tombstone, restore -> tombstone hilang
INSERT INTO documents (user_id,type,title) VALUES (:'ua','cv','CV 1') RETURNING id AS d1, revision AS r1 \gset
INSERT INTO documents (user_id,type,title) VALUES (:'ua','jd','JD 1') RETURNING id AS d2, revision AS r2 \gset
SELECT 'T1a revisi naik' AS tes, (:r2 > :r1) AS ok;
UPDATE documents SET deleted_at = now() WHERE id = :'d1';
SELECT 'T1b tombstone cocok revisi dokumen' AS tes, count(*) = 1 AS ok
  FROM tombstones t JOIN documents d ON d.id = t.resource_id
 WHERE t.resource_id = :'d1' AND t.kind = 'documents' AND t.revision = d.revision AND d.revision > :r2;
UPDATE documents SET deleted_at = NULL WHERE id = :'d1';
SELECT 'T1c restore hapus tombstone' AS tes, count(*) = 0 AS ok FROM tombstones WHERE resource_id = :'d1';

-- T2: FK komposit menolak referensi lintas-user
INSERT INTO documents (user_id,type,title) VALUES (:'ub','cv','Dok B') RETURNING id AS db1 \gset
SELECT set_config('t.ua', :'ua', true) AS x1, set_config('t.db1', :'db1', true) AS x2 \gset
DO $$
BEGIN
  INSERT INTO document_versions (user_id,document_id,version_no,doc_revision,source)
  VALUES (current_setting('t.ua')::uuid, current_setting('t.db1')::uuid, 1, 1, 'upload');
  RAISE EXCEPTION 'SEHARUSNYA GAGAL';
EXCEPTION WHEN foreign_key_violation THEN
  RAISE NOTICE 'T2 OK: FK komposit menolak lintas-user';
END
$$;

-- T3: FTS + tag global + alias
INSERT INTO document_versions (user_id,document_id,version_no,doc_revision,body,source)
VALUES (:'ua',:'d1',1,1,'teks','upload') RETURNING id AS v1 \gset
INSERT INTO document_chunks (user_id,document_id,version_id,doc_revision,ordinal,content)
VALUES (:'ua',:'d1',:'v1',1,0,'Pengalaman membangun layanan Golang dan PostgreSQL') RETURNING id AS c1 \gset
SELECT 'T3a FTS menemukan chunk' AS tes, count(*) = 1 AS ok
  FROM document_chunks WHERE user_id = :'ua' AND fts @@ plainto_tsquery('simple','golang');
INSERT INTO tags (slug,label) VALUES ('golang','Go') RETURNING id AS tg \gset
INSERT INTO tag_aliases (tag_id,alias) VALUES (:'tg','go');
INSERT INTO chunk_tags (chunk_id,tag_id,user_id) VALUES (:'c1',:'tg',:'ua');
INSERT INTO tags (user_id,slug,label) VALUES (:'ub','rahasia','R') RETURNING id AS tb \gset
SELECT set_config('t.c1', :'c1', true) AS x3, set_config('t.tb', :'tb', true) AS x4, set_config('t.ua2', :'ua', true) AS x5 \gset
DO $$
BEGIN
  INSERT INTO chunk_tags (chunk_id,tag_id,user_id)
  VALUES (current_setting('t.c1')::uuid, current_setting('t.tb')::uuid, current_setting('t.ua2')::uuid);
  RAISE EXCEPTION 'SEHARUSNYA GAGAL';
EXCEPTION WHEN check_violation THEN
  RAISE NOTICE 'T2b OK: tag milik user lain ditolak';
END
$$;

-- T4: session profile, live session, segmen, tombstone
INSERT INTO session_profiles (user_id,name,style,answer_length,translation_lang,mode)
VALUES (:'ua','Interview Backend','formal','singkat','id','interview') RETURNING id AS sp \gset
INSERT INTO live_sessions (user_id,session_profile_id,profile_snapshot)
VALUES (:'ua',:'sp','{"mode":"interview"}') RETURNING id AS ls, revision AS lrev \gset
INSERT INTO transcript_segments (user_id,session_id,seq,speaker,content)
VALUES (:'ua',:'ls',0,'interlocutor','Ceritakan pengalaman Anda');
DELETE FROM session_profiles WHERE id = :'sp';
SELECT 'T4a profil dihapus -> FK null + revisi sesi naik' AS tes,
       (session_profile_id IS NULL AND revision > :lrev) AS ok FROM live_sessions WHERE id = :'ls';
SELECT 'T4b tombstone session_profiles' AS tes, count(*) = 1 AS ok
  FROM tombstones WHERE kind = 'session_profiles' AND resource_id = :'sp';
DELETE FROM live_sessions WHERE id = :'ls';
SELECT 'T4c tombstone live_sessions + segmen ikut terhapus' AS tes,
       (SELECT count(*) FROM tombstones WHERE kind = 'live_sessions' AND resource_id = :'ls') = 1
   AND (SELECT count(*) FROM transcript_segments WHERE session_id = :'ls') = 0 AS ok;

-- Data untuk uji cascade (termasuk jalur SET NULL + trigger saat hapus akun)
INSERT INTO session_profiles (user_id,name,style,answer_length,translation_lang,mode)
VALUES (:'ua','Meeting','santai','detail','id','meeting') RETURNING id AS sp2 \gset
INSERT INTO live_sessions (user_id,session_profile_id,profile_snapshot)
VALUES (:'ua',:'sp2','{}') RETURNING id AS ls2 \gset
INSERT INTO transcript_segments (user_id,session_id,seq,speaker,content) VALUES (:'ua',:'ls2',0,'user','halo');

-- T5: hapus akun membersihkan semuanya tanpa error trigger
DELETE FROM users WHERE id = :'ua';
SELECT 'T5 hapus akun bersih' AS tes,
  ( (SELECT count(*) FROM documents WHERE user_id = :'ua')
  + (SELECT count(*) FROM document_versions WHERE user_id = :'ua')
  + (SELECT count(*) FROM document_chunks WHERE user_id = :'ua')
  + (SELECT count(*) FROM chunk_tags WHERE user_id = :'ua')
  + (SELECT count(*) FROM session_profiles WHERE user_id = :'ua')
  + (SELECT count(*) FROM live_sessions WHERE user_id = :'ua')
  + (SELECT count(*) FROM transcript_segments WHERE user_id = :'ua')
  + (SELECT count(*) FROM tombstones WHERE user_id = :'ua')
  + (SELECT count(*) FROM sync_counters WHERE user_id = :'ua') ) = 0 AS ok;

-- T6: audit_log append-only
INSERT INTO audit_log (actor_kind,action) VALUES ('system','smoke.test');
DO $$
BEGIN
  UPDATE audit_log SET action = 'x';
  RAISE EXCEPTION 'SEHARUSNYA GAGAL';
EXCEPTION WHEN insufficient_privilege THEN
  RAISE NOTICE 'T6 OK: audit_log menolak UPDATE';
END
$$;

ROLLBACK;
