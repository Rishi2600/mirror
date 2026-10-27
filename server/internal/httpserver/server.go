// Package httpserver wires the API routes, the optional web app and the
// middleware into one http.Handler.
package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"mirror/server/api"
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

// New returns the server's root handler.
func New(opts Options) http.Handler {
	if opts.Now == nil {
		opts.Now = time.Now
	}

	mux := http.NewServeMux()
	strict := api.NewStrictHandlerWithOptions(&handlers{version: opts.Version, now: opts.Now}, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeError(w, http.StatusBadRequest, err.Error())
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			opts.Logger.Error("writing response", "path", r.URL.Path, "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		},
	})
	api.HandlerWithOptions(strict, api.StdHTTPServerOptions{
		BaseRouter: mux,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeError(w, http.StatusBadRequest, err.Error())
		},
	})

	// Any other /v1 path is an unknown API route: answer 404 in JSON and
	// never fall through to the web app.
	mux.HandleFunc("/v1/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	if opts.WebDistDir != "" {
		mux.Handle("/", webApp(opts.WebDistDir))
	}

	// Recovery sits inside logging, so a recovered panic is logged as a 500.
	return logRequests(opts.Logger, recoverPanics(opts.Logger, mux))
}

// writeError writes a JSON error body.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
