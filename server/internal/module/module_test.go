package module

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/master-abror/nizmeet/server/internal/extension"
	"github.com/master-abror/nizmeet/server/internal/migrate"
)

type fake struct {
	name    string
	spec    *migrate.Spec
	routes  []Route
	feature string
	regErr  error
}

func (f fake) Name() string              { return f.name }
func (f fake) Migrations() *migrate.Spec { return f.spec }
func (f fake) Routes() []Route           { return f.routes }
func (f fake) Jobs() []Job               { return nil }
func (f fake) Register(regs *extension.Registries) error {
	if f.regErr != nil {
		return f.regErr
	}
	if f.feature != "" {
		return regs.AddFeature(extension.Feature{Key: f.feature})
	}
	return nil
}

func okHandler(_ context.Context, _ Request) (Response, error) {
	return Response{Status: 200}, nil
}

func TestAddRejectsInvalid(t *testing.T) {
	r := NewRegistry()
	if err := r.Add(nil); !errors.Is(err, ErrInvalidModule) {
		t.Fatalf("modul nil: err = %v", err)
	}
	if err := r.Add(fake{name: ""}); !errors.Is(err, ErrInvalidModule) {
		t.Fatalf("nama kosong: err = %v", err)
	}
	if err := r.Add(fake{name: "a", spec: &migrate.Spec{Schema: "lain"}}); !errors.Is(err, ErrInvalidModule) {
		t.Fatalf("skema beda dari nama: err = %v", err)
	}
	if err := r.Add(fake{name: "b", routes: []Route{{Method: "GET", Path: "/x"}}}); !errors.Is(err, ErrInvalidModule) {
		t.Fatalf("rute tanpa handler: err = %v", err)
	}
	if err := r.Add(fake{name: "c"}); err != nil {
		t.Fatalf("modul valid: %v", err)
	}
	if err := r.Add(fake{name: "c"}); !errors.Is(err, ErrDuplicateModule) {
		t.Fatalf("nama ganda: err = %v", err)
	}
}

func TestModulesKeepOrderAndReturnCopy(t *testing.T) {
	r := NewRegistry()
	for _, n := range []string{"z", "a", "m"} {
		if err := r.Add(fake{name: n}); err != nil {
			t.Fatalf("Add(%s): %v", n, err)
		}
	}
	var got []string
	for _, m := range r.Modules() {
		got = append(got, m.Name())
	}
	if want := []string{"z", "a", "m"}; !slices.Equal(got, want) {
		t.Fatalf("urutan = %v, ingin %v", got, want)
	}
	mods := r.Modules()
	mods[0] = nil
	if r.Modules()[0] == nil {
		t.Fatal("Modules harus mengembalikan salinan")
	}
	if _, ok := r.Get("a"); !ok {
		t.Fatal("Get(a) tidak ditemukan")
	}
}

func TestApplyRegistersAndWrapsErrors(t *testing.T) {
	r := NewRegistry()
	if err := r.Add(fake{name: "a", feature: "fitur_a"}); err != nil {
		t.Fatal(err)
	}
	regs := extension.NewRegistries()
	if err := r.Apply(regs); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, ok := regs.Features.Get("fitur_a"); !ok {
		t.Fatal("fitur_a harus terdaftar")
	}

	boom := errors.New("boom")
	r2 := NewRegistry()
	if err := r2.Add(fake{name: "b", regErr: boom}); err != nil {
		t.Fatal(err)
	}
	if err := r2.Apply(extension.NewRegistries()); !errors.Is(err, boom) {
		t.Fatalf("Apply harus membungkus galat modul: %v", err)
	}
}

func TestRoutesDetectsDuplicates(t *testing.T) {
	r := NewRegistry()
	rt := Route{Method: "GET", Path: "/v1/x", Handler: okHandler}
	if err := r.Add(fake{name: "a", routes: []Route{rt}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Add(fake{name: "b", routes: []Route{rt}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Routes(); !errors.Is(err, ErrDuplicateRoute) {
		t.Fatalf("Routes: err = %v, ingin ErrDuplicateRoute", err)
	}
}

func TestMigrateDownGuards(t *testing.T) {
	r := NewRegistry()
	if err := r.Add(fake{name: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := r.MigrateDown(context.Background(), nil, "tidak_ada"); !errors.Is(err, ErrUnknownModule) {
		t.Fatalf("modul tak dikenal: err = %v", err)
	}
	if err := r.MigrateDown(context.Background(), nil, "a"); err != nil {
		t.Fatalf("modul tanpa migrasi harus nihil: %v", err)
	}
}
