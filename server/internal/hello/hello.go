// Package hello adalah modul contoh Milestone 0.4: bukti Kontrak Ekstensibilitas.
// Modul ini hanya perlu folder ini ditambah satu entri di internal/modules.
// Rute di bawah hanyalah contoh dan bukan bagian kontrak OpenAPI v1.
package hello

import (
	"context"
	"embed"

	"github.com/master-abror/nizmeet/server/internal/extension"
	"github.com/master-abror/nizmeet/server/internal/migrate"
	"github.com/master-abror/nizmeet/server/internal/module"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Module adalah modul contoh hello.
type Module struct{}

// New membuat modul hello.
func New() *Module {
	return &Module{}
}

// Name mengembalikan nama modul, sama dengan nama skemanya.
func (*Module) Name() string {
	return "hello"
}

// Migrations mengembalikan migrasi modul (skema hello).
func (*Module) Migrations() *migrate.Spec {
	return &migrate.Spec{
		Schema: "hello",
		FS:     migrationsFS,
		Dir:    "migrations",
	}
}

// Routes mengembalikan satu rute contoh.
func (*Module) Routes() []module.Route {
	return []module.Route{
		{Method: "GET", Path: "/v1/hello", Handler: greet},
	}
}

// Jobs mengembalikan nil: modul contoh tanpa job.
func (*Module) Jobs() []module.Job {
	return nil
}

// Register mendaftarkan kunci fitur hello ke registry entitlement.
func (*Module) Register(regs *extension.Registries) error {
	return regs.AddFeature(extension.Feature{Key: "hello"})
}

func greet(_ context.Context, _ module.Request) (module.Response, error) {
	return module.Response{Status: 200, Body: []byte(`{"message":"hello"}`)}, nil
}
