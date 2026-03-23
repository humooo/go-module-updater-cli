package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/humooo/go-module-updater-cli/internal/runner"
	"github.com/humooo/go-module-updater-cli/internal/updates"
)

type fakeRunner struct {
	handler func(ctx context.Context, dir, cmd string, args ...string) (runner.Output, error)
}

func (f *fakeRunner) Run(ctx context.Context, dir, cmd string, args ...string) (runner.Output, error) {
	return f.handler(ctx, dir, cmd, args...)
}

const validGoMod = "module example.com/testmod\n\ngo 1.21\n"

const goListOutput = `{"Path":"example.com/testmod","Version":"v0.0.0","Main":true}
{"Path":"dep.example/x","Version":"v1.0.0","Update":{"Path":"dep.example/x","Version":"v1.1.0"}}
{"Path":"dep.example/y","Version":"v0.3.0","Indirect":true,"Update":{"Path":"dep.example/y","Version":"v0.4.0"}}
`

func happyRunner(_ context.Context, _ string, cmd string, args ...string) (runner.Output, error) {
	if cmd == "git" {
		repoDir := args[len(args)-1]
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			return runner.Output{}, fmt.Errorf("fake: mkdir: %w", err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte(validGoMod), 0o644); err != nil {
			return runner.Output{}, fmt.Errorf("fake: write go.mod: %w", err)
		}
		return runner.Output{}, nil
	}
	if cmd == "go" {
		return runner.Output{Stdout: []byte(goListOutput)}, nil
	}
	return runner.Output{}, fmt.Errorf("fake: unexpected command %q", cmd)
}

func TestRealMain_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := realMain("gmuc", nil, &stdout, &stderr, &fakeRunner{handler: happyRunner})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing repository URL") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "missing repository URL")
	}
}

func TestRealMain_TooManyArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := realMain("gmuc", []string{"a", "b"}, &stdout, &stderr, &fakeRunner{handler: happyRunner})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "expected exactly one repository URL") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "expected exactly one repository URL")
	}
}

func TestRealMain_InvalidFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := realMain("gmuc", []string{"--bad"}, &stdout, &stderr, &fakeRunner{handler: happyRunner})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "invalid arguments") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "invalid arguments")
	}
}

func TestRealMain_CloneError(t *testing.T) {
	fake := &fakeRunner{handler: func(_ context.Context, _, cmd string, _ ...string) (runner.Output, error) {
		if cmd == "git" {
			return runner.Output{Stderr: []byte("fatal: repo not found")}, errors.New("exit status 128")
		}
		return runner.Output{}, nil
	}}
	var stdout, stderr bytes.Buffer
	code := realMain("gmuc", []string{"https://example.com/repo.git"}, &stdout, &stderr, fake)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "failed to clone repository") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "failed to clone repository")
	}
}

func TestRealMain_NoGoMod(t *testing.T) {
	fake := &fakeRunner{handler: func(_ context.Context, _, cmd string, args ...string) (runner.Output, error) {
		if cmd == "git" {
			repoDir := args[len(args)-1]
			if err := os.MkdirAll(repoDir, 0o755); err != nil {
				return runner.Output{}, err
			}
			return runner.Output{}, nil
		}
		return runner.Output{}, nil
	}}
	var stdout, stderr bytes.Buffer
	code := realMain("gmuc", []string{"https://example.com/repo.git"}, &stdout, &stderr, fake)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "go.mod not found") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "go.mod not found")
	}
}

func TestRealMain_HappyPathText(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := realMain("gmuc", []string{"https://example.com/repo.git"}, &stdout, &stderr, &fakeRunner{handler: happyRunner})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"module: example.com/testmod",
		"go: 1.21",
		"updates:",
		"dep.example/x: v1.0.0 -> v1.1.0 (direct)",
		"dep.example/y: v0.3.0 -> v0.4.0 (indirect)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestRealMain_HappyPathJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := realMain("gmuc", []string{"--json", "https://example.com/repo.git"}, &stdout, &stderr, &fakeRunner{handler: happyRunner})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}

	var res struct {
		Module    string              `json:"module"`
		GoVersion string              `json:"goVersion"`
		Updates   []updates.DepUpdate `json:"updates"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, stdout.String())
	}
	if res.Module != "example.com/testmod" {
		t.Errorf("Module = %q, want %q", res.Module, "example.com/testmod")
	}
	if res.GoVersion != "1.21" {
		t.Errorf("GoVersion = %q, want %q", res.GoVersion, "1.21")
	}
	if len(res.Updates) != 2 {
		t.Fatalf("len(Updates) = %d, want 2", len(res.Updates))
	}
	if res.Updates[0].Path != "dep.example/x" || res.Updates[0].Current != "v1.0.0" || res.Updates[0].Latest != "v1.1.0" {
		t.Errorf("Updates[0] = %+v, unexpected", res.Updates[0])
	}
	if res.Updates[1].Path != "dep.example/y" || res.Updates[1].Indirect != true {
		t.Errorf("Updates[1] = %+v, unexpected", res.Updates[1])
	}
}
