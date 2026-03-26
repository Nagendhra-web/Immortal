package cli_test

import (
	"testing"

	"github.com/immortal-engine/immortal/internal/version"
)

func TestVersionOutput(t *testing.T) {
	v := version.Full()
	if v == "" {
		t.Error("version should not be empty")
	}
	if len(v) < 10 {
		t.Error("version string too short")
	}
}
