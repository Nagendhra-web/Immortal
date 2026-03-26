package config_test

import (
	"testing"

	"github.com/immortal-engine/immortal/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.Default()
	if cfg.Name != "immortal" {
		t.Errorf("expected name 'immortal', got '%s'", cfg.Name)
	}
	if cfg.Mode != "reactive" {
		t.Errorf("expected mode 'reactive', got '%s'", cfg.Mode)
	}
	if cfg.API.Port != 7777 {
		t.Errorf("expected port 7777, got %d", cfg.API.Port)
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := config.Default()
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid: %v", err)
	}

	cfg.Mode = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("invalid mode should fail validation")
	}

	cfg.Mode = "reactive"
	cfg.Name = ""
	if err := cfg.Validate(); err == nil {
		t.Error("empty name should fail validation")
	}
}

func TestConfigSaveLoad(t *testing.T) {
	cfg := config.Default()
	cfg.Name = "test-app"

	path := t.TempDir() + "/immortal.json"
	if err := cfg.Save(path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Name != "test-app" {
		t.Errorf("expected name 'test-app', got '%s'", loaded.Name)
	}
}

func TestConfigLoadNotFound(t *testing.T) {
	_, err := config.Load("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
