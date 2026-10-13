package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "version", args: []string{"version"}, wantCode: exitSuccess, wantStdout: "defold-vet dev\n"},
		{name: "version flag", args: []string{"--version"}, wantCode: exitSuccess, wantStdout: "defold-vet dev\n"},
		{name: "help goes to stdout", args: []string{"help"}, wantCode: exitSuccess, wantStdout: "Usage:"},
		{name: "no command is a usage error", args: nil, wantCode: exitCouldNotRun, wantStderr: "Usage:"},
		{name: "unknown command is a usage error", args: []string{"chekc"}, wantCode: exitCouldNotRun, wantStderr: `unknown command "chekc"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			if got := Run(tt.args, strings.NewReader(""), &stdout, &stderr); got != tt.wantCode {
				t.Errorf("exit code = %d, want %d", got, tt.wantCode)
			}

			assertOutput(t, "stdout", stdout.String(), tt.wantStdout)
			assertOutput(t, "stderr", stderr.String(), tt.wantStderr)
		})
	}
}

// An empty want means the stream must stay silent: a command that succeeds
// writes nothing to stderr, and one that fails writes nothing to stdout.
func assertOutput(t *testing.T, stream, got, want string) {
	t.Helper()
	if want == "" {
		if got != "" {
			t.Errorf("%s = %q, want it empty", stream, got)
		}
		return
	}
	if !strings.Contains(got, want) {
		t.Errorf("%s = %q, want it to contain %q", stream, got, want)
	}
}
