package httpserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const indexHTML = "<!doctype html><title>Mirror</title>"

func newTestServer(t *testing.T, webDistDir string) http.Handler {
	t.Helper()
	return New(Options{
		Logger:     slog.New(slog.DiscardHandler),
		Version:    "test-version",
		WebDistDir: webDistDir,
		Now:        func() time.Time { return time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC) },
	})
}

func newWebDist(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.html":           indexHTML,
		"assets/app.js":        "console.log('app')",
		"manifest.webmanifest": "{}",
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealth(t *testing.T) {
	rec := get(t, newTestServer(t, ""), "/v1/healthz")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got Health
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	want := Health{Status: "ok", Version: "test-version", Time: "2026-10-27T12:00:00Z"}
	if got != want {
		t.Errorf("body = %+v, want %+v", got, want)
	}
}

func TestUnknownAPIPathIsNotFound(t *testing.T) {
	h := newTestServer(t, newWebDist(t))

	for _, path := range []string{"/v1/nope", "/v1/", "/v1/healthz/extra"} {
		rec := get(t, h, path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", path, rec.Code)
		}
		if strings.Contains(rec.Body.String(), indexHTML) {
			t.Errorf("%s: served index.html", path)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("%s: Content-Type = %q, want application/json", path, ct)
		}
	}
}

func TestStaticFallback(t *testing.T) {
	h := newTestServer(t, newWebDist(t))

	tests := []struct {
		path string
		want string
	}{
		{"/", indexHTML},
		{"/some/route", indexHTML},
		{"/jobs/123/feed", indexHTML},
		{"/assets/missing.js", indexHTML},
		{"/assets/app.js", "console.log('app')"},
		{"/manifest.webmanifest", "{}"},
	}
	for _, tt := range tests {
		rec := get(t, h, tt.path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", tt.path, rec.Code)
			continue
		}
		body, _ := io.ReadAll(rec.Body)
		if string(body) != tt.want {
			t.Errorf("%s: body = %q, want %q", tt.path, body, tt.want)
		}
	}
}

func TestNoWebAppWhenUnset(t *testing.T) {
	rec := get(t, newTestServer(t, ""), "/some/route")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when WEB_DIST_DIR is unset", rec.Code)
	}
}

func TestPanicBecomes500(t *testing.T) {
	h := recoverPanics(slog.New(slog.DiscardHandler), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rec := get(t, h, "/v1/healthz")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
