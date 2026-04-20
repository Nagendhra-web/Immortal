// Package gitops lets Immortal persist its healing actions as commits
// in a GitOps repository. Argo CD / Flux / any reconciler then propagates
// the change to the rest of the fleet.
//
// The package is intentionally thin: it shells out to git(1) rather than
// bringing in a full git library. This matches what real SRE teams do,
// keeps auditing of the commands trivial, and avoids large dependency
// surfaces that would clash with FedRAMP / air-gapped builds.
//
// Usage:
//
//	gc := gitops.New(gitops.Config{
//	    WorkDir:    "/var/lib/immortal/gitops",
//	    RemoteURL:  "git@github.com:org/infra-prod.git",
//	    Branch:     "main",
//	    AuthorName: "immortal",
//	    AuthorEmail: "immortal@example.com",
//	})
//	err := gc.Commit(gitops.Change{
//	    Path:    "clusters/prod/rest/scale.yaml",
//	    Content: []byte("replicas: 6\n"),
//	    Message: "chore(heal): rest replicas 4 -> 6 (inc-42)",
//	    Verdict: verdictYaml,
//	})
package gitops

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Config describes the target repository + authentication.
type Config struct {
	WorkDir     string // local clone location; must exist and be writable by the engine
	RemoteURL   string // git remote URL (ssh:// or https://)
	Branch      string // branch to commit to, e.g. "main"
	AuthorName  string // git author.name; should clearly identify the engine
	AuthorEmail string // git author.email; used for loop prevention
	SigningKey  string // optional GPG key fingerprint for signed commits
}

// Change is one write to the repo.
type Change struct {
	Path    string // repo-relative path, e.g. "clusters/prod/rest/scale.yaml"
	Content []byte // new file contents (fully overwrites existing)
	Message string // short commit message (first line)
	Verdict []byte // optional: verdict markdown to include in commit body
}

// Client performs git operations against a single repo.
type Client struct {
	cfg   Config
	runCmd func(name string, args ...string) *exec.Cmd
}

// New constructs a Client. Requires Config.WorkDir, RemoteURL, Branch,
// and author credentials to be set; returns an error listing any missing.
func New(cfg Config) (*Client, error) {
	var missing []string
	if cfg.WorkDir == "" {
		missing = append(missing, "WorkDir")
	}
	if cfg.RemoteURL == "" {
		missing = append(missing, "RemoteURL")
	}
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	if cfg.AuthorName == "" {
		cfg.AuthorName = "immortal"
	}
	if cfg.AuthorEmail == "" {
		cfg.AuthorEmail = "immortal@localhost"
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("gitops: missing required config: %s", strings.Join(missing, ", "))
	}
	return &Client{cfg: cfg, runCmd: exec.Command}, nil
}

// EnsureClone pulls the repo into WorkDir if it does not already exist,
// then pulls the latest changes on the configured branch. Safe to call
// repeatedly; it is the first thing Commit() does.
func (c *Client) EnsureClone() error {
	if dirIsGitRepo(c.cfg.WorkDir) {
		return c.run("fetch", "origin", c.cfg.Branch)
	}
	// Clone fresh.
	return c.runAt(filepath.Dir(c.cfg.WorkDir), "clone", "--branch", c.cfg.Branch, c.cfg.RemoteURL, filepath.Base(c.cfg.WorkDir))
}

// Commit writes the Change to the configured path, commits, and pushes.
// Returns the commit hash on success.
func (c *Client) Commit(ch Change) (string, error) {
	if ch.Path == "" {
		return "", errors.New("gitops: Change.Path is required")
	}
	if ch.Message == "" {
		ch.Message = "chore(immortal): automated change"
	}
	if err := c.EnsureClone(); err != nil {
		return "", fmt.Errorf("gitops: ensure clone: %w", err)
	}
	// Check out + reset the branch to origin to avoid local drift.
	if err := c.run("checkout", c.cfg.Branch); err != nil {
		return "", err
	}
	if err := c.run("reset", "--hard", "origin/"+c.cfg.Branch); err != nil {
		return "", err
	}
	// Write file (via external cp wrapper for clarity; Go test-time can
	// intercept this by replacing runCmd).
	if err := writeFile(filepath.Join(c.cfg.WorkDir, ch.Path), ch.Content); err != nil {
		return "", fmt.Errorf("gitops: write: %w", err)
	}
	if err := c.run("add", ch.Path); err != nil {
		return "", err
	}
	// Build the full commit message including the Verdict body if present.
	msg := ch.Message
	if len(ch.Verdict) > 0 {
		msg = ch.Message + "\n\n" + string(ch.Verdict)
	}
	commitArgs := []string{
		"-c", "user.name=" + c.cfg.AuthorName,
		"-c", "user.email=" + c.cfg.AuthorEmail,
		"commit", "-m", msg,
	}
	if c.cfg.SigningKey != "" {
		commitArgs = append(commitArgs, "-S", "--gpg-sign="+c.cfg.SigningKey)
	}
	if err := c.run(commitArgs...); err != nil {
		return "", err
	}
	// Push.
	if err := c.run("push", "origin", c.cfg.Branch); err != nil {
		return "", err
	}
	// Return the commit hash.
	out, err := c.runOutput("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// IsOurCommit reports whether the given commit was authored by Immortal
// (matches Config.AuthorEmail). Used by the engine loop-prevention logic
// so the twin does not react to its own past changes.
func (c *Client) IsOurCommit(sha string) (bool, error) {
	out, err := c.runOutput("log", "-1", "--format=%ae", sha)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == c.cfg.AuthorEmail, nil
}

// Rollback reverts the given commit, writes a new commit with the
// inverse of the change, and pushes. Use when a twin post-apply check
// fails, or when an operator rejects an applied change.
func (c *Client) Rollback(sha, reason string) (string, error) {
	if err := c.EnsureClone(); err != nil {
		return "", err
	}
	revertArgs := []string{
		"-c", "user.name=" + c.cfg.AuthorName,
		"-c", "user.email=" + c.cfg.AuthorEmail,
		"revert", "--no-edit", sha,
	}
	if err := c.run(revertArgs...); err != nil {
		return "", err
	}
	if err := c.run("push", "origin", c.cfg.Branch); err != nil {
		return "", err
	}
	// Amend the revert commit with a human-readable reason.
	if reason != "" {
		amendArgs := []string{
			"-c", "user.name=" + c.cfg.AuthorName,
			"-c", "user.email=" + c.cfg.AuthorEmail,
			"commit", "--amend", "-m",
			fmt.Sprintf("revert: %s\n\nReason: %s", sha, reason),
		}
		if err := c.run(amendArgs...); err != nil {
			return "", err
		}
		if err := c.run("push", "--force-with-lease", "origin", c.cfg.Branch); err != nil {
			return "", err
		}
	}
	out, err := c.runOutput("rev-parse", "HEAD")
	return strings.TrimSpace(out), err
}

// ── internals ────────────────────────────────────────────────────────────

func (c *Client) run(args ...string) error {
	cmd := c.runCmd("git", args...)
	cmd.Dir = c.cfg.WorkDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *Client) runAt(dir string, args ...string) error {
	cmd := c.runCmd("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s (in %s): %w (%s)", strings.Join(args, " "), dir, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *Client) runOutput(args ...string) (string, error) {
	cmd := c.runCmd("git", args...)
	cmd.Dir = c.cfg.WorkDir
	out, err := cmd.Output()
	return string(out), err
}
