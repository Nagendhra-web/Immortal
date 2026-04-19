package template_test

import (
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/builder/template"
)

func TestGenerateGoAPI(t *testing.T) {
	e := template.New()

	files, err := e.Generate("api-go", template.ProjectSpec{
		Name:        "my-api",
		Description: "A test API",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(files) < 3 {
		t.Errorf("expected at least 3 files, got %d", len(files))
	}

	hasMain := false
	for _, f := range files {
		if f.Path == "main.go" {
			hasMain = true
		}
	}
	if !hasMain {
		t.Error("expected main.go")
	}
}

func TestGenerateTSAPI(t *testing.T) {
	e := template.New()

	files, err := e.Generate("api-typescript", template.ProjectSpec{
		Name: "my-ts-api",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(files) < 2 {
		t.Errorf("expected at least 2 files, got %d", len(files))
	}
}

func TestGeneratePythonAPI(t *testing.T) {
	e := template.New()

	files, err := e.Generate("api-python", template.ProjectSpec{
		Name: "my-python-api",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(files) < 2 {
		t.Errorf("expected at least 2 files, got %d", len(files))
	}
}

func TestGenerateStaticHTML(t *testing.T) {
	e := template.New()

	files, err := e.Generate("static-html", template.ProjectSpec{
		Name:        "My Site",
		Description: "A cool website",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(files) < 2 {
		t.Errorf("expected at least 2 files, got %d", len(files))
	}
}

func TestUnknownTemplate(t *testing.T) {
	e := template.New()
	_, err := e.Generate("nonexistent", template.ProjectSpec{})
	if err == nil {
		t.Error("expected error for unknown template")
	}
}

func TestListTemplates(t *testing.T) {
	e := template.New()
	templates := e.Templates()
	if len(templates) < 4 {
		t.Errorf("expected at least 4 templates, got %d", len(templates))
	}
}

func TestCustomTemplate(t *testing.T) {
	e := template.New()
	e.Register("custom", func(spec template.ProjectSpec) []template.GeneratedFile {
		return []template.GeneratedFile{
			{Path: "custom.txt", Content: spec.Name, Type: template.FileTypeMarkdown},
		}
	})

	files, err := e.Generate("custom", template.ProjectSpec{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
	if files[0].Content != "test" {
		t.Error("custom template should use spec name")
	}
}
