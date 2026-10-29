package hello_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/master-abror/nizmeet/server/internal/extension"
	"github.com/master-abror/nizmeet/server/internal/hello"
	"github.com/master-abror/nizmeet/server/internal/migrate"
	"github.com/master-abror/nizmeet/server/internal/module"
)

func TestMetadata(t *testing.T) {
	m := hello.New()
	if m.Name() != "hello" {
		t.Fatalf("Name = %q", m.Name())
	}
	spec := m.Migrations()
	if spec == nil || spec.Schema != "hello" {
		t.Fatalf("Migrations = %+v", spec)
	}
	regs := extension.NewRegistries()
	if err := m.Register(regs); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, ok := regs.Features.Get("hello"); !ok {
		t.Fatal("fitur hello harus terdaftar")
	}
}

func TestRoute(t *testing.T) {
	routes := hello.New().Routes()
	if len(routes) != 1 {
		t.Fatalf("jumlah rute = %d, ingin 1", len(routes))
	}
	rt := routes[0]
	resp, err := rt.Handler(context.Background(), module.Request{Method: rt.Method, Path: rt.Path})
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	if resp.Status != 200 || !strings.Contains(string(resp.Body), "hello") {
		t.Fatalf("respons = %d %s", resp.Status, resp.Body)
	}
}

func queryBool(t *testing.T, db *sql.DB, q string, args ...any) bool {
	t.Helper()
	var ok bool
	if err := db.QueryRow(q, args...).Scan(&ok); err != nil {
		t.Fatalf("query: %v", err)
	}
	return ok
}

// TestMigrationLifecycle memakai database sementara supaya database dev tidak
// tersentuh. public.users dan public.set_updated_at() hanya pengganti minimal
// untuk objek inti yang baru dibuat di Milestone 1.2.
func TestMigrationLifecycle(t *testing.T) {
	baseURL := os.Getenv("NIZMEET_TEST_DB_URL")
	if baseURL == "" {
		t.Skip("NIZMEET_TEST_DB_URL belum diset; tes yang butuh PostgreSQL dilewati")
	}
	ctx := context.Background()

	admin, err := sql.Open("pgx", baseURL)
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close() })

	name := fmt.Sprintf("mt_hello_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
	})

	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	u.Path = "/" + name
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, stmt := range []string{
		`CREATE TABLE public.users (id uuid PRIMARY KEY DEFAULT uuidv7())`,
		`CREATE FUNCTION public.set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.updated_at := now(); RETURN NEW; END $$`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("setup %q: %v", stmt, err)
		}
	}

	runner := migrate.New(db)
	spec := hello.New().Migrations()
	if err := runner.Up(ctx, *spec); err != nil {
		t.Fatalf("Up: %v", err)
	}
	if !queryBool(t, db,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'hello' AND table_name = 'greetings')`) {
		t.Fatal("hello.greetings harus ada setelah Up")
	}

	var userID string
	if err := db.QueryRowContext(ctx, `INSERT INTO public.users DEFAULT VALUES RETURNING id::text`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO hello.greetings (user_id, message) VALUES ($1::uuid, $2)`, userID, "halo"); err != nil {
		t.Fatalf("insert greeting: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO hello.greetings (user_id, message) VALUES ($1::uuid, '')`, userID); err == nil {
		t.Fatal("pesan kosong harus ditolak oleh CHECK")
	}

	if err := runner.Down(ctx, *spec); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if queryBool(t, db, `SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = 'hello')`) {
		t.Fatal("skema hello harus hilang tanpa sisa setelah Down")
	}
	if !queryBool(t, db,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'users')`) {
		t.Fatal("tabel inti public.users harus tetap utuh")
	}
}
