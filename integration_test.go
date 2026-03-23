//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/humooo/go-module-updater-cli/internal/runner"
)

// TestIntegration_MinimalRepo runs real git clone (file://) and go list against a local bare-style repo.
func TestIntegration_MinimalRepo(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "origin")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = repoPath
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
			"GIT_AUTHOR_DATE=2020-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2020-01-01T00:00:00Z",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v %v: %s", name, args, out)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "t@t")
	run("git", "config", "user.name", "t")

	goMod := []byte("module example.com/integrationtest\n\ngo 1.22\n")
	if err := os.WriteFile(filepath.Join(repoPath, "go.mod"), goMod, 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "go.mod")
	run("git", "commit", "-m", "init")

	abs, err := filepath.Abs(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	cloneURL := "file://" + filepath.ToSlash(abs)

	var stdout strings.Builder
	var stderr strings.Builder
	code := realMain("gmuc", []string{cloneURL}, &stdout, &stderr, &runner.ExecRunner{})
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "example.com/integrationtest") {
		t.Fatalf("stdout: %s", out)
	}
}
