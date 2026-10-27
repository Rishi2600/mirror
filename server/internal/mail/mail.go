// Package mail sends email, by SMTP or (in development) to the log.
package mail

import (
	"context"
	"log/slog"
	"time"

	gomail "github.com/wneessen/go-mail"
)

// Mailer sends one plain-text email.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// SMTPConfig holds the SMTP settings from the environment.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// SMTP sends email through an SMTP server.
type SMTP struct {
	client *gomail.Client
	from   string
}

func NewSMTP(cfg SMTPConfig) (*SMTP, error) {
	opts := []gomail.Option{
		gomail.WithPort(cfg.Port),
		gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
		gomail.WithUsername(cfg.Username),
		gomail.WithPassword(cfg.Password),
		gomail.WithTimeout(15 * time.Second),
	}
	// Port 465 is TLS from the start; other ports must upgrade with STARTTLS.
	if cfg.Port == 465 {
		opts = append(opts, gomail.WithSSL())
	} else {
		opts = append(opts, gomail.WithTLSPortPolicy(gomail.TLSMandatory))
	}
	client, err := gomail.NewClient(cfg.Host, opts...)
	if err != nil {
		return nil, err
	}
	return &SMTP{client: client, from: cfg.From}, nil
}

func (m *SMTP) Send(ctx context.Context, to, subject, body string) error {
	msg := gomail.NewMsg()
	if err := msg.From(m.from); err != nil {
		return err
	}
	if err := msg.To(to); err != nil {
		return err
	}
	msg.Subject(subject)
	msg.SetBodyString(gomail.TypeTextPlain, body)
	return m.client.DialAndSendWithContext(ctx, msg)
}

// Log writes emails to the log instead of sending them.
type Log struct {
	Logger *slog.Logger
}

func (m Log) Send(ctx context.Context, to, subject, body string) error {
	m.Logger.Info("email not sent (SMTP_HOST is empty)", "to", to, "subject", subject, "body", body)
	return nil
}
