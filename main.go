package main

import (
	"fmt"
	"os/exec"
	"os"
	"path/filepath"
	"golang.org/x/mod/modfile"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <repository-url>\n", os.Args[0])
		os.Exit(2)
	}
	repoUrl := os.Args[1]
	tmpDir, err := os.MkdirTemp("", "prefix-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temporary directory: %v\n", err)
		os.Exit(3)
	}
	defer os.RemoveAll(tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")

	cmd := exec.Command("git", "clone", "--depth=1", repoUrl, repoDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to clone repository: %v\n", err)
		fmt.Fprintln(os.Stderr, string(out))
		os.Exit(4)
	}
	info, err := os.Stat(filepath.Join(repoDir, "go.mod"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stat go.mod file: %v\n", err)
		os.Exit(5)
	}
	if info.IsDir() {
		fmt.Fprintf(os.Stderr, "go.mod is a directory: %v\n", info.Name())
		os.Exit(6)
	}

	data, err := os.ReadFile(filepath.Join(repoDir, "go.mod"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read go.mod file: %v\n", err)
		os.Exit(7)
	}

	modFile, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse go.mod file: %v\n", err)
		os.Exit(8)
	}

	fmt.Println("Module: ", modFile.Module.Mod.Path)
	fmt.Println("Go: ", modFile.Go.Version)
}