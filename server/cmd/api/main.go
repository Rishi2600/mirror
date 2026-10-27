// Command api runs the Mirror API server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"mirror/server/internal/ai"
	"mirror/server/internal/config"
	"mirror/server/internal/db"
	"mirror/server/internal/httpserver"
	"mirror/server/internal/jobs"
	"mirror/server/internal/mail"
	"mirror/server/internal/storage"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const shutdownTimeout = 15 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	logger := newLogger(cfg.Env)
	slog.SetDefault(logger)

	conn, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to the database (is Postgres running? try make db-up): %w", err)
	}
	defer db.Close(conn)
	applied, err := db.Migrate(context.Background(), conn)
	if err != nil {
		return err
	}
	logger.Info("database ready", "migrations_applied", applied)

	mailer, err := newMailer(cfg.SMTP, logger)
	if err != nil {
		return err
	}

	files, err := storage.NewLocal(filepath.Join(cfg.DataDir, "files"), cfg.FileKey)
	if err != nil {
		return err
	}
	parser := ai.NewGroq(cfg.GroqAPIKey, cfg.GroqModel, conn, logger)
	queue, err := jobs.Start(context.Background(), cfg.DatabaseURL, conn, files, parser, logger)
	if err != nil {
		return fmt.Errorf("starting background jobs: %w", err)
	}

	srv := &http.Server{
		Addr: net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		Handler: httpserver.New(httpserver.Options{
			Logger:        logger,
			Version:       version,
			WebDistDir:    cfg.WebDistDir,
			DB:            conn,
			Mailer:        mailer,
			SecureCookies: cfg.Env != config.EnvDevelopment,
			Files:         files,
			CVQueue:       queue,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		logger.Info("api listening",
			"addr", srv.Addr,
			"version", version,
			"env", cfg.Env,
			"data_dir", cfg.DataDir,
			"web_dist_dir", cfg.WebDistDir,
		)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return fmt.Errorf("server stopped: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := queue.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("stopping background jobs: %w", err)
	}
	if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Info("stopped")
	return nil
}

// newMailer sends by SMTP when SMTP_HOST is set, and to the log otherwise.
func newMailer(cfg mail.SMTPConfig, logger *slog.Logger) (mail.Mailer, error) {
	if cfg.Host == "" {
		logger.Warn("SMTP_HOST is empty: sign-in codes go to this log, not by email")
		return mail.Log{Logger: logger}, nil
	}
	logger.Info("sending email by SMTP", "host", cfg.Host, "port", cfg.Port)
	return mail.NewSMTP(cfg)
}

// newLogger logs readable text in development and JSON elsewhere.
func newLogger(env config.Env) *slog.Logger {
	if env == config.EnvDevelopment {
		return slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}
