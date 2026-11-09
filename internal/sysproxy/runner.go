package sysproxy

import (
	"context"
	"os/exec"
)

// runner runs one system command. Tests replace it to watch the commands a
// platform would run.
type runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// execRunner runs the commands for real. LC_ALL=C keeps networksetup's output
// in the one language the parsers know.
type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(cmd.Environ(), "LC_ALL=C")
	return cmd.CombinedOutput()
}
