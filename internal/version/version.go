package version

import "fmt"

var (
	Version   = "0.3.0"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func Full() string {
	return fmt.Sprintf("immortal v%s (commit: %s, built: %s)", Version, GitCommit, BuildDate)
}
