package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testDatabaseURL = "postgres://mirror:mirror@localhost:5432/mirror"

// requiredDefaults are valid values for the required settings.
var requiredDefaults = map[string]string{
	"DATABASE_URL":        testDatabaseURL,
	"GROQ_API_KEY":        "gsk_test",
	"FILE_ENCRYPTION_KEY": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	"FILE_SIGNING_KEY":    "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=",
}

// envFrom returns a getenv over vars, falling back to requiredDefaults. Set
// a key to "" in vars to test it missing.
func envFrom(vars map[string]string) func(string) string {
	return func(key string) string {
		if v, ok := vars[key]; ok {
			return v
		}
		return requiredDefaults[key]
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
	if cfg.GroqModel != "openai/gpt-oss-120b" || len(cfg.FileKey) != 32 {
		t.Errorf("GroqModel = %q, FileKey = %d bytes; want the default model and 32 bytes", cfg.GroqModel, len(cfg.FileKey))
	}
}

func TestLoadValidValues(t *testing.T) {
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(t.TempDir(), "nested", "data")

	cfg, err := Load(envFrom(map[string]string{
		"APP_ENV":       "production",
		"API_PORT":      "9090",
		"DATA_DIR":      dataDir,
		"WEB_DIST_DIR":  dist,
		"SMTP_HOST":     "smtp.example.com",
		"SMTP_USERNAME": "mailer",
		"SMTP_PASSWORD": "secret",
		"MAIL_FROM":     "Mirror <mirror@example.com>",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Env != EnvProduction || cfg.Port != 9090 || cfg.DataDir != dataDir || cfg.WebDistDir != dist {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if cfg.SMTP.Host != "smtp.example.com" || cfg.SMTP.Port != 587 {
		t.Errorf("SMTP = %+v, want host smtp.example.com and default port 587", cfg.SMTP)
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
		{"production without smtp", map[string]string{"APP_ENV": "production"}, "SMTP_HOST"},
		{"groq key missing", map[string]string{"GROQ_API_KEY": ""}, "GROQ_API_KEY"},
		{"file key missing", map[string]string{"FILE_ENCRYPTION_KEY": ""}, "FILE_ENCRYPTION_KEY"},
		{"file key not base64", map[string]string{"FILE_ENCRYPTION_KEY": "not base64!"}, "FILE_ENCRYPTION_KEY"},
		{"file key too short", map[string]string{"FILE_ENCRYPTION_KEY": "c2hvcnQ="}, "FILE_ENCRYPTION_KEY"},
		{"signing key missing", map[string]string{"FILE_SIGNING_KEY": ""}, "FILE_SIGNING_KEY"},
		{"smtp without login", map[string]string{"SMTP_HOST": "smtp.example.com", "MAIL_FROM": "a@example.com"}, "SMTP_USERNAME"},
		{"smtp with bad from", map[string]string{"SMTP_HOST": "smtp.example.com", "SMTP_USERNAME": "u", "SMTP_PASSWORD": "p", "MAIL_FROM": "not an address"}, "MAIL_FROM"},
		{"smtp with bad port", map[string]string{"SMTP_HOST": "smtp.example.com", "SMTP_PORT": "x", "SMTP_USERNAME": "u", "SMTP_PASSWORD": "p", "MAIL_FROM": "a@example.com"}, "SMTP_PORT"},
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
