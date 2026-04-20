package gitops

import (
	"os"
	"path/filepath"
)

// writeFile writes data to path, creating parent directories as needed.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// dirIsGitRepo reports whether path contains a .git directory.
func dirIsGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir() || !info.IsDir() // .git can also be a file (submodule)
}
