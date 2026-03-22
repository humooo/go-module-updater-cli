package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/humooo/go-module-updater-cli/internal/modinfo"
	"github.com/humooo/go-module-updater-cli/internal/runner"
	"github.com/humooo/go-module-updater-cli/internal/updates"
)

const (
	cloneTimeout = 15 * time.Minute
	listTimeout  = 20 * time.Minute
)

type cliError struct {
	code    int
	message string
	details string
}

func (e *cliError) Error() string {
	return e.message
}

func fail(code int, format string, args ...any) *cliError {
	return &cliError{
		code:    code,
		message: fmt.Sprintf(format, args...),
		details: "",
	}
}

func failWithDetails(code int, out []byte, format string, args ...any) *cliError {
	details := string(out)
	if len(details) > 1000 {
		details = details[:1000] + "..."
	}
	return &cliError{
		code:    code,
		message: fmt.Sprintf(format, args...),
		details: details,
	}
}

func main() {
	os.Exit(realMain(os.Args[0], os.Args[1:], os.Stdout, os.Stderr, &runner.ExecRunner{}))
}

func realMain(name string, args []string, stdout, stderr io.Writer, r runner.Runner) int {
	if err := execute(name, args, stdout, stderr, r); err != nil {
		printError(stderr, err)
		return exitCode(err)
	}
	return 0
}

func exitCode(err error) int {
	var cliErr *cliError
	if errors.As(err, &cliErr) && cliErr.code == 2 {
		return 2
	}
	return 1
}

func printError(w io.Writer, err error) {
	var cliErr *cliError
	if errors.As(err, &cliErr) {
		fmt.Fprintf(w, "error: %s\n", cliErr.message)
		if cliErr.details != "" {
			fmt.Fprintf(w, "details: %s\n", cliErr.details)
		}
		return
	}
	fmt.Fprintf(w, "error: %v\n", err)
}

func execute(name string, args []string, stdout, stderr io.Writer, r runner.Runner) error {
	flags := flag.NewFlagSet(filepath.Base(name), flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOut := flags.Bool("json", false, "print result as JSON")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "Usage: %s [--json] <git-repository-url>\n", filepath.Base(name))
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return fail(2, "invalid arguments: %v", err)
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return fail(2, "missing repository URL")
	}
	repoURL := flags.Arg(0)

	tmpDir, err := os.MkdirTemp("", "gmuc-")
	if err != nil {
		return fail(1, "failed to create temporary directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repoDir := filepath.Join(tmpDir, "repo")

	cloneCtx, cancelClone := context.WithTimeout(context.Background(), cloneTimeout)
	defer cancelClone()

	out, err := r.Run(cloneCtx, "", "git", "clone", "--depth=1", "--single-branch", repoURL, repoDir)
	if err != nil {
		if errors.Is(cloneCtx.Err(), context.DeadlineExceeded) {
			return fail(1, "git clone timed out after %v", cloneTimeout)
		}
		return failWithDetails(1, out, "failed to clone repository: %v", err)

	}

	goModPath := filepath.Join(repoDir, "go.mod")
	info, err := os.Stat(goModPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fail(1, "not a Go module: go.mod not found in repository root")
		}
		return fail(1, "failed to stat go.mod: %v", err)
	}
	if info.IsDir() {
		return fail(1, "go.mod is a directory")
	}

	data, err := os.ReadFile(goModPath)
	if err != nil {
		return fail(1, "failed to read go.mod: %v", err)
	}
	modInfo, err := modinfo.Parse(data)
	if err != nil {
		return failWithDetails(1, data, "failed to parse go.mod: %v", err)
	}

	modulePath := modInfo.Module
	goVer := modInfo.GoVersion

	listCtx, cancelList := context.WithTimeout(context.Background(), listTimeout)
	defer cancelList()

	listOut, err := r.Run(listCtx, repoDir, "go", "list", "-m", "-u", "-json", "all")
	if err != nil {
		if errors.Is(listCtx.Err(), context.DeadlineExceeded) {
			return fail(1, "go list timed out after %v", listTimeout)
		}
		return failWithDetails(1, listOut, "failed to list modules: %v", err)
	}

	depUpdates, err := updates.Parse(bytes.NewReader(listOut))
	if err != nil {
		return failWithDetails(1, listOut, "failed to parse go list output: %v", err)
	}

	res := outputResult{
		Module:    modulePath,
		GoVersion: goVer,
		Updates:   depUpdates,
	}

	if *jsonOut {
		if err := writeJSON(stdout, res); err != nil {
			return failWithDetails(1, nil, "failed to write JSON: %v", err)
		}
		return nil
	}

	printText(stdout, res)
	return nil
}

type outputResult struct {
	Module    string              `json:"module"`
	GoVersion string              `json:"goVersion"`
	Updates   []updates.DepUpdate `json:"updates"`
}

func writeJSON(w io.Writer, res outputResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

func printText(w io.Writer, res outputResult) {
	fmt.Fprintf(w, "module: %s\n", res.Module)
	fmt.Fprintf(w, "go: %s\n", res.GoVersion)
	fmt.Fprintln(w, "updates:")
	if len(res.Updates) == 0 {
		fmt.Fprintln(w, "  (none)")
		return
	}
	for _, u := range res.Updates {
		kind := "direct"
		if u.Indirect {
			kind = "indirect"
		}
		fmt.Fprintf(w, "  - %s: %s -> %s (%s)\n", u.Path, u.Current, u.Latest, kind)
	}
}
