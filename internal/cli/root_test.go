package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelpAndLogsNeedNoBrowserOrCredentials(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"run", "--help"}, {"logs", "--log-file", filepath.Join(t.TempDir(), "missing")}} {
		command := NewCommand()
		var out bytes.Buffer
		command.SetOut(&out)
		command.SetErr(&out)
		command.SetArgs(args)
		if err := command.Execute(); err != nil || out.Len() == 0 {
			t.Fatalf("%v: output=%q, err=%v", args, out.String(), err)
		}
	}
}

func TestRejectInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"run", "--interval", "0s"}, {"status", "--mode", "unknown"}, {"login", "extra"}, {"status", "--check-url", "file:///etc/passwd"}} {
		command := NewCommand()
		command.SetArgs(args)
		command.SetIn(strings.NewReader(""))
		if err := command.Execute(); err == nil {
			t.Fatalf("accepted invalid args: %v", args)
		}
	}
}
