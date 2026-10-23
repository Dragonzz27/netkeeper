package runner

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	"ecust-network-auto-login/internal/config"
	"ecust-network-auto-login/internal/history"
)

type fakePortal struct {
	online   bool
	checkErr error
	loginErr error
	logins   int
	onLogin  func()
}

func (p *fakePortal) Check(context.Context) (bool, error) { return p.online, p.checkErr }
func (p *fakePortal) Login(_ context.Context, username, password string) error {
	p.logins++
	if username != "student" || password != "secret" {
		return errors.New("unexpected credentials")
	}
	if p.onLogin != nil {
		p.onLogin()
	}
	return p.loginErr
}

func TestOnceRecordsOnlyVerifiedLogins(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		online                  bool
		checkErr, loginErr      error
		wantLogins, wantEntries int
	}{
		{name: "already online", online: true},
		{name: "offline", wantLogins: 1, wantEntries: 1},
		{name: "check failed", checkErr: errors.New("network error"), wantLogins: 1, wantEntries: 1},
		{name: "login failed", loginErr: errors.New("bad password"), wantLogins: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &fakePortal{online: tc.online, checkErr: tc.checkErr, loginErr: tc.loginErr}
			last := time.Now().Add(-time.Hour)
			r := Runner{Portal: p, Credentials: config.Credentials{Username: "student", Password: "secret"}, LogFile: filepath.Join(t.TempDir(), "log"), Out: io.Discard, LastLogin: &last}
			err := r.Once(context.Background())
			if (err != nil) != (tc.loginErr != nil) || p.logins != tc.wantLogins {
				t.Fatalf("got %d logins, err=%v", p.logins, err)
			}
			entries, err := history.Read(r.LogFile)
			if err != nil || len(entries) != tc.wantEntries {
				t.Fatalf("got %d entries, err=%v", len(entries), err)
			}
			if len(entries) > 0 && (entries[0].ElapsedMin == nil || *entries[0].ElapsedMin < 60) {
				t.Fatal("previous login was not used")
			}
		})
	}
}

func TestRunRetriesAndCancelsPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &fakePortal{loginErr: errors.New("temporary failure")}
	p.onLogin = func() {
		if p.logins == 2 {
			cancel()
		}
	}
	r := Runner{Portal: p, Credentials: config.Credentials{Username: "student", Password: "secret"}, LogFile: filepath.Join(t.TempDir(), "log"), Out: io.Discard}
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx, time.Millisecond) }()
	select {
	case err := <-done:
		if err != nil || p.logins != 2 {
			t.Fatalf("retry/cancellation failed: %v, logins=%d", err, p.logins)
		}
	case <-time.After(time.Second):
		t.Fatal("runner did not stop")
	}
}
