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
	"os/exec"
	"path/filepath"
	"time"

	"github.com/humooo/go-module-updater-cli/internal/modinfo"
	"github.com/humooo/go-module-updater-cli/internal/runner"
	"github.com/humooo/go-module-updater-cli/internal/updates"
)

const (
	defaultCloneTimeout = 15 * time.Minute
	defaultListTimeout  = 20 * time.Minute
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

func failWithContext(code int, out runner.Output, format string, args ...any) *cliError {
	details := string(out.Stderr)
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
	var flagOut bytes.Buffer
	flags.SetOutput(&flagOut)
	jsonOut := flags.Bool("json", false, "print result as JSON")
	cloneTimeout := flags.Duration("clone-timeout", defaultCloneTimeout, "timeout for git clone")
	listTimeout := flags.Duration("list-timeout", defaultListTimeout, "timeout for go list")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: %s [flags] <git-repository-url>\n\nFlags:\n", filepath.Base(name))
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if flagOut.Len() > 0 {
				_, _ = stdout.Write(flagOut.Bytes())
			}
			return nil
		}
		return fail(2, "invalid arguments: %v", err)
	}
	flags.SetOutput(stderr)
	switch n := flags.NArg(); {
	case n == 0:
		flags.Usage()
		return fail(2, "missing repository URL")
	case n > 1:
		flags.Usage()
		return fail(2, "expected exactly one repository URL")
	}
	repoURL := flags.Arg(0)

	tmpDir, err := os.MkdirTemp("", "gmuc-")
	if err != nil {
		return fail(1, "failed to create temporary directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repoDir := filepath.Join(tmpDir, "repo")

	if err := cloneRepo(context.Background(), r, repoURL, repoDir, *cloneTimeout); err != nil {
		return err
	}

	modInfo, err := readModuleInfo(repoDir)
	if err != nil {
		return err
	}

	depUpdates, err := listUpdates(context.Background(), r, repoDir, *listTimeout)
	if err != nil {
		return err
	}

	res := outputResult{
		Module:    modInfo.Module,
		GoVersion: modInfo.GoVersion,
		Updates:   depUpdates,
	}

	if *jsonOut {
		if err := writeJSON(stdout, res); err != nil {
			return failWithContext(1, runner.Output{}, "failed to write JSON: %v", err)
		}
		return nil
	}

	printText(stdout, res)
	return nil
}

func cloneRepo(ctx context.Context, r runner.Runner, url, destDir string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	out, err := r.Run(ctx, "", "git", "clone", "--depth=1", "--single-branch", url, destDir)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fail(1, "git not found in PATH; please install git")
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fail(1, "git clone timed out after %v", timeout)
		}
		return failWithContext(1, out, "failed to clone repository: %v", err)
	}
	return nil
}

func readModuleInfo(repoDir string) (modinfo.Info, error) {
	goModPath := filepath.Join(repoDir, "go.mod")
	info, err := os.Stat(goModPath)
	if err != nil {
		if os.IsNotExist(err) {
			return modinfo.Info{}, fail(1, "not a Go module: go.mod not found in repository root")
		}
		return modinfo.Info{}, fail(1, "failed to stat go.mod: %v", err)
	}
	if info.IsDir() {
		return modinfo.Info{}, fail(1, "go.mod is a directory")
	}

	data, err := os.ReadFile(goModPath)
	if err != nil {
		return modinfo.Info{}, fail(1, "failed to read go.mod: %v", err)
	}
	modInfo, err := modinfo.Parse(data)
	if err != nil {
		return modinfo.Info{}, fail(1, "failed to parse go.mod: %v", err)
	}
	return modInfo, nil
}

func listUpdates(ctx context.Context, r runner.Runner, repoDir string, timeout time.Duration) ([]updates.DepUpdate, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	out, err := r.Run(ctx, repoDir, "go", "list", "-m", "-u", "-json", "all")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fail(1, "go not found in PATH; please install Go")
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fail(1, "go list timed out after %v", timeout)
		}
		return nil, failWithContext(1, out, "failed to list modules: %v", err)
	}

	depUpdates, err := updates.Parse(bytes.NewReader(out.Stdout))
	if err != nil {
		return nil, failWithContext(1, out, "failed to parse go list output: %v", err)
	}
	return depUpdates, nil
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
