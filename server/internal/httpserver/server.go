// Package httpserver holds the routes, handlers and middleware.
package httpserver

import (
	"log/slog"
	"net/http"
	"time"
)

// Options configure the handler.
type Options struct {
	Logger  *slog.Logger
	Version string
	// WebDistDir, when set, is the folder of the built web app to serve.
	WebDistDir string
	// Now returns the current time; it defaults to time.Now.
	Now func() time.Time
}

type server struct {
	version string
	now     func() time.Time
}

// New returns the server's root handler.
func New(opts Options) http.Handler {
	s := &server{version: opts.Version, now: opts.Now}
	if s.now == nil {
		s.now = time.Now
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/healthz", s.health)
	// Unknown API paths get a JSON 404, never the web app.
	mux.HandleFunc("/v1/", s.notFound)
	if opts.WebDistDir != "" {
		mux.Handle("/", webApp(opts.WebDistDir))
	}

	// Recovery sits inside logging, so a recovered panic is logged as a 500.
	return logRequests(opts.Logger, recoverPanics(opts.Logger, mux))
}
