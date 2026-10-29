package registry_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/master-abror/nizmeet/server/internal/registry"
)

func TestRegisterAndGet(t *testing.T) {
	r := registry.New[int]("angka")
	if err := r.Register("satu", 1); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if v, ok := r.Get("satu"); !ok || v != 1 {
		t.Fatalf("Get(satu) = %d, %v", v, ok)
	}
	if _, ok := r.Get("dua"); ok {
		t.Fatal("Get(dua) seharusnya tidak ada")
	}
	if r.Len() != 1 {
		t.Fatalf("Len = %d, ingin 1", r.Len())
	}
}

func TestRegisterRejectsEmptyAndDuplicate(t *testing.T) {
	r := registry.New[int]("angka")
	if err := r.Register("", 1); !errors.Is(err, registry.ErrEmptyKey) {
		t.Fatalf("kunci kosong: err = %v", err)
	}
	if err := r.Register("a", 1); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := r.Register("a", 2); !errors.Is(err, registry.ErrDuplicate) {
		t.Fatalf("kunci ganda: err = %v", err)
	}
	if v, _ := r.Get("a"); v != 1 {
		t.Fatalf("nilai lama harus utuh, dapat %d", v)
	}
}

func TestKeysAreSorted(t *testing.T) {
	r := registry.New[struct{}]("kunci")
	for _, k := range []string{"b", "a", "c"} {
		if err := r.Register(k, struct{}{}); err != nil {
			t.Fatalf("Register(%s): %v", k, err)
		}
	}
	if got, want := r.Keys(), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Fatalf("Keys = %v, ingin %v", got, want)
	}
}
