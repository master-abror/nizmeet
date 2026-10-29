// Package registry menyediakan registry generik berkunci string yang dipakai
// semua titik ekstensi (ADR-0003). Kunci kosong dan kunci ganda ditolak.
package registry

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
)

var (
	// ErrEmptyKey dikembalikan bila kunci kosong.
	ErrEmptyKey = errors.New("registry: kunci kosong")
	// ErrDuplicate dikembalikan bila kunci sudah terdaftar.
	ErrDuplicate = errors.New("registry: kunci sudah terdaftar")
)

// Registry menyimpan nilai bertipe T berdasarkan kunci string.
type Registry[T any] struct {
	name  string
	mu    sync.RWMutex
	items map[string]T
}

// New membuat registry kosong. name hanya dipakai untuk pesan galat.
func New[T any](name string) *Registry[T] {
	return &Registry[T]{name: name, items: make(map[string]T)}
}

// Register menambahkan v di bawah key.
func (r *Registry[T]) Register(key string, v T) error {
	if key == "" {
		return fmt.Errorf("%w (registry %q)", ErrEmptyKey, r.name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[key]; exists {
		return fmt.Errorf("%w: %q (registry %q)", ErrDuplicate, key, r.name)
	}
	r.items[key] = v
	return nil
}

// Get mengambil nilai untuk key.
func (r *Registry[T]) Get(key string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.items[key]
	return v, ok
}

// Keys mengembalikan semua kunci, terurut.
func (r *Registry[T]) Keys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Sorted(maps.Keys(r.items))
}

// Len mengembalikan jumlah entri.
func (r *Registry[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}
