package scaffold

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/immortal-engine/immortal/internal/builder/template"
)

type Result struct {
	ProjectDir   string   `json:"project_dir"`
	FilesCreated []string `json:"files_created"`
	Errors       []string `json:"errors,omitempty"`
}

func Write(baseDir string, files []template.GeneratedFile) (*Result, error) {
	result := &Result{ProjectDir: baseDir}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("scaffold: mkdir base: %w", err)
	}

	for _, f := range files {
		fullPath := filepath.Join(baseDir, f.Path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("mkdir %s: %v", dir, err))
			continue
		}
		if err := os.WriteFile(fullPath, []byte(f.Content), 0644); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("write %s: %v", fullPath, err))
			continue
		}
		result.FilesCreated = append(result.FilesCreated, f.Path)
	}

	return result, nil
}

func Preview(files []template.GeneratedFile) map[string]string {
	preview := make(map[string]string)
	for _, f := range files {
		preview[f.Path] = f.Content
	}
	return preview
}
