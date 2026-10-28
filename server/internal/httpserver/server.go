// Package httpserver holds the routes, handlers and middleware.
package httpserver

import (
	"log/slog"
	"net/http"
	"time"

	"gorm.io/gorm"

	"mirror/server/internal/mail"
	"mirror/server/internal/storage"
)

// Options configure the handler.
type Options struct {
	Logger  *slog.Logger
	Version string
	// WebDistDir, when set, is the folder of the built web app to serve.
	WebDistDir string
	// Now returns the current time; it defaults to time.Now.
	Now    func() time.Time
	DB     *gorm.DB
	Mailer mail.Mailer
	// SecureCookies marks cookies HTTPS-only; true outside development.
	SecureCookies bool
	Files         *storage.Local
	CVQueue       CVQueue
}

type server struct {
	version       string
	now           func() time.Time
	logger        *slog.Logger
	db            *gorm.DB
	mailer        mail.Mailer
	secureCookies bool
	files         *storage.Local
	cvQueue       CVQueue
}

// New returns the server's root handler.
func New(opts Options) http.Handler {
	s := &server{
		version:       opts.Version,
		now:           opts.Now,
		logger:        opts.Logger,
		db:            opts.DB,
		mailer:        opts.Mailer,
		secureCookies: opts.SecureCookies,
		files:         opts.Files,
		cvQueue:       opts.CVQueue,
	}
	if s.now == nil {
		s.now = time.Now
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/healthz", s.health)
	mux.HandleFunc("POST /v1/auth/otp/request", s.requestCode)
	mux.HandleFunc("POST /v1/auth/otp/verify", s.verifyCode)
	mux.HandleFunc("POST /v1/auth/logout", s.logout)
	mux.HandleFunc("GET /v1/me", s.me)
	mux.HandleFunc("POST /v1/me/role", s.setRole)
	mux.HandleFunc("POST /v1/profile/cv", s.uploadCV)
	mux.HandleFunc("GET /v1/profile", s.getProfile)
	mux.HandleFunc("PUT /v1/profile", s.updateProfile)
	mux.HandleFunc("POST /v1/profile/photo", s.uploadPhoto)
	mux.HandleFunc("GET /v1/profile/photo", s.getPhoto)
	mux.HandleFunc("GET /v1/card", s.getCard)
	mux.HandleFunc("POST /v1/card/finish", s.finishCard)
	// Unknown API paths get a JSON 404, never the web app.
	mux.HandleFunc("/v1/", s.notFound)
	if opts.WebDistDir != "" {
		mux.Handle("/", webApp(opts.WebDistDir))
	}

	// Recovery sits inside logging, so a recovered panic is logged as a 500.
	return logRequests(opts.Logger, recoverPanics(opts.Logger, mux))
}
