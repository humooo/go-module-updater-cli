package runner

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

type Output struct {
	Stdout []byte
	Stderr []byte
}

type Runner interface {
	Run(ctx context.Context, dir string, cmd string, args ...string) (Output, error)
}

type ExecRunner struct{}

func (r *ExecRunner) Run(ctx context.Context, dir string, cmd string, args ...string) (Output, error) {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	stdOut, stdErr := bytes.Buffer{}, bytes.Buffer{}
	c.Stdout = &stdOut
	c.Stderr = &stdErr
	if err := c.Run(); err != nil {
		return Output{
			Stdout: stdOut.Bytes(),
			Stderr: stdErr.Bytes(),
		}, err
	}
	return Output{
		Stdout: stdOut.Bytes(),
		Stderr: stdErr.Bytes(),
	}, nil
}
