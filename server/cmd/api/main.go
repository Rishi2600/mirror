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
	"strconv"
	"syscall"
	"time"

	"mirror/server/internal/config"
	"mirror/server/internal/httpserver"
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

	srv := &http.Server{
		Addr: net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		Handler: httpserver.New(httpserver.Options{
			Logger:     logger,
			Version:    version,
			WebDistDir: cfg.WebDistDir,
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
	if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Info("stopped")
	return nil
}

// newLogger logs readable text in development and JSON elsewhere.
func newLogger(env config.Env) *slog.Logger {
	if env == config.EnvDevelopment {
		return slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}
