-- +goose Up
-- Identik dengan hello.greetings di docs/contracts/db/ddl.sql.
-- Skema hello dibuat dan dihapus oleh runner migrasi (ADR-0003, butir 5).
COMMENT ON SCHEMA hello IS 'Modul contoh hello (Milestone 0.4). Dapat dihapus utuh dengan DROP SCHEMA hello CASCADE.';

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

-- +goose Down
DROP TABLE hello.greetings;
COMMENT ON SCHEMA hello IS NULL;
