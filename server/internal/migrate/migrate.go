// Package migrate menjalankan migrasi per modul (ADR-0003, butir 4 sampai 6).
//
// Setiap modul memiliki skema PostgreSQL sendiri, dan tabel versi migrasinya
// berada di skema itu. Runner, bukan berkas migrasi modul, yang membuat dan
// menghapus skema: skema tidak boleh dihapus dari dalam migrasi down yang
// masih perlu mencatat versinya.
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"github.com/pressly/goose/v3"
)

// versionTable adalah nama tabel versi migrasi di dalam skema modul.
const versionTable = "goose_db_version"

// schemaName membatasi nama skema ke huruf kecil, angka, dan garis bawah,
// sehingga aman dipakai sebagai identifier yang dikutip.
var schemaName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// ErrInvalidSchema dikembalikan bila nama skema modul tidak valid atau dicadangkan.
var ErrInvalidSchema = errors.New("migrate: nama skema modul tidak valid")

// Spec menjelaskan migrasi satu modul.
type Spec struct {
	// Schema adalah nama skema PostgreSQL modul (sama dengan nama modul).
	Schema string
	// FS memuat direktori migrasi (mis. hasil go:embed).
	FS fs.FS
	// Dir adalah direktori migrasi di dalam FS; kosong berarti akar FS.
	Dir string
}

// Runner menjalankan migrasi modul pada satu *sql.DB. Runner tidak memiliki
// koneksi dan tidak menutupnya.
type Runner struct {
	db *sql.DB
}

// New membuat Runner untuk db.
func New(db *sql.DB) *Runner {
	return &Runner{db: db}
}

func (s Spec) validate() error {
	if !schemaName.MatchString(s.Schema) {
		return fmt.Errorf("%w: %q", ErrInvalidSchema, s.Schema)
	}
	if s.Schema == "public" || s.Schema == "information_schema" || strings.HasPrefix(s.Schema, "pg_") {
		return fmt.Errorf("%w: %q dicadangkan", ErrInvalidSchema, s.Schema)
	}
	if s.FS == nil {
		return errors.New("migrate: FS migrasi wajib diisi")
	}
	return nil
}

func quote(identifier string) string {
	// identifier sudah divalidasi oleh schemaName: hanya [a-z0-9_].
	return `"` + identifier + `"`
}

func (r *Runner) provider(s Spec) (*goose.Provider, error) {
	dir := s.Dir
	if dir == "" {
		dir = "."
	}
	sub, err := fs.Sub(s.FS, dir)
	if err != nil {
		return nil, fmt.Errorf("migrate: direktori migrasi %q: %w", dir, err)
	}
	p, err := goose.NewProvider(goose.DialectPostgres, r.db, sub,
		goose.WithTableName(s.Schema+"."+versionTable))
	if err != nil {
		return nil, fmt.Errorf("migrate: provider modul %q: %w", s.Schema, err)
	}
	return p, nil
}

func (r *Runner) schemaExists(ctx context.Context, name string) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)`, name).Scan(&ok)
	return ok, err
}

// Up membuat skema modul bila belum ada, lalu menerapkan semua migrasi yang
// belum berjalan. Aman dijalankan berulang.
func (r *Runner) Up(ctx context.Context, s Spec) error {
	if err := s.validate(); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS `+quote(s.Schema)); err != nil {
		return fmt.Errorf("migrate: buat skema %q: %w", s.Schema, err)
	}
	p, err := r.provider(s)
	if err != nil {
		return err
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("migrate: up modul %q: %w", s.Schema, err)
	}
	return nil
}

// Down membatalkan semua migrasi modul (urutan terbalik), lalu menghapus skema
// modul beserta tabel versinya. Bila skema tidak ada, Down tidak melakukan apa pun.
func (r *Runner) Down(ctx context.Context, s Spec) error {
	if err := s.validate(); err != nil {
		return err
	}
	exists, err := r.schemaExists(ctx, s.Schema)
	if err != nil {
		return fmt.Errorf("migrate: cek skema %q: %w", s.Schema, err)
	}
	if !exists {
		return nil
	}
	p, err := r.provider(s)
	if err != nil {
		return err
	}
	if _, err := p.DownTo(ctx, 0); err != nil {
		return fmt.Errorf("migrate: down modul %q: %w", s.Schema, err)
	}
	if _, err := r.db.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+quote(s.Schema)+` CASCADE`); err != nil {
		return fmt.Errorf("migrate: hapus skema %q: %w", s.Schema, err)
	}
	return nil
}
