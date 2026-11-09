package sysproxy

import (
	"context"
	"os/exec"
	"time"
)

// commandTimeout is how long one system command may run. It is per command,
// so a tool that hangs cannot spend the whole operation's budget and stop the
// commands after it from running.
var commandTimeout = 5 * time.Second

// runner runs one system command. Tests replace it to watch the commands a
// platform would run.
type runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// execRunner runs the commands for real. LC_ALL=C keeps networksetup's output
// in the one language the parsers know.
type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(cmd.Environ(), "LC_ALL=C")
	return cmd.CombinedOutput()
}
