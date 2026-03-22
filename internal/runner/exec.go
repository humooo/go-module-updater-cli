package runner

import (
	"context"
	"os/exec"
	"os"
)

type Runner interface {
	Run(ctx context.Context, dir string, cmd string, args ...string) ([]byte, error)
}

type ExecRunner struct {}

func (r *ExecRunner) Run(ctx context.Context, dir string, cmd string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return c.CombinedOutput()
}