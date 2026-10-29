package extension_test

import (
	"errors"
	"testing"

	"github.com/master-abror/nizmeet/server/internal/extension"
	"github.com/master-abror/nizmeet/server/internal/registry"
)

func TestAddEachKind(t *testing.T) {
	r := extension.NewRegistries()
	if err := r.AddDocumentType(extension.DocumentType{Key: "cv", SchemaVersion: 1}); err != nil {
		t.Fatalf("AddDocumentType: %v", err)
	}
	if err := r.AddSyncKind(extension.SyncKind{Key: "documents", SchemaVersion: 1}); err != nil {
		t.Fatalf("AddSyncKind: %v", err)
	}
	if err := r.AddLLMTask(extension.LLMTask{Key: "fast_pass"}); err != nil {
		t.Fatalf("AddLLMTask: %v", err)
	}
	if err := r.AddFeature(extension.Feature{Key: "copilot"}); err != nil {
		t.Fatalf("AddFeature: %v", err)
	}
	if _, ok := r.DocumentTypes.Get("cv"); !ok {
		t.Fatal("tipe dokumen cv tidak ditemukan")
	}
	if _, ok := r.SyncKinds.Get("documents"); !ok {
		t.Fatal("kind documents tidak ditemukan")
	}
	if _, ok := r.LLMTasks.Get("fast_pass"); !ok {
		t.Fatal("tugas fast_pass tidak ditemukan")
	}
	if _, ok := r.Features.Get("copilot"); !ok {
		t.Fatal("fitur copilot tidak ditemukan")
	}
}

func TestRejectsDuplicateAndInvalid(t *testing.T) {
	r := extension.NewRegistries()
	if err := r.AddFeature(extension.Feature{Key: "x"}); err != nil {
		t.Fatalf("AddFeature: %v", err)
	}
	if err := r.AddFeature(extension.Feature{Key: "x"}); !errors.Is(err, registry.ErrDuplicate) {
		t.Fatalf("duplikat: err = %v", err)
	}
	if err := r.AddDocumentType(extension.DocumentType{Key: "t", SchemaVersion: 0}); !errors.Is(err, extension.ErrInvalidSchemaVersion) {
		t.Fatalf("schema_version 0 pada tipe dokumen: err = %v", err)
	}
	if err := r.AddSyncKind(extension.SyncKind{Key: "k", SchemaVersion: 0}); !errors.Is(err, extension.ErrInvalidSchemaVersion) {
		t.Fatalf("schema_version 0 pada kind sync: err = %v", err)
	}
}
