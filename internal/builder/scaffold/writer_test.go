package scaffold_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/immortal-engine/immortal/internal/builder/scaffold"
	"github.com/immortal-engine/immortal/internal/builder/template"
)

func TestWriteFiles(t *testing.T) {
	dir := t.TempDir()

	files := []template.GeneratedFile{
		{Path: "main.go", Content: "package main", Type: template.FileTypeGo},
		{Path: "pkg/util.go", Content: "package pkg", Type: template.FileTypeGo},
		{Path: "README.md", Content: "# Hello", Type: template.FileTypeMarkdown},
	}

	result, err := scaffold.Write(dir, files)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.FilesCreated) != 3 {
		t.Errorf("expected 3 files created, got %d", len(result.FilesCreated))
	}

	// Verify files exist
	content, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "package main" {
		t.Error("file content mismatch")
	}

	// Verify nested dir was created
	content, err = os.ReadFile(filepath.Join(dir, "pkg", "util.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "package pkg" {
		t.Error("nested file content mismatch")
	}
}

func TestPreview(t *testing.T) {
	files := []template.GeneratedFile{
		{Path: "a.go", Content: "aaa"},
		{Path: "b.go", Content: "bbb"},
	}

	preview := scaffold.Preview(files)
	if len(preview) != 2 {
		t.Errorf("expected 2 entries, got %d", len(preview))
	}
	if preview["a.go"] != "aaa" {
		t.Error("wrong preview content")
	}
}

func TestEndToEnd(t *testing.T) {
	// Full pipeline: template -> generate -> scaffold
	eng := template.New()
	files, err := eng.Generate("api-go", template.ProjectSpec{
		Name:        "test-project",
		Description: "E2E test",
	})
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(t.TempDir(), "test-project")
	result, err := scaffold.Write(dir, files)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.FilesCreated) < 3 {
		t.Errorf("expected at least 3 files, got %d", len(result.FilesCreated))
	}
	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}
