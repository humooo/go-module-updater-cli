package main

import (
	"fmt"
	"os/exec"
	"os"
	"path/filepath"
	"golang.org/x/mod/modfile"
	"encoding/json"
	"bytes"
	"io"
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

	cmd = exec.Command("go", "list", "-m", "-u", "-json", "all")
	cmd.Dir = repoDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list modules: %v\n", err)
		fmt.Fprintln(os.Stderr, string(out))
		os.Exit(9)
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var m struct {
			Path    string `json:"Path"`
			Version string `json:"Version"`
			Main    bool   `json:"Main"`
			Replace struct {
				Path    string `json:"Path"`
				Version string `json:"Version"`
			} `json:"Replace"`
			Update *struct {
				Path    string `json:"Path"`
				Version string `json:"Version"`
			} `json:"Update"`
		}
		if err := dec.Decode(&m); err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "Failed to decode module: %v\n", err)
			os.Exit(10)
		}
		if m.Main || m.Update == nil {
			continue
		}
		fmt.Println(m.Path)
		fmt.Println(m.Version)
		fmt.Println(m.Update.Path)
		fmt.Println(m.Update.Version)
	}
	
	fmt.Println("--------------------------------")	
	fmt.Println("Module:", modFile.Module.Mod.Path)
	fmt.Println("Go:", modFile.Go.Version)
	fmt.Println("--------------------------------")
}