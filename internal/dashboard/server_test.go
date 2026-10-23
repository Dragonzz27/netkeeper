package dashboard

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlerLegacyChartAndEmptyLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log")
	handler := Handler(path)
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		return w
	}
	if w := request(); w.Code != 200 || !strings.Contains(w.Body.String(), "暂无登录日志") {
		t.Fatalf("empty page failed: %d", w.Code)
	}
	legacy := `{"timestamp":"2026-10-23 12:00:00","last_login":"2026-10-23 10:00:00","elapsed_min":null}`
	if err := os.WriteFile(path, []byte(legacy+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	w := request()
	if w.Code != 200 || !strings.Contains(w.Body.String(), "2 小时") || !strings.Contains(w.Body.String(), "<polyline") {
		t.Fatalf("legacy chart failed: %s", w.Body.String())
	}
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if w := request(); w.Code != 500 {
		t.Fatalf("invalid log returned status %d", w.Code)
	}
}
