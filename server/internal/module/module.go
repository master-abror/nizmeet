// Package module mendefinisikan kontrak modul server dan registry modul
// (ADR-0003, butir 1 dan 2). Paket ini tidak mengimpor net/http maupun router:
// adaptor ke chi dibuat di Milestone 1.1.
package module

import (
	"context"
	"errors"
	"fmt"

	"github.com/master-abror/nizmeet/server/internal/extension"
	"github.com/master-abror/nizmeet/server/internal/migrate"
)

var (
	// ErrInvalidModule dikembalikan bila modul tidak memenuhi kontrak.
	ErrInvalidModule = errors.New("module: modul tidak valid")
	// ErrDuplicateModule dikembalikan bila nama modul sudah terdaftar.
	ErrDuplicateModule = errors.New("module: nama modul sudah terdaftar")
	// ErrDuplicateRoute dikembalikan bila dua modul mendaftarkan rute yang sama.
	ErrDuplicateRoute = errors.New("module: rute ganda")
	// ErrUnknownModule dikembalikan bila nama modul tidak dikenal.
	ErrUnknownModule = errors.New("module: modul tidak dikenal")
)

// Request adalah permintaan abstrak, tidak bergantung pada net/http.
type Request struct {
	Method string
	Path   string
	Query  map[string][]string
	Body   []byte
}

// Response adalah jawaban abstrak.
type Response struct {
	Status int
	Body   []byte
}

// Handler menangani satu rute.
type Handler func(ctx context.Context, req Request) (Response, error)

// Route adalah satu rute milik modul.
type Route struct {
	Method  string
	Path    string
	Handler Handler
}

// Job adalah satu pekerjaan latar milik modul. Penjadwalan menjadi urusan
// milestone yang memilikinya.
type Job struct {
	Name string
	Run  func(ctx context.Context) error
}

// Module adalah kontrak yang dipenuhi setiap modul server: route, migrasi,
// job, dan titik masuk registry (tipe dokumen, kind sync, LLMTask, fitur).
type Module interface {
	// Name adalah nama modul; bila modul punya migrasi, sama dengan nama skemanya.
	Name() string
	// Migrations mengembalikan migrasi modul, atau nil bila modul tanpa skema.
	Migrations() *migrate.Spec
	Routes() []Route
	Jobs() []Job
	// Register mendaftarkan entri milik modul ke registry titik ekstensi.
	Register(regs *extension.Registries) error
}

// Registry menyimpan modul sesuai urutan registrasi. Tidak aman untuk akses
// konkuren: dipakai saat startup.
type Registry struct {
	mods  []Module
	index map[string]Module
}

// NewRegistry membuat registry modul kosong.
func NewRegistry() *Registry {
	return &Registry{index: make(map[string]Module)}
}

// Add mendaftarkan modul setelah memeriksa kontraknya.
func (r *Registry) Add(m Module) error {
	if m == nil {
		return fmt.Errorf("%w: modul nil", ErrInvalidModule)
	}
	name := m.Name()
	if name == "" {
		return fmt.Errorf("%w: nama kosong", ErrInvalidModule)
	}
	if _, dup := r.index[name]; dup {
		return fmt.Errorf("%w: %q", ErrDuplicateModule, name)
	}
	if spec := m.Migrations(); spec != nil && spec.Schema != name {
		return fmt.Errorf("%w: skema %q harus sama dengan nama modul %q", ErrInvalidModule, spec.Schema, name)
	}
	for _, rt := range m.Routes() {
		if rt.Method == "" || rt.Path == "" || rt.Handler == nil {
			return fmt.Errorf("%w: rute modul %q tidak lengkap", ErrInvalidModule, name)
		}
	}
	r.mods = append(r.mods, m)
	r.index[name] = m
	return nil
}

// Modules mengembalikan salinan daftar modul sesuai urutan registrasi.
func (r *Registry) Modules() []Module {
	out := make([]Module, len(r.mods))
	copy(out, r.mods)
	return out
}

// Get mengambil modul berdasarkan nama.
func (r *Registry) Get(name string) (Module, bool) {
	m, ok := r.index[name]
	return m, ok
}

// Apply memanggil Register pada setiap modul sesuai urutan registrasi.
func (r *Registry) Apply(regs *extension.Registries) error {
	for _, m := range r.mods {
		if err := m.Register(regs); err != nil {
			return fmt.Errorf("module: registrasi modul %q: %w", m.Name(), err)
		}
	}
	return nil
}

// Routes mengumpulkan rute semua modul dan menolak rute ganda.
func (r *Registry) Routes() ([]Route, error) {
	owner := make(map[string]string)
	var out []Route
	for _, m := range r.mods {
		for _, rt := range m.Routes() {
			key := rt.Method + " " + rt.Path
			if prev, dup := owner[key]; dup {
				return nil, fmt.Errorf("%w: %s (modul %q dan %q)", ErrDuplicateRoute, key, prev, m.Name())
			}
			owner[key] = m.Name()
			out = append(out, rt)
		}
	}
	return out, nil
}

// MigrateUp menjalankan migrasi semua modul yang punya skema.
func (r *Registry) MigrateUp(ctx context.Context, runner *migrate.Runner) error {
	for _, m := range r.mods {
		spec := m.Migrations()
		if spec == nil {
			continue
		}
		if err := runner.Up(ctx, *spec); err != nil {
			return err
		}
	}
	return nil
}

// MigrateDown membatalkan semua migrasi satu modul dan menghapus skemanya.
// Modul tanpa migrasi tidak melakukan apa pun.
func (r *Registry) MigrateDown(ctx context.Context, runner *migrate.Runner, name string) error {
	m, ok := r.index[name]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownModule, name)
	}
	spec := m.Migrations()
	if spec == nil {
		return nil
	}
	return runner.Down(ctx, *spec)
}
