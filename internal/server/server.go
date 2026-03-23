// Package server implements the MailHamster SMTP listener backend.
package server

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"io"
	"log/slog"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"

	"github.com/Tiboriuss/MailHamster/internal/config"
	"github.com/Tiboriuss/MailHamster/internal/relay"
	"github.com/Tiboriuss/MailHamster/internal/rewrite"
)

// Backend implements gosmtp.Backend.
type Backend struct {
	cfg    *config.Config
	logger *slog.Logger
}

// New returns a configured Backend.
func New(cfg *config.Config, logger *slog.Logger) *Backend {
	return &Backend{cfg: cfg, logger: logger}
}

// NewSession creates a new per-connection session.
func (b *Backend) NewSession(c *gosmtp.Conn) (gosmtp.Session, error) {
	return &session{cfg: b.cfg, logger: b.logger}, nil
}

// session holds per-connection state and implements gosmtp.Session + gosmtp.AuthSession.
type session struct {
	cfg      *config.Config
	logger   *slog.Logger
	username string
	authed   bool
	from     string
	rcpts    []string
}

// AuthMechanisms advertises supported SASL mechanisms (PLAIN).
// Implementing this interface causes go-smtp to include AUTH in EHLO.
func (s *session) AuthMechanisms() []string {
	return []string{sasl.Plain}
}

// Auth returns a SASL server handler for the requested mechanism.
func (s *session) Auth(mech string) (sasl.Server, error) {
	if mech != sasl.Plain {
		return nil, &gosmtp.SMTPError{
			Code:         504,
			EnhancedCode: gosmtp.EnhancedCode{5, 5, 4},
			Message:      "unrecognized authentication type",
		}
	}
	return sasl.NewPlainServer(func(identity, username, password string) error {
		return s.authenticate(username, password)
	}), nil
}

func (s *session) authenticate(username, password string) error {
	for _, u := range s.cfg.Auth.Users {
		uMatch := subtle.ConstantTimeCompare([]byte(u.Username), []byte(username)) == 1
		pMatch := subtle.ConstantTimeCompare([]byte(u.Password), []byte(password)) == 1
		if uMatch && pMatch {
			s.username = username
			s.authed = true
			s.logger.Debug("authenticated", "user", username)
			return nil
		}
	}
	s.logger.Warn("authentication failed", "user", username)
	return &gosmtp.SMTPError{
		Code:         535,
		EnhancedCode: gosmtp.EnhancedCode{5, 7, 8},
		Message:      "authentication credentials invalid",
	}
}

// Mail is called when the client issues MAIL FROM.
func (s *session) Mail(from string, opts *gosmtp.MailOptions) error {
	if !s.authed {
		return &gosmtp.SMTPError{
			Code:         530,
			EnhancedCode: gosmtp.EnhancedCode{5, 7, 0},
			Message:      "authentication required",
		}
	}
	s.from = from
	s.logger.Debug("MAIL FROM", "from", from)
	return nil
}

// Rcpt is called for each RCPT TO command.
func (s *session) Rcpt(to string, opts *gosmtp.RcptOptions) error {
	s.rcpts = append(s.rcpts, to)
	s.logger.Debug("RCPT TO", "to", to)
	return nil
}

// Data is called when the client sends the message body.
func (s *session) Data(r io.Reader) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read message: %w", err)
	}

	msg, err := rewrite.Rewrite(bytes.NewReader(raw), s.cfg)
	if err != nil {
		s.logger.Warn("rewrite failed, relaying original message", "err", err)
		msg = raw
	}

	envelopeFrom := s.from
	if s.cfg.Rewrite.Enabled && s.cfg.Rewrite.From != "" {
		envelopeFrom = s.cfg.Rewrite.From
	}

	if err := relay.Send(msg, envelopeFrom, s.rcpts, s.cfg); err != nil {
		s.logger.Error("relay failed",
			"user", s.username,
			"from", s.from,
			"envelope_from", envelopeFrom,
			"rcpts", s.rcpts,
			"err", err,
		)
		return &gosmtp.SMTPError{
			Code:         451,
			EnhancedCode: gosmtp.EnhancedCode{4, 4, 2},
			Message:      "relay failed, try again later",
		}
	}

	s.logger.Info("relayed",
		"user", s.username,
		"from", s.from,
		"envelope_from", envelopeFrom,
		"rcpts", s.rcpts,
		"bytes", len(msg),
	)
	return nil
}

// Reset discards the current mail transaction state.
func (s *session) Reset() {
	s.from = ""
	s.rcpts = nil
}

// Logout is called when the connection is closed.
func (s *session) Logout() error {
	return nil
}
