//go:build !windows

package sysproxy

import (
	"context"
	"testing"
	"time"
)

// One hung command must not eat the whole operation: each command gets its
// own short deadline, and the next one can still run.
func TestExecRunnerTimesOutEachCommand(t *testing.T) {
	old := commandTimeout
	commandTimeout = 30 * time.Millisecond
	t.Cleanup(func() { commandTimeout = old })

	start := time.Now()
	_, err := (execRunner{}).Run(context.Background(), "sh", "-c", "sleep 1")
	if err == nil {
		t.Fatal("a hung command reported success")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("the command ran for %v, want it cut off near the timeout", elapsed)
	}
}
