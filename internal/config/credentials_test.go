package config

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePrecedence(t *testing.T) {
	t.Setenv("CAMPUS_USERNAME", "env-user")
	t.Setenv("CAMPUS_PASSWORD", "env-password")
	c, err := Resolve("flag-user", strings.NewReader(""), io.Discard)
	if err != nil || c.Username != "flag-user" || c.Password != "env-password" {
		t.Fatalf("unexpected credentials or error: %v", err)
	}
	c, err = Resolve("", strings.NewReader(""), io.Discard)
	if err != nil || c.Username != "env-user" || c.Password != "env-password" {
		t.Fatalf("environment precedence failed: %v", err)
	}
}

func TestResolvePromptsWithoutSaving(t *testing.T) {
	t.Setenv("CAMPUS_USERNAME", "")
	t.Setenv("CAMPUS_PASSWORD", "")
	dir := t.TempDir()
	t.Chdir(dir)
	const password = `p'a"ss$word\end`
	c, err := Resolve("", strings.NewReader("student\n"+password+"\n"), io.Discard)
	if err != nil || c.Username != "student" || c.Password != password {
		t.Fatalf("prompt failed: %v", err)
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatal("interactive credentials must not create files")
	}
}

func TestResolveMissingCredentialsIgnoresDotEnv(t *testing.T) {
	t.Setenv("CAMPUS_USERNAME", "")
	t.Setenv("CAMPUS_PASSWORD", "")
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, ".env")
	const original = "CAMPUS_USERNAME='file-user'\nCAMPUS_PASSWORD='file-password'\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve("", strings.NewReader(""), io.Discard); err == nil || !strings.Contains(err.Error(), "CAMPUS_USERNAME") {
		t.Fatal("expected error for missing noninteractive credentials")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != original {
		t.Fatal("existing configuration file must remain untouched")
	}
}
