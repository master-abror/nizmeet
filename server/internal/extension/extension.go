// Package extension memuat empat registry titik ekstensi Milestone 0.4:
// tipe dokumen, resource kind sync, LLMTask, dan entitlement (ADR-0003, butir 3).
// Isi bawaan didaftarkan oleh milestone yang memilikinya, bukan di sini.
package extension

import (
	"errors"
	"fmt"

	"github.com/master-abror/nizmeet/server/internal/registry"
)

// ErrInvalidSchemaVersion dikembalikan bila schema_version kurang dari 1.
var ErrInvalidSchemaVersion = errors.New("extension: schema_version harus bernilai 1 atau lebih")

// DocumentType adalah satu tipe dokumen (documents.type berupa string, bukan enum DB).
type DocumentType struct {
	Key           string
	SchemaVersion int
}

// SyncKind adalah satu resource kind sync.
type SyncKind struct {
	Key           string
	SchemaVersion int
}

// LLMTask adalah satu tugas LLM (mis. fast_pass, translate, main_pass).
type LLMTask struct {
	Key string
}

// Feature adalah satu kunci fitur entitlement (kode memeriksa fitur, bukan nama paket).
type Feature struct {
	Key string
}

// Registries mengelompokkan keempat registry.
type Registries struct {
	DocumentTypes *registry.Registry[DocumentType]
	SyncKinds     *registry.Registry[SyncKind]
	LLMTasks      *registry.Registry[LLMTask]
	Features      *registry.Registry[Feature]
}

// NewRegistries membuat keempat registry dalam keadaan kosong.
func NewRegistries() *Registries {
	return &Registries{
		DocumentTypes: registry.New[DocumentType]("document_types"),
		SyncKinds:     registry.New[SyncKind]("sync_kinds"),
		LLMTasks:      registry.New[LLMTask]("llm_tasks"),
		Features:      registry.New[Feature]("features"),
	}
}

// AddDocumentType mendaftarkan tipe dokumen di bawah d.Key.
func (r *Registries) AddDocumentType(d DocumentType) error {
	if d.SchemaVersion < 1 {
		return fmt.Errorf("%w: tipe dokumen %q", ErrInvalidSchemaVersion, d.Key)
	}
	return r.DocumentTypes.Register(d.Key, d)
}

// AddSyncKind mendaftarkan resource kind sync di bawah k.Key.
func (r *Registries) AddSyncKind(k SyncKind) error {
	if k.SchemaVersion < 1 {
		return fmt.Errorf("%w: kind sync %q", ErrInvalidSchemaVersion, k.Key)
	}
	return r.SyncKinds.Register(k.Key, k)
}

// AddLLMTask mendaftarkan tugas LLM di bawah t.Key.
func (r *Registries) AddLLMTask(t LLMTask) error {
	return r.LLMTasks.Register(t.Key, t)
}

// AddFeature mendaftarkan kunci fitur di bawah f.Key.
func (r *Registries) AddFeature(f Feature) error {
	return r.Features.Register(f.Key, f)
}
