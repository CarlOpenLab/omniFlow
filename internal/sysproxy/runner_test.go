package sysproxy

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// fakeRunner records commands and answers the reads a test describes. Writes
// succeed with no output unless a test injects a failure.
type fakeRunner struct {
	outputs map[string]string
	errs    map[string]error
	calls   [][]string
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{outputs: map[string]string{}, errs: map[string]error{}}
}

func (f *fakeRunner) set(args []string, output string) {
	f.outputs[commandKey(args)] = output
}

func (f *fakeRunner) fail(args []string, err error) {
	f.errs[commandKey(args)] = err
}

func commandKey(args []string) string { return strings.Join(args, "\x00") }

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	f.calls = append(f.calls, call)
	key := commandKey(call)
	if err := f.errs[key]; err != nil {
		return nil, err
	}
	if out, ok := f.outputs[key]; ok {
		return []byte(out), nil
	}
	if len(args) > 0 && (strings.HasPrefix(args[0], "-get") || strings.HasPrefix(args[0], "-list")) {
		return nil, fmt.Errorf("unexpected read command %v", call)
	}
	return nil, nil
}

func (f *fakeRunner) callsStarting(prefix ...string) [][]string {
	var out [][]string
	for _, c := range f.calls {
		if len(c) < len(prefix)+1 {
			continue
		}
		match := true
		for i, p := range prefix {
			if c[i+1] != p {
				match = false
				break
			}
		}
		if match {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeRunner) resetCalls() { f.calls = nil }

func assertCalls(t *testing.T, got, want [][]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("calls:\n%s\nwant:\n%s", formatCalls(got), formatCalls(want))
	}
	for i := range want {
		if strings.Join(got[i], " ") != strings.Join(want[i], " ") {
			t.Fatalf("call %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func formatCalls(calls [][]string) string {
	var b strings.Builder
	for _, c := range calls {
		b.WriteString(strings.Join(c, " "))
		b.WriteByte('\n')
	}
	return b.String()
}
