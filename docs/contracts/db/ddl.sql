-- =============================================================================
-- docs/contracts/db/ddl.sql
-- Kontrak skema PostgreSQL 18 - Milestone C.1 (DRAF; dibekukan di C.8)
--
-- Jalankan pada DB KOSONG, sebagai pemilik database:
--   psql -v ON_ERROR_STOP=1 -d <db> -f docs/contracts/db/ddl.sql
--
-- Seluruh berkas berada dalam satu transaksi: bila ada error, tidak ada yang
-- tersisa. Tidak memakai ekstensi (uuidv7() adalah fungsi bawaan PostgreSQL 18).
--
-- KONVENSI (ringkas; versi lengkap ada di docs/contracts/erd.md):
--   * ID           : uuid, DEFAULT uuidv7().
--   * Nama         : snake_case; tabel jamak; FK <entitas>_id.
--                    Indeks ix_<tabel>__<kolom>, unik ux_..., check ck_..., FK fk_...
--   * Waktu        : timestamptz; created_at / updated_at.
--   * Himpunan nilai tertutup : text + CHECK (bukan tipe ENUM).
--                    Registry (tipe dokumen, preset) tidak diberi CHECK nilai.
--   * Isolasi      : tabel milik pengguna punya user_id NOT NULL -> users(id)
--                    ON DELETE CASCADE, dan indeks berawalan user_id. Relasi
--                    antar tabel milik pengguna memakai FK KOMPOSIT
--                    (user_id, <induk>_id) -> induk(user_id, id), sehingga
--                    referensi lintas-user mustahil di tingkat DB.
--   * Sync         : kolom revision (diisi trigger dari sync_counters), tabel
--                    tombstones, cursor per device per kind (ADR-0002).
--   * Hapus akun   : DELETE FROM users menghapus seluruh data pengguna (CASCADE).
-- =============================================================================

BEGIN;

-- Namespace modul contoh "hello" (Milestone 0.4). Rollback modul:
--   DROP SCHEMA hello CASCADE;   -- tidak meninggalkan sisa.
CREATE SCHEMA hello;
COMMENT ON SCHEMA hello IS 'Modul contoh hello (Milestone 0.4). Dapat dihapus utuh dengan DROP SCHEMA hello CASCADE.';

-- -----------------------------------------------------------------------------
-- Fungsi trigger umum
-- -----------------------------------------------------------------------------

-- Mengisi updated_at pada tabel yang tidak memakai assign_revision().
CREATE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END
$$;

-- =============================================================================
-- DOMAIN: AKUN, SESI AUTH, PERANGKAT
-- =============================================================================

-- -----------------------------------------------------------------------------
-- users
-- Tujuan : akun pelanggan. Akar kepemilikan seluruh data pengguna.
-- Menulis: server (modul auth: register, login, verifikasi email, nonaktifkan).
-- Membaca: server (auth, licensing, admin); portal lewat API.
-- Catatan: tabel ini sendiri tidak punya user_id (id = identitas pengguna).
--          DELETE FROM users = hapus akun beserta seluruh datanya (CASCADE).
-- -----------------------------------------------------------------------------
CREATE TABLE users (
    id                uuid        NOT NULL DEFAULT uuidv7(),
    email             text        NOT NULL,
    password_hash     text        NOT NULL,                -- argon2id; tidak pernah di-log
    email_verified_at timestamptz,
    disabled_at       timestamptz,                         -- NULL = aktif; terisi = akses dicabut
    last_login_at     timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_users PRIMARY KEY (id),
    CONSTRAINT ck_users__email_not_empty CHECK (email <> '')
);
CREATE UNIQUE INDEX ux_users__email_lower ON users (lower(email));
CREATE TRIGGER trg_users__updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
COMMENT ON TABLE users IS 'Akun pelanggan; akar kepemilikan data. Ditulis modul auth, dibaca auth/licensing/admin.';

-- -----------------------------------------------------------------------------
-- sync_counters
-- Tujuan : sumber revisi monoton per pengguna (ADR-0002 butir 2). Semua resource
--          sync (documents, session_profiles, live_sessions) dan tombstones
--          mengambil revision dari sini lewat next_revision().
-- Menulis: HANYA fungsi next_revision() (dipanggil trigger). Aplikasi tidak
--          menulis langsung.
-- Membaca: server (diagnostik); klien tidak membacanya.
-- Catatan: kunci baris ditahan sampai commit, sehingga urutan commit = urutan
--          revisi per pengguna dan cursor klien tidak pernah melewatkan revisi.
-- -----------------------------------------------------------------------------
CREATE TABLE sync_counters (
    user_id       uuid   NOT NULL,
    last_revision bigint NOT NULL DEFAULT 0,
    CONSTRAINT pk_sync_counters PRIMARY KEY (user_id),
    CONSTRAINT fk_sync_counters__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT ck_sync_counters__revision_nonneg CHECK (last_revision >= 0)
);
COMMENT ON TABLE sync_counters IS 'Counter revisi monoton per pengguna. Hanya ditulis next_revision().';

-- Mengambil revisi berikutnya untuk satu pengguna (atomik, serial per pengguna).
CREATE FUNCTION next_revision(p_user_id uuid) RETURNS bigint
LANGUAGE sql AS $$
    INSERT INTO sync_counters (user_id, last_revision)
    VALUES (p_user_id, 1)
    ON CONFLICT (user_id) DO UPDATE
        SET last_revision = sync_counters.last_revision + 1
    RETURNING last_revision
$$;

-- Trigger untuk tabel resource sync: isi revision + updated_at pada INSERT/UPDATE.
-- Dilewati bila pengguna sudah tidak ada (terjadi saat cascade hapus akun).
CREATE FUNCTION assign_revision() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.user_id) THEN
        RETURN NEW;
    END IF;
    NEW.revision   := next_revision(NEW.user_id);
    NEW.updated_at := now();
    RETURN NEW;
END
$$;

-- -----------------------------------------------------------------------------
-- plans
-- Tujuan : katalog paket (batas device, fitur/entitlement, kuota, harga).
-- Menulis: admin/seed migrasi. Membaca: licensing, billing, client-config.
-- Catatan: tabel GLOBAL (tanpa user_id).
-- -----------------------------------------------------------------------------
CREATE TABLE plans (
    id              uuid        NOT NULL DEFAULT uuidv7(),
    code            text        NOT NULL,
    name            text        NOT NULL,
    device_limit    integer     NOT NULL,
    features        jsonb       NOT NULL DEFAULT '[]'::jsonb,  -- features[] pada token lisensi
    quotas          jsonb       NOT NULL DEFAULT '{}'::jsonb,  -- mis. menit audio, token
    price_minor     bigint      NOT NULL,
    currency        char(3)     NOT NULL,
    interval_months smallint    NOT NULL,
    is_active       boolean     NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_plans PRIMARY KEY (id),
    CONSTRAINT ux_plans__code UNIQUE (code),
    CONSTRAINT ck_plans__device_limit CHECK (device_limit > 0),
    CONSTRAINT ck_plans__features_array CHECK (jsonb_typeof(features) = 'array'),
    CONSTRAINT ck_plans__quotas_object CHECK (jsonb_typeof(quotas) = 'object'),
    CONSTRAINT ck_plans__price CHECK (price_minor >= 0),
    CONSTRAINT ck_plans__interval CHECK (interval_months > 0)
);
CREATE TRIGGER trg_plans__updated_at BEFORE UPDATE ON plans
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
COMMENT ON TABLE plans IS 'Katalog paket (global). Ditulis admin/seed, dibaca licensing/billing/client-config.';

-- -----------------------------------------------------------------------------
-- subscriptions
-- Tujuan : langganan pengguna pada satu plan.
-- Menulis: server (billing, webhook pembayaran). Membaca: billing, licensing, portal.
-- -----------------------------------------------------------------------------
CREATE TABLE subscriptions (
    id                       uuid        NOT NULL DEFAULT uuidv7(),
    user_id                  uuid        NOT NULL,
    plan_id                  uuid        NOT NULL,
    status                   text        NOT NULL DEFAULT 'pending',
    current_period_start     timestamptz,
    current_period_end       timestamptz,
    cancel_at_period_end     boolean     NOT NULL DEFAULT false,
    canceled_at              timestamptz,
    provider                 text        NOT NULL,
    provider_subscription_id text,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_subscriptions PRIMARY KEY (id),
    CONSTRAINT fk_subscriptions__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_subscriptions__plan FOREIGN KEY (plan_id) REFERENCES plans (id),
    -- Indeks unik ini juga menjadi indeks user_id (kolom pertama) dan target FK komposit.
    CONSTRAINT ux_subscriptions__user_id_id UNIQUE (user_id, id),
    CONSTRAINT ck_subscriptions__status CHECK (status IN ('pending', 'active', 'canceled', 'expired'))
);
CREATE UNIQUE INDEX ux_subscriptions__provider_ref
    ON subscriptions (provider, provider_subscription_id)
    WHERE provider_subscription_id IS NOT NULL;
CREATE INDEX ix_subscriptions__plan_id ON subscriptions (plan_id);
CREATE TRIGGER trg_subscriptions__updated_at BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
COMMENT ON TABLE subscriptions IS 'Langganan pengguna pada satu plan. Ditulis billing/webhook, dibaca billing/licensing/portal.';

-- -----------------------------------------------------------------------------
-- licenses
-- Tujuan : hak pakai aplikasi (plan, masa berlaku); sumber token lisensi Ed25519.
-- Menulis: server (licensing: terbit/perpanjang/cabut; webhook pembayaran).
-- Membaca: licensing (aktivasi, heartbeat), admin, portal.
-- -----------------------------------------------------------------------------
CREATE TABLE licenses (
    id              uuid        NOT NULL DEFAULT uuidv7(),
    user_id         uuid        NOT NULL,
    subscription_id uuid,
    plan_id         uuid        NOT NULL,
    status          text        NOT NULL DEFAULT 'active',
    valid_from      timestamptz NOT NULL,
    valid_until     timestamptz NOT NULL,
    revoked_at      timestamptz,
    revoked_reason  text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_licenses PRIMARY KEY (id),
    CONSTRAINT fk_licenses__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_licenses__subscription FOREIGN KEY (user_id, subscription_id)
        REFERENCES subscriptions (user_id, id) ON DELETE SET NULL (subscription_id),
    CONSTRAINT fk_licenses__plan FOREIGN KEY (plan_id) REFERENCES plans (id),
    CONSTRAINT ux_licenses__user_id_id UNIQUE (user_id, id),
    CONSTRAINT ck_licenses__status CHECK (status IN ('active', 'revoked', 'expired')),
    CONSTRAINT ck_licenses__validity CHECK (valid_until > valid_from)
);
CREATE INDEX ix_licenses__user_id_valid_until ON licenses (user_id, valid_until);
CREATE INDEX ix_licenses__subscription ON licenses (user_id, subscription_id);
CREATE INDEX ix_licenses__plan_id ON licenses (plan_id);
CREATE TRIGGER trg_licenses__updated_at BEFORE UPDATE ON licenses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
COMMENT ON TABLE licenses IS 'Lisensi pengguna (plan + masa berlaku). Ditulis licensing/billing, dibaca licensing/admin/portal.';

-- -----------------------------------------------------------------------------
-- devices
-- Tujuan : perangkat yang diaktifkan pada sebuah lisensi (fingerprint ter-hash).
--          Juga pemilik cursor sync.
-- Menulis: server (licensing: aktivasi, heartbeat, revoke).
-- Membaca: licensing, sync, admin, portal (daftar perangkat).
-- Catatan: batas device = COUNT baris revoked_at IS NULL per license_id
--          dibandingkan plans.device_limit (dihitung di aplikasi).
-- -----------------------------------------------------------------------------
CREATE TABLE devices (
    id                uuid        NOT NULL DEFAULT uuidv7(),
    user_id           uuid        NOT NULL,
    license_id        uuid        NOT NULL,
    fingerprint_hash  text        NOT NULL,
    name              text,
    platform          text        NOT NULL,
    app_version       text,
    activated_at      timestamptz NOT NULL DEFAULT now(),
    last_heartbeat_at timestamptz,
    revoked_at        timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_devices PRIMARY KEY (id),
    CONSTRAINT fk_devices__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_devices__license FOREIGN KEY (user_id, license_id)
        REFERENCES licenses (user_id, id) ON DELETE CASCADE,
    CONSTRAINT ux_devices__user_id_id UNIQUE (user_id, id)
);
-- Satu fingerprint aktif per lisensi; perangkat yang dicabut boleh diaktifkan ulang (baris baru).
CREATE UNIQUE INDEX ux_devices__license_fingerprint_active
    ON devices (license_id, fingerprint_hash) WHERE revoked_at IS NULL;
CREATE INDEX ix_devices__license_active ON devices (license_id) WHERE revoked_at IS NULL;
CREATE TRIGGER trg_devices__updated_at BEFORE UPDATE ON devices
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
COMMENT ON TABLE devices IS 'Perangkat teraktivasi per lisensi. Ditulis licensing, dibaca licensing/sync/admin/portal.';

-- -----------------------------------------------------------------------------
-- auth_sessions
-- Tujuan : sesi autentikasi (refresh token berotasi, disimpan sebagai hash).
--          BUKAN sesi live meeting (lihat live_sessions).
-- Menulis: server (auth: login, refresh/rotasi, logout, cabut).
-- Membaca: auth (validasi refresh, deteksi reuse), admin.
-- Catatan: reuse terdeteksi bila token yang disodorkan cocok dengan
--          prev_refresh_hash -> seluruh sesi dicabut (revoked_at diisi).
-- -----------------------------------------------------------------------------
CREATE TABLE auth_sessions (
    id                uuid        NOT NULL DEFAULT uuidv7(),
    user_id           uuid        NOT NULL,
    device_id         uuid,                                 -- NULL untuk sesi portal
    refresh_hash      text        NOT NULL,
    prev_refresh_hash text,
    rotated_at        timestamptz,
    expires_at        timestamptz NOT NULL,
    revoked_at        timestamptz,
    revoke_reason     text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_auth_sessions PRIMARY KEY (id),
    CONSTRAINT fk_auth_sessions__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_auth_sessions__device FOREIGN KEY (user_id, device_id)
        REFERENCES devices (user_id, id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX ux_auth_sessions__refresh_hash ON auth_sessions (refresh_hash);
CREATE INDEX ix_auth_sessions__prev_refresh_hash ON auth_sessions (prev_refresh_hash)
    WHERE prev_refresh_hash IS NOT NULL;
CREATE INDEX ix_auth_sessions__user_id ON auth_sessions (user_id);
CREATE INDEX ix_auth_sessions__expires_at ON auth_sessions (expires_at);   -- job retensi
COMMENT ON TABLE auth_sessions IS 'Sesi autentikasi (refresh token berotasi, hash). Ditulis auth, dibaca auth/admin.';

-- -----------------------------------------------------------------------------
-- one_time_tokens
-- Tujuan : token sekali pakai untuk reset password dan verifikasi email (hash).
-- Menulis: server (auth). Membaca: auth.
-- -----------------------------------------------------------------------------
CREATE TABLE one_time_tokens (
    id         uuid        NOT NULL DEFAULT uuidv7(),
    user_id    uuid        NOT NULL,
    purpose    text        NOT NULL,
    token_hash text        NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_one_time_tokens PRIMARY KEY (id),
    CONSTRAINT fk_one_time_tokens__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT ux_one_time_tokens__token_hash UNIQUE (token_hash),
    CONSTRAINT ck_one_time_tokens__purpose CHECK (purpose IN ('password_reset', 'email_verify'))
);
CREATE INDEX ix_one_time_tokens__user_id ON one_time_tokens (user_id, purpose);
CREATE INDEX ix_one_time_tokens__expires_at ON one_time_tokens (expires_at);   -- job retensi
COMMENT ON TABLE one_time_tokens IS 'Token sekali pakai reset password / verifikasi email (hash). Ditulis dan dibaca modul auth.';

-- =============================================================================
-- DOMAIN: PEMBAYARAN & METERING
-- =============================================================================

-- -----------------------------------------------------------------------------
-- payments
-- Tujuan : catatan pembayaran per langganan (bayar, gagal, refund).
-- Menulis: server (billing, handler webhook). Membaca: billing, portal (invoice), admin.
-- -----------------------------------------------------------------------------
CREATE TABLE payments (
    id                  uuid        NOT NULL DEFAULT uuidv7(),
    user_id             uuid        NOT NULL,
    subscription_id     uuid,
    provider            text        NOT NULL,
    provider_payment_id text        NOT NULL,
    amount_minor        bigint      NOT NULL,
    currency            char(3)     NOT NULL,
    status              text        NOT NULL DEFAULT 'pending',
    paid_at             timestamptz,
    refunded_at         timestamptz,
    provider_payload    jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_payments PRIMARY KEY (id),
    CONSTRAINT fk_payments__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_payments__subscription FOREIGN KEY (user_id, subscription_id)
        REFERENCES subscriptions (user_id, id) ON DELETE SET NULL (subscription_id),
    CONSTRAINT ux_payments__provider_ref UNIQUE (provider, provider_payment_id),
    CONSTRAINT ck_payments__amount CHECK (amount_minor >= 0),
    CONSTRAINT ck_payments__status CHECK (status IN ('pending', 'paid', 'failed', 'refunded'))
);
CREATE INDEX ix_payments__user_id_created_at ON payments (user_id, created_at DESC);
CREATE INDEX ix_payments__subscription ON payments (user_id, subscription_id);
CREATE TRIGGER trg_payments__updated_at BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
COMMENT ON TABLE payments IS 'Pembayaran per langganan. Ditulis billing/webhook, dibaca billing/portal/admin.';

-- -----------------------------------------------------------------------------
-- payment_webhook_events
-- Tujuan : kunci idempotensi webhook gateway (webhook ganda tidak diproses dua kali).
-- Menulis: server (handler webhook, setelah verifikasi signature).
-- Membaca: billing (dedup, audit).
-- Catatan: user_id terisi setelah event dipetakan ke pengguna (nullable).
-- -----------------------------------------------------------------------------
CREATE TABLE payment_webhook_events (
    id           uuid        NOT NULL DEFAULT uuidv7(),
    user_id      uuid,
    provider     text        NOT NULL,
    event_id     text        NOT NULL,
    payload      jsonb       NOT NULL,
    received_at  timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    CONSTRAINT pk_payment_webhook_events PRIMARY KEY (id),
    CONSTRAINT fk_payment_webhook_events__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT ux_payment_webhook_events__provider_event UNIQUE (provider, event_id)
);
CREATE INDEX ix_payment_webhook_events__user_id ON payment_webhook_events (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX ix_payment_webhook_events__received_at ON payment_webhook_events (received_at);   -- job retensi
COMMENT ON TABLE payment_webhook_events IS 'Idempotensi webhook gateway. Ditulis handler webhook, dibaca billing.';

-- -----------------------------------------------------------------------------
-- usage_events
-- Tujuan : metering generik (menit audio, token, dst.) untuk kuota per plan.
--          Kind baru tidak mengubah skema (Kontrak Ekstensibilitas).
-- Menulis: server (provider gateway, Live Session Service). Membaca: kuota, portal, admin.
-- -----------------------------------------------------------------------------
CREATE TABLE usage_events (
    id          uuid        NOT NULL DEFAULT uuidv7(),
    user_id     uuid        NOT NULL,
    device_id   uuid,
    kind        text        NOT NULL,
    units       numeric     NOT NULL,
    meta        jsonb       NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_usage_events PRIMARY KEY (id),
    CONSTRAINT fk_usage_events__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_usage_events__device FOREIGN KEY (user_id, device_id)
        REFERENCES devices (user_id, id) ON DELETE SET NULL (device_id),
    CONSTRAINT ck_usage_events__kind CHECK (kind <> ''),
    CONSTRAINT ck_usage_events__units CHECK (units >= 0)
);
CREATE INDEX ix_usage_events__user_kind_time ON usage_events (user_id, kind, occurred_at);
CREATE INDEX ix_usage_events__occurred_at ON usage_events (occurred_at);   -- job retensi
COMMENT ON TABLE usage_events IS 'Metering generik (kind, units, meta). Ditulis provider gateway/live session, dibaca kuota/portal/admin.';

-- =============================================================================
-- DOMAIN: PROFIL, DOKUMEN, VERSI, CHUNK, TAG
-- =============================================================================

-- -----------------------------------------------------------------------------
-- profiles
-- Tujuan : profil karier pengguna (satu per pengguna).
-- Menulis: server (API profil; portal). Membaca: server, portal.
-- -----------------------------------------------------------------------------
CREATE TABLE profiles (
    id             uuid        NOT NULL DEFAULT uuidv7(),
    user_id        uuid        NOT NULL,
    headline       text,
    summary        text,
    skills         jsonb       NOT NULL DEFAULT '[]'::jsonb,
    languages      jsonb       NOT NULL DEFAULT '[]'::jsonb,   -- bahasa yang dikuasai pengguna
    schema_version integer     NOT NULL DEFAULT 1,
    payload        jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_profiles PRIMARY KEY (id),
    CONSTRAINT fk_profiles__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT ux_profiles__user_id UNIQUE (user_id),          -- sekaligus indeks user_id
    CONSTRAINT ck_profiles__skills_array CHECK (jsonb_typeof(skills) = 'array'),
    CONSTRAINT ck_profiles__languages_array CHECK (jsonb_typeof(languages) = 'array')
);
CREATE TRIGGER trg_profiles__updated_at BEFORE UPDATE ON profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
COMMENT ON TABLE profiles IS 'Profil karier pengguna (1:1). Ditulis API profil, dibaca server/portal.';

-- -----------------------------------------------------------------------------
-- tombstones
-- Tujuan : catatan penghapusan untuk feed sync (kind + resource_id + revision).
-- Menulis: trigger (documents: soft delete; session_profiles/live_sessions: DELETE).
-- Membaca: sync (GET /sync/changes).
-- Catatan: tidak punya FK ke resource (resource boleh sudah di-purge).
--          Restore dokumen menghapus tombstone-nya.
-- -----------------------------------------------------------------------------
CREATE TABLE tombstones (
    id          uuid        NOT NULL DEFAULT uuidv7(),
    user_id     uuid        NOT NULL,
    kind        text        NOT NULL,
    resource_id uuid        NOT NULL,
    revision    bigint      NOT NULL,
    deleted_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_tombstones PRIMARY KEY (id),
    CONSTRAINT fk_tombstones__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT ux_tombstones__resource UNIQUE (user_id, kind, resource_id),
    CONSTRAINT ck_tombstones__kind CHECK (kind <> '')
);
CREATE INDEX ix_tombstones__user_kind_revision ON tombstones (user_id, kind, revision);
CREATE INDEX ix_tombstones__deleted_at ON tombstones (deleted_at);   -- job retensi
COMMENT ON TABLE tombstones IS 'Penghapusan untuk feed sync. Ditulis trigger, dibaca sync API.';

-- Tombstone untuk resource yang DI-DELETE (session_profiles, live_sessions).
-- Argumen trigger: nama kind. Dilewati saat cascade hapus akun.
CREATE FUNCTION record_tombstone() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM users WHERE id = OLD.user_id) THEN
        INSERT INTO tombstones (user_id, kind, resource_id, revision, deleted_at)
        VALUES (OLD.user_id, TG_ARGV[0], OLD.id, next_revision(OLD.user_id), now())
        ON CONFLICT (user_id, kind, resource_id) DO UPDATE
            SET revision = EXCLUDED.revision, deleted_at = EXCLUDED.deleted_at;
    END IF;
    RETURN NULL;
END
$$;

-- -----------------------------------------------------------------------------
-- documents
-- Tujuan : dokumen pengguna (cv, jd, note, portfolio, presentation, dst.).
--          Resource sync kind "documents". File asli ada di object storage
--          (blob_key); teks di document_versions/document_chunks.
-- Menulis: server (documents API, parser job). Membaca: server, sync, retriever, portal.
-- Catatan: type = string dari registry (bukan enum DB). Hapus = soft delete
--          (deleted_at) agar dapat dipulihkan; trigger membuat tombstone.
--          Baris hanya di-DELETE oleh job purge retensi atau cascade hapus akun.
-- -----------------------------------------------------------------------------
CREATE TABLE documents (
    id                 uuid        NOT NULL DEFAULT uuidv7(),
    user_id            uuid        NOT NULL,
    type               text        NOT NULL,
    title              text        NOT NULL,
    status             text        NOT NULL DEFAULT 'pending',
    lang               text,                                -- terdeteksi saat parse
    schema_version     integer     NOT NULL DEFAULT 1,
    revision           bigint      NOT NULL DEFAULT 0,      -- diisi trigger assign_revision
    current_version_id uuid,
    blob_key           text,
    mime_type          text,
    size_bytes         bigint,
    payload            jsonb       NOT NULL DEFAULT '{}'::jsonb,
    deleted_at         timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_documents PRIMARY KEY (id),
    CONSTRAINT fk_documents__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT ux_documents__user_id_id UNIQUE (user_id, id),
    CONSTRAINT ck_documents__type CHECK (type <> ''),
    CONSTRAINT ck_documents__status CHECK (status IN ('pending', 'processing', 'ready', 'failed')),
    CONSTRAINT ck_documents__size CHECK (size_bytes IS NULL OR size_bytes >= 0)
);
CREATE INDEX ix_documents__user_id_revision ON documents (user_id, revision);
CREATE INDEX ix_documents__user_type_live ON documents (user_id, type) WHERE deleted_at IS NULL;
CREATE INDEX ix_documents__pending_parse ON documents (created_at) WHERE status IN ('pending', 'processing');
CREATE INDEX ix_documents__deleted_at ON documents (deleted_at) WHERE deleted_at IS NOT NULL;   -- job purge
CREATE TRIGGER trg_documents__revision BEFORE INSERT OR UPDATE ON documents
    FOR EACH ROW EXECUTE FUNCTION assign_revision();

-- Soft delete / restore -> tombstone di feed sync.
CREATE FUNCTION documents_sync_tombstone() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.deleted_at IS NOT NULL THEN
        INSERT INTO tombstones (user_id, kind, resource_id, revision, deleted_at)
        VALUES (NEW.user_id, 'documents', NEW.id, NEW.revision, NEW.deleted_at)
        ON CONFLICT (user_id, kind, resource_id) DO UPDATE
            SET revision = EXCLUDED.revision, deleted_at = EXCLUDED.deleted_at;
    ELSE
        DELETE FROM tombstones
         WHERE user_id = NEW.user_id AND kind = 'documents' AND resource_id = NEW.id;
    END IF;
    RETURN NULL;
END
$$;
CREATE TRIGGER trg_documents__tombstone AFTER UPDATE OF deleted_at ON documents
    FOR EACH ROW WHEN (OLD.deleted_at IS DISTINCT FROM NEW.deleted_at)
    EXECUTE FUNCTION documents_sync_tombstone();
COMMENT ON TABLE documents IS 'Dokumen pengguna; resource sync kind documents. Ditulis documents API/parser, dibaca server/sync/retriever/portal.';

-- -----------------------------------------------------------------------------
-- document_versions
-- Tujuan : riwayat versi dokumen (tiap edit/unggah/restore membuat versi baru);
--          versi lama dapat dipulihkan (konflik: server menang, versi lama tetap).
-- Menulis: server (documents API). Membaca: server, portal (riwayat versi + pulihkan).
-- Catatan: doc_revision = documents.revision pada saat versi dibuat (diisi aplikasi).
-- -----------------------------------------------------------------------------
CREATE TABLE document_versions (
    id                   uuid        NOT NULL DEFAULT uuidv7(),
    user_id              uuid        NOT NULL,
    document_id          uuid        NOT NULL,
    version_no           integer     NOT NULL,
    doc_revision         bigint      NOT NULL,
    body                 text,                              -- teks hasil parse / hasil edit
    blob_key             text,                              -- file asli versi ini (bila ada)
    source               text        NOT NULL,
    created_by_device_id uuid,
    created_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_document_versions PRIMARY KEY (id),
    CONSTRAINT fk_document_versions__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_document_versions__document FOREIGN KEY (user_id, document_id)
        REFERENCES documents (user_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_document_versions__device FOREIGN KEY (user_id, created_by_device_id)
        REFERENCES devices (user_id, id) ON DELETE SET NULL (created_by_device_id),
    CONSTRAINT ux_document_versions__user_id_id UNIQUE (user_id, id),
    CONSTRAINT ux_document_versions__document_no UNIQUE (document_id, version_no),
    CONSTRAINT ck_document_versions__no CHECK (version_no > 0),
    CONSTRAINT ck_document_versions__source CHECK (source IN ('upload', 'edit', 'restore'))
);
CREATE INDEX ix_document_versions__user_document ON document_versions (user_id, document_id, version_no DESC);

-- Penunjuk versi aktif (ditambahkan setelah document_versions ada).
ALTER TABLE documents
    ADD CONSTRAINT fk_documents__current_version FOREIGN KEY (user_id, current_version_id)
        REFERENCES document_versions (user_id, id) ON DELETE SET NULL (current_version_id);
COMMENT ON TABLE document_versions IS 'Riwayat versi dokumen (dapat dipulihkan). Ditulis documents API, dibaca server/portal.';

-- -----------------------------------------------------------------------------
-- document_chunks
-- Tujuan : potongan teks versi aktif (dibuat SERVER; konsisten di semua device)
--          + indeks FTS untuk Retriever (PgRetriever, Milestone 4.2).
-- Menulis: server (parser/chunker; re-chunk mengganti seluruh chunk dokumen).
-- Membaca: retriever (FTS), sync (dikirim ke desktop bersama dokumen), portal.
-- Catatan: fts_config dipilih dari documents.lang oleh aplikasi; default 'simple'
--          sampai konfigurasi Indonesia/Inggris diverifikasi di 4.2 (OPEN QUESTION).
--          tags = metadata mentah hasil ekstraksi (skill, perusahaan, peran,
--          periode); tag kanonik ada di chunk_tags.
-- -----------------------------------------------------------------------------
CREATE TABLE document_chunks (
    id           uuid        NOT NULL DEFAULT uuidv7(),
    user_id      uuid        NOT NULL,
    document_id  uuid        NOT NULL,
    version_id   uuid        NOT NULL,
    doc_revision bigint      NOT NULL,
    ordinal      integer     NOT NULL,
    content      text        NOT NULL,
    token_count  integer,
    tags         jsonb       NOT NULL DEFAULT '{}'::jsonb,
    fts_config   regconfig   NOT NULL DEFAULT 'simple'::regconfig,
    fts          tsvector    GENERATED ALWAYS AS (to_tsvector(fts_config, content)) STORED,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_document_chunks PRIMARY KEY (id),
    CONSTRAINT fk_document_chunks__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_document_chunks__document FOREIGN KEY (user_id, document_id)
        REFERENCES documents (user_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_document_chunks__version FOREIGN KEY (user_id, version_id)
        REFERENCES document_versions (user_id, id) ON DELETE CASCADE,
    CONSTRAINT ux_document_chunks__user_id_id UNIQUE (user_id, id),
    CONSTRAINT ux_document_chunks__version_ordinal UNIQUE (version_id, ordinal),
    CONSTRAINT ck_document_chunks__ordinal CHECK (ordinal >= 0)
);
CREATE INDEX ix_document_chunks__user_document ON document_chunks (user_id, document_id, ordinal);
CREATE INDEX ix_document_chunks__fts ON document_chunks USING gin (fts);
COMMENT ON TABLE document_chunks IS 'Chunk teks + indeks FTS. Ditulis chunker server, dibaca retriever/sync/portal.';

-- -----------------------------------------------------------------------------
-- tags
-- Tujuan : registri tag kanonik. user_id NULL = tag global (registri bersama,
--          mis. golang); user_id terisi = tag buatan pengguna.
-- Menulis: server (seed/admin untuk global; ekstraksi/API untuk milik pengguna).
-- Membaca: retriever (boost/filter), session profile, portal, desktop (via sync chunk).
-- Catatan: satu-satunya tabel "milik pengguna" yang boleh punya baris tanpa user_id.
-- -----------------------------------------------------------------------------
CREATE TABLE tags (
    id         uuid        NOT NULL DEFAULT uuidv7(),
    user_id    uuid,
    slug       text        NOT NULL,
    label      text        NOT NULL,
    kind       text        NOT NULL DEFAULT 'skill',       -- skill, company, role, period, ...
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_tags PRIMARY KEY (id),
    CONSTRAINT fk_tags__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT ck_tags__slug CHECK (slug <> '' AND slug = lower(slug))
);
CREATE UNIQUE INDEX ux_tags__global_slug ON tags (slug) WHERE user_id IS NULL;
CREATE UNIQUE INDEX ux_tags__user_slug ON tags (user_id, slug) WHERE user_id IS NOT NULL;
CREATE TRIGGER trg_tags__updated_at BEFORE UPDATE ON tags
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
COMMENT ON TABLE tags IS 'Registri tag kanonik (global bila user_id NULL). Ditulis seed/ekstraksi/API, dibaca retriever/profil.';

-- -----------------------------------------------------------------------------
-- tag_aliases
-- Tujuan : normalisasi alias -> tag kanonik (mis. go = golang), lintas bahasa.
-- Menulis: server (seed/admin; API). Membaca: ekstraksi tag, retriever, Fast-Pass.
-- Catatan: user_id harus sama dengan tags.user_id (dijaga trigger).
-- -----------------------------------------------------------------------------
CREATE TABLE tag_aliases (
    id         uuid        NOT NULL DEFAULT uuidv7(),
    tag_id     uuid        NOT NULL,
    user_id    uuid,
    alias      text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_tag_aliases PRIMARY KEY (id),
    CONSTRAINT fk_tag_aliases__tag FOREIGN KEY (tag_id) REFERENCES tags (id) ON DELETE CASCADE,
    CONSTRAINT fk_tag_aliases__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT ck_tag_aliases__alias CHECK (alias <> '' AND alias = lower(alias))
);
CREATE UNIQUE INDEX ux_tag_aliases__global_alias ON tag_aliases (alias) WHERE user_id IS NULL;
CREATE UNIQUE INDEX ux_tag_aliases__user_alias ON tag_aliases (user_id, alias) WHERE user_id IS NOT NULL;
CREATE INDEX ix_tag_aliases__tag_id ON tag_aliases (tag_id);

CREATE FUNCTION check_tag_alias_scope() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    v_tag_user uuid;
BEGIN
    SELECT user_id INTO v_tag_user FROM tags WHERE id = NEW.tag_id;
    IF v_tag_user IS DISTINCT FROM NEW.user_id THEN
        RAISE EXCEPTION 'tag_aliases.user_id (%) harus sama dengan tags.user_id (%)', NEW.user_id, v_tag_user
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END
$$;
CREATE TRIGGER trg_tag_aliases__scope BEFORE INSERT OR UPDATE ON tag_aliases
    FOR EACH ROW EXECUTE FUNCTION check_tag_alias_scope();
COMMENT ON TABLE tag_aliases IS 'Alias tag -> tag kanonik. Ditulis seed/API, dibaca ekstraksi/retriever/Fast-Pass.';

-- -----------------------------------------------------------------------------
-- chunk_tags
-- Tujuan : tag kanonik per chunk (hasil ekstraksi); dasar boost/filter retrieval.
-- Menulis: server (chunker/tagger). Membaca: retriever, sync (ke desktop chunk_tags).
-- Catatan: tag harus global atau milik user yang sama (dijaga trigger).
-- -----------------------------------------------------------------------------
CREATE TABLE chunk_tags (
    chunk_id uuid NOT NULL,
    tag_id   uuid NOT NULL,
    user_id  uuid NOT NULL,
    CONSTRAINT pk_chunk_tags PRIMARY KEY (chunk_id, tag_id),
    CONSTRAINT fk_chunk_tags__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_chunk_tags__chunk FOREIGN KEY (user_id, chunk_id)
        REFERENCES document_chunks (user_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_chunk_tags__tag FOREIGN KEY (tag_id) REFERENCES tags (id) ON DELETE CASCADE
);
CREATE INDEX ix_chunk_tags__user_tag ON chunk_tags (user_id, tag_id);
CREATE INDEX ix_chunk_tags__tag_id ON chunk_tags (tag_id);

CREATE FUNCTION check_chunk_tag_scope() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    v_tag_user uuid;
BEGIN
    SELECT user_id INTO v_tag_user FROM tags WHERE id = NEW.tag_id;
    IF v_tag_user IS NOT NULL AND v_tag_user <> NEW.user_id THEN
        RAISE EXCEPTION 'chunk_tags: tag % milik pengguna lain', NEW.tag_id
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END
$$;
CREATE TRIGGER trg_chunk_tags__scope BEFORE INSERT OR UPDATE ON chunk_tags
    FOR EACH ROW EXECUTE FUNCTION check_chunk_tag_scope();
COMMENT ON TABLE chunk_tags IS 'Tag kanonik per chunk. Ditulis chunker/tagger, dibaca retriever/sync.';

-- =============================================================================
-- DOMAIN: SESSION PROFILE & PRESET
-- =============================================================================

-- -----------------------------------------------------------------------------
-- presets
-- Tujuan : registri preset bawaan (peran, gaya bahasa, dst.) sebagai DATA, bukan
--          kode. Preset baru = baris baru.
-- Menulis: seed/admin. Membaca: server (client-config, session setup), desktop/portal via API.
-- Catatan: tabel GLOBAL (tanpa user_id). Preset buatan pengguna = session_profiles.
-- -----------------------------------------------------------------------------
CREATE TABLE presets (
    id             uuid        NOT NULL DEFAULT uuidv7(),
    kind           text        NOT NULL,                    -- role, style, ...
    code           text        NOT NULL,                    -- mis. formal, informal, santai, slang
    label          jsonb       NOT NULL DEFAULT '{}'::jsonb, -- label per bahasa UI
    payload        jsonb       NOT NULL DEFAULT '{}'::jsonb,
    sort_order     integer     NOT NULL DEFAULT 0,
    is_active      boolean     NOT NULL DEFAULT true,
    schema_version integer     NOT NULL DEFAULT 1,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_presets PRIMARY KEY (id),
    CONSTRAINT ux_presets__kind_code UNIQUE (kind, code),
    CONSTRAINT ck_presets__kind CHECK (kind <> ''),
    CONSTRAINT ck_presets__code CHECK (code <> '')
);
CREATE TRIGGER trg_presets__updated_at BEFORE UPDATE ON presets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
COMMENT ON TABLE presets IS 'Registri preset bawaan (peran, gaya) sebagai data. Ditulis seed/admin, dibaca server/klien.';

-- -----------------------------------------------------------------------------
-- session_profiles
-- Tujuan : Session Profile tersimpan sebagai preset bernama milik pengguna.
--          Resource sync kind "session_profiles".
-- Menulis: server (presets API; mutasi desktop/portal). Membaca: server, sync,
--          Live Session Service (dikirim saat session.start).
-- Catatan: style = presets.code (kind='style'), tanpa FK karena kind konstan.
--          answer_lang NULL = ikuti bahasa lawan bicara.
--          active_document_ids tanpa FK (satu baris, mudah di-sync); ID basi
--          ditoleransi aplikasi. Hapus = DELETE + tombstone.
-- -----------------------------------------------------------------------------
CREATE TABLE session_profiles (
    id                  uuid        NOT NULL DEFAULT uuidv7(),
    user_id             uuid        NOT NULL,
    name                text        NOT NULL,
    role_preset_id      uuid,
    role_text           text,                                 -- peran teks bebas
    style               text        NOT NULL,
    answer_length       text        NOT NULL,
    interlocutor_lang   text        NOT NULL DEFAULT 'auto',  -- 'auto' atau satu bahasa tetap
    answer_lang         text,
    translation_lang    text        NOT NULL,
    focus_tags          text[]      NOT NULL DEFAULT '{}',    -- slug tag kanonik
    active_document_ids uuid[]      NOT NULL DEFAULT '{}',
    mode                text        NOT NULL,
    schema_version      integer     NOT NULL DEFAULT 1,
    revision            bigint      NOT NULL DEFAULT 0,       -- diisi trigger assign_revision
    payload             jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_session_profiles PRIMARY KEY (id),
    CONSTRAINT fk_session_profiles__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_session_profiles__role_preset FOREIGN KEY (role_preset_id) REFERENCES presets (id) ON DELETE SET NULL,
    CONSTRAINT ux_session_profiles__user_id_id UNIQUE (user_id, id),
    CONSTRAINT ck_session_profiles__name CHECK (name <> ''),
    CONSTRAINT ck_session_profiles__answer_length CHECK (answer_length IN ('singkat', 'detail')),
    CONSTRAINT ck_session_profiles__mode CHECK (mode IN ('interview', 'meeting'))
);
CREATE UNIQUE INDEX ux_session_profiles__user_name ON session_profiles (user_id, lower(name));
CREATE INDEX ix_session_profiles__user_id_revision ON session_profiles (user_id, revision);
CREATE INDEX ix_session_profiles__role_preset ON session_profiles (role_preset_id) WHERE role_preset_id IS NOT NULL;
CREATE TRIGGER trg_session_profiles__revision BEFORE INSERT OR UPDATE ON session_profiles
    FOR EACH ROW EXECUTE FUNCTION assign_revision();
CREATE TRIGGER trg_session_profiles__tombstone AFTER DELETE ON session_profiles
    FOR EACH ROW EXECUTE FUNCTION record_tombstone('session_profiles');
COMMENT ON TABLE session_profiles IS 'Session Profile / preset bernama pengguna; sync kind session_profiles. Ditulis presets API, dibaca server/sync/live session.';

-- =============================================================================
-- DOMAIN: RIWAYAT SESI LIVE (keputusan Owner: transkrip disimpan di cloud)
-- =============================================================================

-- -----------------------------------------------------------------------------
-- live_sessions
-- Tujuan : riwayat sesi live (meeting/interview). Resource sync kind "live_sessions";
--          desktop menyimpan salinan baca ("riwayat sesi").
-- Menulis: server (Live Session Service: mulai, selesai). Membaca: server, sync, portal.
-- Catatan: mode "tanpa simpan" (6.2) = TIDAK ada baris di tabel ini maupun
--          transcript_segments. Audio mentah tidak pernah disimpan.
--          profile_snapshot = Session Profile efektif saat sesi dimulai.
--          revision naik saat sesi dimulai, berakhir, atau diubah (bukan per segmen).
--          Hapus = DELETE + tombstone; segmen ikut terhapus (CASCADE).
-- -----------------------------------------------------------------------------
CREATE TABLE live_sessions (
    id                 uuid        NOT NULL DEFAULT uuidv7(),
    user_id            uuid        NOT NULL,
    device_id          uuid,
    session_profile_id uuid,
    profile_snapshot   jsonb       NOT NULL,
    status             text        NOT NULL DEFAULT 'active',
    started_at         timestamptz NOT NULL DEFAULT now(),
    ended_at           timestamptz,
    schema_version     integer     NOT NULL DEFAULT 1,
    revision           bigint      NOT NULL DEFAULT 0,        -- diisi trigger assign_revision
    payload            jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_live_sessions PRIMARY KEY (id),
    CONSTRAINT fk_live_sessions__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_live_sessions__device FOREIGN KEY (user_id, device_id)
        REFERENCES devices (user_id, id) ON DELETE SET NULL (device_id),
    CONSTRAINT fk_live_sessions__profile FOREIGN KEY (user_id, session_profile_id)
        REFERENCES session_profiles (user_id, id) ON DELETE SET NULL (session_profile_id),
    CONSTRAINT ux_live_sessions__user_id_id UNIQUE (user_id, id),
    CONSTRAINT ck_live_sessions__status CHECK (status IN ('active', 'ended')),
    CONSTRAINT ck_live_sessions__period CHECK (ended_at IS NULL OR ended_at >= started_at)
);
CREATE INDEX ix_live_sessions__user_id_revision ON live_sessions (user_id, revision);
CREATE INDEX ix_live_sessions__user_started ON live_sessions (user_id, started_at DESC);
CREATE TRIGGER trg_live_sessions__revision BEFORE INSERT OR UPDATE ON live_sessions
    FOR EACH ROW EXECUTE FUNCTION assign_revision();
CREATE TRIGGER trg_live_sessions__tombstone AFTER DELETE ON live_sessions
    FOR EACH ROW EXECUTE FUNCTION record_tombstone('live_sessions');
COMMENT ON TABLE live_sessions IS 'Riwayat sesi live; sync kind live_sessions. Ditulis Live Session Service, dibaca server/sync/portal.';

-- -----------------------------------------------------------------------------
-- transcript_segments
-- Tujuan : transkrip FINAL per ucapan dalam satu sesi live (bahasa asli +
--          terjemahan ucapan). Transkrip interim tidak disimpan.
-- Menulis: server (Live Session Service, append-only selama sesi).
-- Membaca: server, sync (diambil per sesi, dipaginasi per seq), portal.
-- Catatan: speaker = penanda pembicara dari mixer audio (mis. 'user',
--          'interlocutor'); himpunan nilai final ditetapkan di protokol WS (C.3).
-- -----------------------------------------------------------------------------
CREATE TABLE transcript_segments (
    id               uuid        NOT NULL DEFAULT uuidv7(),
    user_id          uuid        NOT NULL,
    session_id       uuid        NOT NULL,
    seq              integer     NOT NULL,
    speaker          text        NOT NULL,
    lang             text,                                    -- bahasa ucapan terdeteksi
    content          text        NOT NULL,
    translation      text,
    translation_lang text,
    start_ms         integer,
    end_ms           integer,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_transcript_segments PRIMARY KEY (id),
    CONSTRAINT fk_transcript_segments__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_transcript_segments__session FOREIGN KEY (user_id, session_id)
        REFERENCES live_sessions (user_id, id) ON DELETE CASCADE,
    CONSTRAINT ux_transcript_segments__session_seq UNIQUE (session_id, seq),
    CONSTRAINT ck_transcript_segments__seq CHECK (seq >= 0),
    CONSTRAINT ck_transcript_segments__speaker CHECK (speaker <> ''),
    CONSTRAINT ck_transcript_segments__offsets CHECK (
        (start_ms IS NULL OR start_ms >= 0) AND (end_ms IS NULL OR start_ms IS NULL OR end_ms >= start_ms))
);
CREATE INDEX ix_transcript_segments__user_session ON transcript_segments (user_id, session_id, seq);
COMMENT ON TABLE transcript_segments IS 'Transkrip final per ucapan. Ditulis Live Session Service, dibaca server/sync/portal.';

-- =============================================================================
-- DOMAIN: SYNC
-- =============================================================================

-- -----------------------------------------------------------------------------
-- sync_cursors
-- Tujuan : cursor per device per kind (revisi terakhir yang di-ack device).
-- Menulis: server (POST /sync/ack). Membaca: sync API, admin (diagnostik).
-- -----------------------------------------------------------------------------
CREATE TABLE sync_cursors (
    user_id        uuid        NOT NULL,
    device_id      uuid        NOT NULL,
    kind           text        NOT NULL,
    acked_revision bigint      NOT NULL DEFAULT 0,
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_sync_cursors PRIMARY KEY (device_id, kind),
    CONSTRAINT fk_sync_cursors__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_sync_cursors__device FOREIGN KEY (user_id, device_id)
        REFERENCES devices (user_id, id) ON DELETE CASCADE,
    CONSTRAINT ck_sync_cursors__kind CHECK (kind <> ''),
    CONSTRAINT ck_sync_cursors__revision CHECK (acked_revision >= 0)
);
CREATE INDEX ix_sync_cursors__user_id ON sync_cursors (user_id);
CREATE TRIGGER trg_sync_cursors__updated_at BEFORE UPDATE ON sync_cursors
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
COMMENT ON TABLE sync_cursors IS 'Cursor sync per device per kind. Ditulis sync API (ack), dibaca sync API/admin.';

-- -----------------------------------------------------------------------------
-- sync_mutations
-- Tujuan : ledger idempotensi mutasi klien (client_mutation_id): mutasi yang
--          sama dikirim ulang tidak menggandakan data (ADR-0002 butir 4).
-- Menulis: server (sync API, dalam transaksi yang sama dengan mutasi). Membaca: sync API.
-- -----------------------------------------------------------------------------
CREATE TABLE sync_mutations (
    user_id            uuid        NOT NULL,
    client_mutation_id uuid        NOT NULL,
    device_id          uuid,
    kind               text        NOT NULL,
    resource_id        uuid,
    applied_revision   bigint,
    result             text        NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_sync_mutations PRIMARY KEY (user_id, client_mutation_id),
    CONSTRAINT fk_sync_mutations__user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_sync_mutations__device FOREIGN KEY (user_id, device_id)
        REFERENCES devices (user_id, id) ON DELETE SET NULL (device_id),
    CONSTRAINT ck_sync_mutations__kind CHECK (kind <> ''),
    CONSTRAINT ck_sync_mutations__result CHECK (result IN ('applied', 'conflict'))
);
CREATE INDEX ix_sync_mutations__created_at ON sync_mutations (created_at);   -- job retensi
COMMENT ON TABLE sync_mutations IS 'Ledger idempotensi client_mutation_id. Ditulis dan dibaca sync API.';

-- =============================================================================
-- DOMAIN: AUDIT
-- =============================================================================

-- -----------------------------------------------------------------------------
-- audit_log
-- Tujuan : jejak aksi sensitif (terutama aksi admin; juga aksi sistem/pengguna).
-- Menulis: server (admin endpoints, job). Membaca: admin panel.
-- Catatan: APPEND-ONLY (UPDATE ditolak trigger; DELETE hanya oleh job retensi).
--          user_id = pengguna yang menjadi SUBJEK aksi (nullable); tanpa FK agar
--          jejak tidak ikut terhapus bersama akun (OPEN QUESTION di erd.md).
--          actor_id tanpa FK karena model identitas admin belum diputuskan.
-- -----------------------------------------------------------------------------
CREATE TABLE audit_log (
    id          uuid        NOT NULL DEFAULT uuidv7(),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    actor_kind  text        NOT NULL,
    actor_id    uuid,
    action      text        NOT NULL,
    target_kind text,
    target_id   uuid,
    user_id     uuid,
    request_id  text,
    meta        jsonb       NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT pk_audit_log PRIMARY KEY (id),
    CONSTRAINT ck_audit_log__actor_kind CHECK (actor_kind IN ('user', 'admin', 'system')),
    CONSTRAINT ck_audit_log__action CHECK (action <> '')
);
CREATE INDEX ix_audit_log__user_id_time ON audit_log (user_id, occurred_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX ix_audit_log__actor_time ON audit_log (actor_id, occurred_at DESC) WHERE actor_id IS NOT NULL;
CREATE INDEX ix_audit_log__occurred_at ON audit_log (occurred_at);   -- job retensi

CREATE FUNCTION audit_log_block_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_log bersifat append-only (UPDATE ditolak)' USING ERRCODE = '42501';
END
$$;
CREATE TRIGGER trg_audit_log__block_update BEFORE UPDATE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_block_update();
COMMENT ON TABLE audit_log IS 'Jejak audit append-only. Ditulis admin endpoints/job, dibaca admin panel.';

-- =============================================================================
-- MODUL CONTOH: hello (Milestone 0.4) - schema "hello"
-- =============================================================================

-- -----------------------------------------------------------------------------
-- hello.greetings
-- Tujuan : bukti Kontrak Ekstensibilitas: modul baru menambah tabel sendiri di
--          namespace sendiri tanpa mengubah skema inti.
-- Menulis: modul server internal/hello. Membaca: modul hello, route group portal hello.
-- -----------------------------------------------------------------------------
CREATE TABLE hello.greetings (
    id         uuid        NOT NULL DEFAULT uuidv7(),
    user_id    uuid        NOT NULL,
    message    text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_greetings PRIMARY KEY (id),
    CONSTRAINT fk_greetings__user FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_greetings__message CHECK (message <> '')
);
CREATE INDEX ix_greetings__user_id_created_at ON hello.greetings (user_id, created_at DESC);
CREATE TRIGGER trg_greetings__updated_at BEFORE UPDATE ON hello.greetings
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
COMMENT ON TABLE hello.greetings IS 'Tabel modul contoh hello. Ditulis dan dibaca modul hello.';

COMMIT;
