package config

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

func TestResolvePrecedenceAndDoesNotRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	const original = "# keep this comment\nCAMPUS_USERNAME='file-user'\nCAMPUS_PASSWORD='file-password'\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CAMPUS_USERNAME", "env-user")
	t.Setenv("CAMPUS_PASSWORD", "env-password")
	c, err := Resolve(path, "flag-user", strings.NewReader(""), io.Discard)
	if err != nil || c.Username != "flag-user" || c.Password != "env-password" {
		t.Fatalf("unexpected credentials or error: %+v, %v", c, err)
	}
	c, err = Resolve(path, "", strings.NewReader(""), io.Discard)
	if err != nil || c.Username != "env-user" {
		t.Fatalf("environment precedence failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != original {
		t.Fatal("non-interactive credentials must not rewrite the file")
	}
}

func TestResolvePromptsAndPreservesOtherKeys(t *testing.T) {
	t.Setenv("CAMPUS_USERNAME", "")
	t.Setenv("CAMPUS_PASSWORD", "")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("OTHER='keep'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	const password = `p'a"ss$word\end`
	c, err := Resolve(path, "", strings.NewReader("student\n"+password+"\n"), io.Discard)
	if err != nil || c.Username != "student" || c.Password != password {
		t.Fatalf("prompt failed: %v", err)
	}
	values, err := godotenv.Read(path)
	if err != nil || values["CAMPUS_PASSWORD"] != password || values["OTHER"] != "keep" || values["CAMPUS_USERNAME"] != "student" {
		t.Fatalf("saved configuration did not round-trip: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("configuration permission = %o", info.Mode().Perm())
	}
}

func TestResolveMissingCredentialsEOF(t *testing.T) {
	t.Setenv("CAMPUS_USERNAME", "")
	t.Setenv("CAMPUS_PASSWORD", "")
	path := filepath.Join(t.TempDir(), ".env")
	if _, err := Resolve(path, "", strings.NewReader(""), io.Discard); err == nil {
		t.Fatal("expected error for missing noninteractive credentials")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("incomplete input must not save a configuration file")
	}
}
