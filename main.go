package main

import (
	"fmt"
	"os/exec"
	"os"
	"path/filepath"
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
	fmt.Println(string(out))
}