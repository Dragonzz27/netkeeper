package portal

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestOnlineURL(t *testing.T) {
	for _, tc := range []struct {
		location string
		want     bool
	}{
		{"http://www.baidu.com/", true},
		{"https://www.baidu.com/search", true},
		{"http://www.baidu.com/srun_portal_pc.php", false},
		{"http://www.baidu.com/login", false},
		{"http://172.20.3.90:804/srun_portal_pc.php", false},
		{"http://www.baidu.com.example.org/", false},
		{"chrome-error://chromewebdata/", false},
		{"", false},
	} {
		if got := onlineURL("http://www.baidu.com", tc.location); got != tc.want {
			t.Errorf("onlineURL(%q) = %v, want %v", tc.location, got, tc.want)
		}
	}
}

func TestBrowserLogin(t *testing.T) {
	chrome := os.Getenv("ECUST_TEST_CHROME")
	if chrome == "" {
		t.Skip("set ECUST_TEST_CHROME to run local browser integration tests")
	}
	for _, tc := range []struct {
		name, mode, domain                   string
		domainSet, jsRedirect, wrongPassword bool
	}{
		{name: "campus", mode: "campus"},
		{name: "server-page-domain", mode: "server", domain: "@free", jsRedirect: true},
		{name: "server-explicit-empty-domain", mode: "server", domainSet: true},
		{name: "wrong-password", mode: "campus", wrongPassword: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var authenticated atomic.Bool
			var submitted atomic.Bool
			mux := http.NewServeMux()
			mux.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
				if authenticated.Load() {
					fmt.Fprint(w, "<html><body>Connected</body></html>")
					return
				}
				if tc.jsRedirect {
					fmt.Fprint(w, `<html><script>setTimeout(() => location.href = '/srun_portal', 100)</script></html>`)
				} else {
					http.Redirect(w, r, "/srun_portal", http.StatusFound)
				}
			})
			mux.HandleFunc("/srun_portal", func(w http.ResponseWriter, r *http.Request) {
				button := `<input type="submit" value="登录">`
				if tc.mode == "server" {
					button = `<button id="login" type="submit">登录</button>`
				}
				fmt.Fprintf(w, `<html><body><form method="POST" action="/authenticate">
				<input id="username" name="username"><input id="password" name="password" type="password">
				<input id="domain" name="domain" type="hidden" value="">
				<input type="radio" name="net-type" value="@free" checked>%s</form></body></html>`, button)
			})
			mux.HandleFunc("/authenticate", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					http.Error(w, "POST required", http.StatusMethodNotAllowed)
					return
				}
				submitted.Store(true)
				if r.FormValue("username") == "student" && r.FormValue("password") == "secret" && r.FormValue("domain") == tc.domain {
					authenticated.Store(true)
				}
				http.Redirect(w, r, "/probe", http.StatusSeeOther)
			})
			server := httptest.NewServer(mux)
			defer server.Close()
			b, err := New(Options{Mode: tc.mode, ChromePath: chrome, CheckURL: server.URL + "/probe", PortalURL: server.URL + "/srun_portal", Headless: true, NoSandbox: os.Getenv("ECUST_TEST_NO_SANDBOX") == "1", Timeout: 16 * time.Second, Domain: tc.domain, DomainSet: tc.domainSet})
			if err != nil {
				t.Fatal(err)
			}
			online, err := b.Check(context.Background())
			if err != nil || online {
				t.Fatalf("initial check: online=%v, err=%v", online, err)
			}
			password := "secret"
			if tc.wrongPassword {
				password = "wrong"
			}
			err = b.Login(context.Background(), "student", password)
			if !submitted.Load() {
				t.Fatal("login form was not submitted")
			}
			if (err != nil) != tc.wrongPassword {
				t.Fatalf("unexpected login result: %v", err)
			}
			online, err = b.Check(context.Background())
			if err != nil || online == tc.wrongPassword {
				t.Fatalf("final check: online=%v, err=%v", online, err)
			}
		})
	}
}

func TestBrowserCheckHTTPFailure(t *testing.T) {
	chrome := os.Getenv("ECUST_TEST_CHROME")
	if chrome == "" {
		t.Skip("set ECUST_TEST_CHROME to run local browser integration tests")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	b, err := New(Options{Mode: "campus", ChromePath: chrome, CheckURL: server.URL, PortalURL: server.URL, Headless: true, NoSandbox: os.Getenv("ECUST_TEST_NO_SANDBOX") == "1", Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if online, err := b.Check(context.Background()); online || err == nil {
		t.Fatalf("HTTP failure must not be online: online=%v, err=%v", online, err)
	}
}
