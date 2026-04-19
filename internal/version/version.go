// Package version reports the running binary's version.
//
// During a release build GoReleaser and the Makefile inject Version, GitCommit,
// and BuildDate via -ldflags. When a user runs
//   go install github.com/Nagendhra-web/Immortal/cmd/immortal@v0.5.0
// without those ldflags, we fall back to runtime build info so the version
// string still reflects the installed module tag.
package version

import (
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
)

// These variables are overwritten at build time by ldflags. See .goreleaser.yaml and Makefile.
var (
	Version   = "0.5.0"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

const defaultVersion = "0.5.0"

var resolveOnce sync.Once

// resolve fills the version fields from runtime build info when the defaults
// are still in place, so `go install` users get a truthful answer.
func resolve() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	// Module version, e.g. "v0.5.0" or "(devel)".
	if Version == defaultVersion && info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = strings.TrimPrefix(info.Main.Version, "v")
	}
	// VCS metadata baked in by the Go toolchain since 1.18.
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if GitCommit == "unknown" && s.Value != "" {
				if len(s.Value) >= 7 {
					GitCommit = s.Value[:7]
				} else {
					GitCommit = s.Value
				}
			}
		case "vcs.time":
			if BuildDate == "unknown" && s.Value != "" {
				BuildDate = s.Value
			}
		}
	}
}

// Full returns a human-readable version string.
func Full() string {
	resolveOnce.Do(resolve)
	return fmt.Sprintf("immortal v%s (commit: %s, built: %s)", Version, GitCommit, BuildDate)
}

// Short returns just the version number, e.g. "0.5.0".
func Short() string {
	resolveOnce.Do(resolve)
	return Version
}
