package modules_test

import (
	"testing"

	"github.com/master-abror/nizmeet/server/internal/extension"
	"github.com/master-abror/nizmeet/server/internal/module"
	"github.com/master-abror/nizmeet/server/internal/modules"
)

// TestAllRegistersCleanly menjaga agar semua modul terdaftar memenuhi kontrak:
// nama unik, skema sama dengan nama, rute lengkap dan tidak ganda.
func TestAllRegistersCleanly(t *testing.T) {
	reg := module.NewRegistry()
	for _, m := range modules.All() {
		if err := reg.Add(m); err != nil {
			t.Fatalf("Add(%q): %v", m.Name(), err)
		}
	}
	if err := reg.Apply(extension.NewRegistries()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := reg.Routes(); err != nil {
		t.Fatalf("Routes: %v", err)
	}
}
