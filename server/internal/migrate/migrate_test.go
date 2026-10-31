package migrate_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/master-abror/nizmeet/server/internal/migrate"
)

// fixture membuat migrasi contoh (dua langkah) untuk skema yang diberikan.
func fixture(schema string) migrate.Spec {
	fsys := fstest.MapFS{
		"migrations/00001_create_things.sql": {Data: []byte(
			"-- +goose Up\nCREATE TABLE " + schema + ".things (id int PRIMARY KEY);\n" +
				"-- +goose Down\nDROP TABLE " + schema + ".things;\n")},
		"migrations/00002_add_note.sql": {Data: []byte(
			"-- +goose Up\nALTER TABLE " + schema + ".things ADD COLUMN note text;\n" +
				"-- +goose Down\nALTER TABLE " + schema + ".things DROP COLUMN note;\n")},
	}
	return migrate.Spec{Schema: schema, FS: fsys, Dir: "migrations"}
}

func TestInvalidSchemaRejected(t *testing.T) {
	r := migrate.New(nil) // validasi terjadi sebelum db dipakai
	names := []string{
		"", "public", "Public", "pg_catalog", "information_schema",
		"a-b", "1abc", `a"; DROP SCHEMA public; --`, strings.Repeat("a", 64),
	}
	for _, name := range names {
		s := migrate.Spec{Schema: name, FS: fstest.MapFS{}}
		if err := r.Up(context.Background(), s); !errors.Is(err, migrate.ErrInvalidSchema) {
			t.Errorf("Up(%q): err = %v, ingin ErrInvalidSchema", name, err)
		}
		if err := r.Down(context.Background(), s); !errors.Is(err, migrate.ErrInvalidSchema) {
			t.Errorf("Down(%q): err = %v, ingin ErrInvalidSchema", name, err)
		}
	}
}

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("NIZMEET_TEST_DB_URL")
	if url == "" {
		t.Skip("NIZMEET_TEST_DB_URL belum diset; tes yang butuh PostgreSQL dilewati")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return db
}

// dropSchema hanya mau menghapus skema tes (awalan mt_).
func dropSchema(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	if !strings.HasPrefix(name, "mt_") {
		t.Fatalf("menolak menghapus skema %q (bukan skema tes)", name)
	}
	if _, err := db.Exec(`DROP SCHEMA IF EXISTS ` + name + ` CASCADE`); err != nil {
		t.Fatalf("drop %s: %v", name, err)
	}
}

func prepareSchema(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	dropSchema(t, db, name)
	t.Cleanup(func() { dropSchema(t, db, name) })
}

func queryBool(t *testing.T, db *sql.DB, q string, args ...any) bool {
	t.Helper()
	var ok bool
	if err := db.QueryRow(q, args...).Scan(&ok); err != nil {
		t.Fatalf("query: %v", err)
	}
	return ok
}

func schemaExists(t *testing.T, db *sql.DB, schema string) bool {
	return queryBool(t, db,
		`SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)`, schema)
}

func tableExists(t *testing.T, db *sql.DB, schema, table string) bool {
	return queryBool(t, db,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2)`,
		schema, table)
}

func columnExists(t *testing.T, db *sql.DB, schema, table, column string) bool {
	return queryBool(t, db,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 AND column_name = $3)`,
		schema, table, column)
}

func publicTableCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestUpDownLeavesNoTrace(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	const schema = "mt_trace"
	prepareSchema(t, db, schema)
	r := migrate.New(db)
	before := publicTableCount(t, db)

	if err := r.Up(ctx, fixture(schema)); err != nil {
		t.Fatalf("Up: %v", err)
	}
	if !tableExists(t, db, schema, "things") || !columnExists(t, db, schema, "things", "note") {
		t.Fatal("setelah Up: tabel things dan kolom note harus ada")
	}
	if !tableExists(t, db, schema, "goose_db_version") {
		t.Fatal("tabel versi migrasi harus berada di skema modul")
	}

	if err := r.Down(ctx, fixture(schema)); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if schemaExists(t, db, schema) {
		t.Fatal("setelah Down: skema modul harus hilang tanpa sisa")
	}
	if after := publicTableCount(t, db); after != before {
		t.Fatalf("tabel di public berubah: sebelum %d, sesudah %d", before, after)
	}
}

func TestUpIsIdempotent(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	const schema = "mt_idem"
	prepareSchema(t, db, schema)
	r := migrate.New(db)

	for i := 1; i <= 2; i++ {
		if err := r.Up(ctx, fixture(schema)); err != nil {
			t.Fatalf("Up ke-%d: %v", i, err)
		}
	}
	if !columnExists(t, db, schema, "things", "note") {
		t.Fatal("kolom note harus ada setelah Up berulang")
	}
}

func TestModulesAreIndependent(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	const a, b = "mt_a", "mt_b"
	prepareSchema(t, db, a)
	prepareSchema(t, db, b)
	r := migrate.New(db)

	if err := r.Up(ctx, fixture(a)); err != nil {
		t.Fatalf("Up %s: %v", a, err)
	}
	if err := r.Up(ctx, fixture(b)); err != nil {
		t.Fatalf("Up %s: %v", b, err)
	}
	if err := r.Down(ctx, fixture(a)); err != nil {
		t.Fatalf("Down %s: %v", a, err)
	}
	if schemaExists(t, db, a) {
		t.Fatalf("skema %s harus hilang", a)
	}
	if !tableExists(t, db, b, "things") || !tableExists(t, db, b, "goose_db_version") {
		t.Fatalf("modul %s harus tetap utuh setelah modul %s dihapus", b, a)
	}
}

func TestDownOnMissingSchemaIsNoop(t *testing.T) {
	db := openDB(t)
	const schema = "mt_missing"
	prepareSchema(t, db, schema)
	if err := migrate.New(db).Down(context.Background(), fixture(schema)); err != nil {
		t.Fatalf("Down pada skema yang tidak ada harus nihil: %v", err)
	}
}
