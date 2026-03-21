package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/humooo/go-module-updater-cli/internal/runner"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/humooo/go-module-updater-cli/internal/modinfo"
)

const (
	cloneTimeout = 15 * time.Minute
	listTimeout  = 20 * time.Minute
)

func main() {
	os.Exit(run())
}

func run() int {
	jsonOut := flag.Bool("json", false, "print result as JSON")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [--json] <git-repository-url>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		return 2
	}
	repoURL := flag.Arg(0)

	tmpDir, err := os.MkdirTemp("", "gmuc-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temporary directory: %v\n", err)
		return 3
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repoDir := filepath.Join(tmpDir, "repo")

	cloneCtx, cancelClone := context.WithTimeout(context.Background(), cloneTimeout)
	defer cancelClone()

	r := runner.ExecRunner{}
	out, err := r.Run(cloneCtx, "", "git", "clone", "--depth=1", "--single-branch", repoURL, repoDir)
	if err != nil {
		if errors.Is(cloneCtx.Err(), context.DeadlineExceeded) {
			fmt.Fprintf(os.Stderr, "git clone timed out after %v\n", cloneTimeout)
			return 4
		}
		fmt.Fprintf(os.Stderr, "failed to clone repository: %v\n", err)
		fmt.Fprintln(os.Stderr, string(out))
		return 4
	}

	goModPath := filepath.Join(repoDir, "go.mod")
	info, err := os.Stat(goModPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "not a Go module: go.mod not found in repository root\n")
			return 5
		}
		fmt.Fprintf(os.Stderr, "failed to stat go.mod: %v\n", err)
		return 5
	}
	if info.IsDir() {
		fmt.Fprintf(os.Stderr, "go.mod is a directory\n")
		return 6
	}

	data, err := os.ReadFile(goModPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read go.mod: %v\n", err)
		return 7
	}
	modInfo, err := modinfo.Parse(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse go.mod: %v\n", err)
		return 8
	}

	modulePath := modInfo.Module
	goVer := modInfo.GoVersion

	listCtx, cancelList := context.WithTimeout(context.Background(), listTimeout)
	defer cancelList()

	listOut, err := r.Run(listCtx, repoDir, "go", "list", "-m", "-u", "-json", "all")
	if err != nil {
		if errors.Is(listCtx.Err(), context.DeadlineExceeded) {
			fmt.Fprintf(os.Stderr, "go list timed out after %v\n", listTimeout)
			return 9
		}
		fmt.Fprintf(os.Stderr, "failed to list modules: %v\n", err)
		fmt.Fprintln(os.Stderr, string(listOut))
		return 9
	}

	updates, err := parseModuleUpdates(bytes.NewReader(listOut))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse go list output: %v\n", err)
		return 10
	}

	res := outputResult{
		Module:    modulePath,
		GoVersion: goVer,
		Updates:   updates,
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write JSON: %v\n", err)
			return 1
		}
		return 0
	}

	printText(res)
	return 0
}

type moduleLine struct {
	Path     string `json:"Path"`
	Version  string `json:"Version"`
	Main     bool   `json:"Main"`
	Indirect bool   `json:"Indirect"`
	Update   *struct {
		Path    string `json:"Path"`
		Version string `json:"Version"`
	} `json:"Update"`
}

type depUpdate struct {
	Path     string `json:"path"`
	Current  string `json:"currentVersion"`
	Latest   string `json:"latestVersion"`
	Indirect bool   `json:"indirect,omitempty"`
}

type outputResult struct {
	Module    string      `json:"module"`
	GoVersion string      `json:"goVersion"`
	Updates   []depUpdate `json:"updates"`
}

func parseModuleUpdates(r io.Reader) ([]depUpdate, error) {
	dec := json.NewDecoder(r)
	var out []depUpdate
	for {
		var m moduleLine
		if err := dec.Decode(&m); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if m.Main || m.Update == nil {
			continue
		}
		latest := m.Update.Version
		if latest == "" {
			continue
		}
		out = append(out, depUpdate{
			Path:     m.Path,
			Current:  m.Version,
			Latest:   latest,
			Indirect: m.Indirect,
		})
	}
	return out, nil
}

func printText(res outputResult) {
	fmt.Printf("module: %s\n", res.Module)
	fmt.Printf("go: %s\n", res.GoVersion)
	fmt.Println("updates:")
	if len(res.Updates) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, u := range res.Updates {
		kind := "direct"
		if u.Indirect {
			kind = "indirect"
		}
		fmt.Printf("  - %s: %s -> %s (%s)\n", u.Path, u.Current, u.Latest, kind)
	}
}
