// Package relay sends a message to the configured upstream SMTP server.
// It supports three TLS modes: none (plain), starttls, and tls (implicit).
package relay

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"github.com/Tiboriuss/MailHamster/internal/config"
)

// Send delivers msg (RFC 5322 bytes) to the configured upstream server.
// envelopeFrom is the MAIL FROM address; rcpts are the RCPT TO addresses.
func Send(msg []byte, envelopeFrom string, rcpts []string, cfg *config.Config) error {
	if len(rcpts) == 0 {
		return errors.New("no recipients")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Upstream.Host, cfg.Upstream.Port)

	c, err := dial(addr, cfg)
	if err != nil {
		return err
	}
	defer c.Close()

	if cfg.Upstream.Username != "" {
		auth := &plainAuth{
			username: cfg.Upstream.Username,
			password: cfg.Upstream.Password,
		}
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("upstream auth: %w", err)
		}
	}

	if err := c.Mail(envelopeFrom); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	for _, to := range rcpts {
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("RCPT TO %s: %w", to, err)
		}
	}

	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := wc.Write(msg); err != nil {
		wc.Close()
		return fmt.Errorf("write message: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("end DATA: %w", err)
	}

	return c.Quit()
}

func dial(addr string, cfg *config.Config) (*smtp.Client, error) {
	switch cfg.Upstream.TLS {
	case "tls":
		tlsCfg := &tls.Config{ServerName: cfg.Upstream.Host}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return nil, fmt.Errorf("tls dial %s: %w", addr, err)
		}
		c, err := smtp.NewClient(conn, cfg.Upstream.Host)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("smtp client (tls): %w", err)
		}
		return c, nil

	case "starttls":
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("dial %s: %w", addr, err)
		}
		c, err := smtp.NewClient(conn, cfg.Upstream.Host)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("smtp client (starttls): %w", err)
		}
		tlsCfg := &tls.Config{ServerName: cfg.Upstream.Host}
		if err := c.StartTLS(tlsCfg); err != nil {
			c.Close()
			return nil, fmt.Errorf("STARTTLS: %w", err)
		}
		return c, nil

	default:
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("dial %s: %w", addr, err)
		}
		c, err := smtp.NewClient(conn, cfg.Upstream.Host)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("smtp client (none): %w", err)
		}
		return c, nil
	}
}

// plainAuth implements smtp.Auth using SASL PLAIN without the TLS-required
// check from net/smtp.PlainAuth. The user explicitly chose a TLS mode via dial().
type plainAuth struct {
	username string
	password string
}

func (a *plainAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	resp := "\x00" + a.username + "\x00" + a.password
	return "PLAIN", []byte(resp), nil
}

func (a *plainAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		return nil, fmt.Errorf("unexpected PLAIN challenge: %s", strings.TrimSpace(string(fromServer)))
	}
	return nil, nil
}
