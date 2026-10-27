// Package config loads and validates the server's settings from environment
// variables. Every location the server touches comes from here.
package config

import (
	"errors"
	"fmt"
	netmail "net/mail"
	"os"
	"path/filepath"
	"strconv"

	"mirror/server/internal/mail"
)

// Env names the environment the server runs in.
type Env string

const (
	EnvDevelopment Env = "development"
	EnvProduction  Env = "production"
)

// Config holds the validated settings.
type Config struct {
	// Env is APP_ENV: development (text logs) or production (JSON logs).
	Env Env
	// Port is API_PORT, the port the API listens on.
	Port int
	// DataDir is DATA_DIR as an absolute path. It exists and is writable.
	DataDir string
	// WebDistDir is WEB_DIST_DIR as an absolute path, or empty when unset.
	// When set, it is a folder holding the built web app's index.html.
	WebDistDir string
	// DatabaseURL is DATABASE_URL, the Postgres connection string. Required.
	DatabaseURL string
	// SMTP holds SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD and
	// MAIL_FROM. An empty Host means emails go to the log (not allowed in
	// production).
	SMTP mail.SMTPConfig
}

// Defaults for unset variables.
const (
	defaultEnv     = EnvDevelopment
	defaultPort    = 8080
	defaultDataDir = "data"
	defaultSMTP    = 587
)

// Load reads the settings through getenv (os.Getenv in production) and
// validates them. Relative paths resolve against the working directory.
// It creates DATA_DIR if it is missing. All problems are reported together.
func Load(getenv func(string) string) (Config, error) {
	var cfg Config
	var errs []error
	var err error

	cfg.Env = Env(valueOr(getenv("APP_ENV"), string(defaultEnv)))
	if cfg.Env != EnvDevelopment && cfg.Env != EnvProduction {
		errs = append(errs, fmt.Errorf("APP_ENV must be %q or %q, got %q", EnvDevelopment, EnvProduction, cfg.Env))
	}

	cfg.Port, err = parsePort("API_PORT", getenv("API_PORT"), defaultPort)
	if err != nil {
		errs = append(errs, err)
	}

	dataDir, err := prepareDataDir(valueOr(getenv("DATA_DIR"), defaultDataDir))
	if err != nil {
		errs = append(errs, err)
	}
	cfg.DataDir = dataDir

	if v := getenv("WEB_DIST_DIR"); v != "" {
		webDist, err := checkWebDistDir(v)
		if err != nil {
			errs = append(errs, err)
		}
		cfg.WebDistDir = webDist
	}

	cfg.DatabaseURL = getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required (see .env.example)"))
	}

	cfg.SMTP, err = loadSMTP(getenv, cfg.Env)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration:\n%w", errors.Join(errs...))
	}
	return cfg, nil
}

func parsePort(name, value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fallback, fmt.Errorf("%s must be a port number from 1 to 65535, got %q", name, value)
	}
	return port, nil
}

// loadSMTP reads the SMTP settings. They are all required once SMTP_HOST is
// set, and SMTP_HOST is required in production.
func loadSMTP(getenv func(string) string, env Env) (mail.SMTPConfig, error) {
	cfg := mail.SMTPConfig{
		Host:     getenv("SMTP_HOST"),
		Username: getenv("SMTP_USERNAME"),
		Password: getenv("SMTP_PASSWORD"),
		From:     getenv("MAIL_FROM"),
	}
	if cfg.Host == "" {
		if env == EnvProduction {
			return cfg, errors.New("SMTP_HOST is required in production")
		}
		return cfg, nil
	}

	var errs []error
	port, err := parsePort("SMTP_PORT", getenv("SMTP_PORT"), defaultSMTP)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.Port = port
	if cfg.Username == "" || cfg.Password == "" {
		errs = append(errs, errors.New("SMTP_USERNAME and SMTP_PASSWORD are required when SMTP_HOST is set"))
	}
	if _, err := netmail.ParseAddress(cfg.From); err != nil {
		errs = append(errs, fmt.Errorf("MAIL_FROM must be an address like \"Mirror <you@example.com>\", got %q", cfg.From))
	}
	return cfg, errors.Join(errs...)
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// prepareDataDir resolves DATA_DIR, creates it if missing, and checks that
// the server can write there.
func prepareDataDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("DATA_DIR %q cannot be resolved: %w", dir, err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return abs, fmt.Errorf("DATA_DIR %q cannot be created: %w", abs, err)
	}
	probe, err := os.CreateTemp(abs, ".write-check-*")
	if err != nil {
		return abs, fmt.Errorf("DATA_DIR %q is not writable: %w", abs, err)
	}
	probe.Close()
	os.Remove(probe.Name())
	return abs, nil
}

// checkWebDistDir resolves WEB_DIST_DIR and checks that it holds a built app.
func checkWebDistDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("WEB_DIST_DIR %q cannot be resolved: %w", dir, err)
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return abs, fmt.Errorf("WEB_DIST_DIR %q is not a folder", abs)
	}
	if _, err := os.Stat(filepath.Join(abs, "index.html")); err != nil {
		return abs, fmt.Errorf("WEB_DIST_DIR %q has no index.html; build the web app first", abs)
	}
	return abs, nil
}
