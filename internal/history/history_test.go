package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLegacyLogAndAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "login_log.jsonl")
	legacy := `{"timestamp":"2026-10-23 10:00:00","last_login":null,"elapsed_min":null}
{"timestamp":"2026-10-23 11:30:00","last_login":"2026-10-23 10:00:00","elapsed_min":null}
`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := Read(path)
	if err != nil || len(entries) != 2 || entries[1].ElapsedMin == nil || *entries[1].ElapsedMin != 90 {
		t.Fatalf("legacy log failed: %v, %+v", err, entries)
	}
	last, err := Last(path)
	if err != nil || last.Format(TimeLayout) != "2026-10-23 11:30:00" {
		t.Fatalf("last login failed: %v", err)
	}
	if err := Append(path, NewEntry(last.Add(time.Hour), last)); err != nil {
		t.Fatal(err)
	}
	entries, err = Read(path)
	if err != nil || len(entries) != 3 || *entries[2].ElapsedMin != 60 {
		t.Fatalf("append failed: %v", err)
	}
	if Pretty(entries[1].ElapsedMin) != "1 小时 30 分钟" || Pretty(nil) != "-" {
		t.Fatal("unexpected formatted interval")
	}
}

func TestMissingAndInvalidLogs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.jsonl")
	if entries, err := Read(path); err != nil || len(entries) != 0 {
		t.Fatalf("missing log should be empty: %v", err)
	}
	if err := os.WriteFile(path, []byte("\n{broken json}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil || !strings.Contains(err.Error(), "2") {
		t.Fatalf("invalid log should report its line: %v", err)
	}
}
