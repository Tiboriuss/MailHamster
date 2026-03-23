// Package rewrite modifies RFC 5322 message headers before relaying.
package rewrite

import (
	"bytes"
	"fmt"
	"io"
	"net/mail"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"

	"github.com/Tiboriuss/MailHamster/internal/config"
)

// Rewrite reads an RFC 5322 message from r and returns the (possibly modified)
// message bytes. If rewriting is disabled the input is returned verbatim.
func Rewrite(r io.Reader, cfg *config.Config) ([]byte, error) {
	if !cfg.Rewrite.Enabled {
		return io.ReadAll(r)
	}

	entity, err := message.Read(r)
	if err != nil && !message.IsUnknownCharset(err) && !message.IsUnknownEncoding(err) {
		return nil, fmt.Errorf("parse message: %w", err)
	}

	addr := &mail.Address{
		Name:    cfg.Rewrite.FromName,
		Address: cfg.Rewrite.From,
	}
	formatted := addr.String()

	entity.Header.Set("From", formatted)
	if entity.Header.Get("Sender") != "" {
		entity.Header.Set("Sender", formatted)
	}

	var buf bytes.Buffer
	if err := entity.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("serialize message: %w", err)
	}
	return buf.Bytes(), nil
}
