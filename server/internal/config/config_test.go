package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testDatabaseURL = "postgres://mirror:mirror@localhost:5432/mirror"

// envFrom returns a getenv over vars, with DATABASE_URL set unless vars
// sets it (to "" to test it missing).
func envFrom(vars map[string]string) func(string) string {
	return func(key string) string {
		if v, ok := vars[key]; ok {
			return v
		}
		if key == "DATABASE_URL" {
			return testDatabaseURL
		}
		return ""
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Chdir(t.TempDir())

	cfg, err := Load(envFrom(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Env != EnvDevelopment {
		t.Errorf("Env = %q, want %q", cfg.Env, EnvDevelopment)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if !filepath.IsAbs(cfg.DataDir) || filepath.Base(cfg.DataDir) != "data" {
		t.Errorf("DataDir = %q, want an absolute path ending in data", cfg.DataDir)
	}
	if info, err := os.Stat(cfg.DataDir); err != nil || !info.IsDir() {
		t.Errorf("DataDir was not created: %v", err)
	}
	if cfg.WebDistDir != "" {
		t.Errorf("WebDistDir = %q, want empty", cfg.WebDistDir)
	}
	if cfg.DatabaseURL != testDatabaseURL {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, testDatabaseURL)
	}
}

func TestLoadValidValues(t *testing.T) {
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(t.TempDir(), "nested", "data")

	cfg, err := Load(envFrom(map[string]string{
		"APP_ENV":      "production",
		"API_PORT":     "9090",
		"DATA_DIR":     dataDir,
		"WEB_DIST_DIR": dist,
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Env != EnvProduction || cfg.Port != 9090 || cfg.DataDir != dataDir || cfg.WebDistDir != dist {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	readOnly := t.TempDir()
	if err := os.Chmod(readOnly, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(readOnly, 0o700) })
	noIndex := t.TempDir()

	tests := []struct {
		name string
		vars map[string]string
		want string
	}{
		{"unknown env", map[string]string{"APP_ENV": "staging"}, "APP_ENV"},
		{"port not a number", map[string]string{"API_PORT": "abc"}, "API_PORT"},
		{"port zero", map[string]string{"API_PORT": "0"}, "API_PORT"},
		{"port too large", map[string]string{"API_PORT": "70000"}, "API_PORT"},
		{"data dir not writable", map[string]string{"DATA_DIR": filepath.Join(readOnly, "data")}, "DATA_DIR"},
		{"web dist missing", map[string]string{"WEB_DIST_DIR": filepath.Join(noIndex, "missing")}, "WEB_DIST_DIR"},
		{"web dist without index.html", map[string]string{"WEB_DIST_DIR": noIndex}, "WEB_DIST_DIR"},
		{"database url missing", map[string]string{"DATABASE_URL": ""}, "DATABASE_URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			_, err := Load(envFrom(tt.vars))
			if err == nil {
				t.Fatal("Load succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not name %s", err, tt.want)
			}
		})
	}
}
