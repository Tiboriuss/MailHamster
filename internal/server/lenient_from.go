// Package server implements the MailHamster SMTP listener backend.
package server

import (
	"bufio"
	"bytes"
	"net"
	"regexp"
	"strings"
)

// lenientListener wraps a net.Listener and returns lenientConn connections
// that normalise MAIL FROM commands containing display names.
type lenientListener struct {
	net.Listener
}

// NewLenientListener wraps inner so that accepted connections automatically
// normalise MAIL FROM display-name formats before go-smtp parses them.
func NewLenientListener(inner net.Listener) net.Listener {
	return &lenientListener{Listener: inner}
}

func (l *lenientListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &lenientConn{Conn: c, rd: bufio.NewReader(c)}, nil
}

// lenientConn normalises MAIL FROM lines before they reach go-smtp's parser.
type lenientConn struct {
	net.Conn
	rd      *bufio.Reader
	pending []byte
}

func (c *lenientConn) Read(b []byte) (int, error) {
	if len(c.pending) > 0 {
		n := copy(b, c.pending)
		c.pending = c.pending[n:]
		return n, nil
	}

	line, err := c.rd.ReadBytes('\n')
	if len(line) > 0 {
		line = normalizeMailFrom(line)
		n := copy(b, line)
		if n < len(line) {
			c.pending = append(c.pending, line[n:]...)
		}
		// Return data now; err (if any) is surfaced on the next call.
		return n, nil
	}
	return 0, err
}

// emailRe matches the innermost <local@domain> in a string.
var emailRe = regexp.MustCompile(`<([a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+(?:\.[a-zA-Z0-9\-]+)*)>`)

// normalizeMailFrom rewrites a MAIL FROM line that contains a display-name
// (nested angle brackets or bare display name before the address) into the
// RFC 5321 compliant form "MAIL FROM:<addr@domain>".
// Lines that are not MAIL FROM commands, or that are already well-formed,
// are returned unchanged.
func normalizeMailFrom(line []byte) []byte {
	const prefix = "MAIL FROM:"
	if len(line) < len(prefix) || !strings.EqualFold(string(line[:len(prefix)]), prefix) {
		return line
	}

	// Strip CRLF for processing; we will add it back.
	stripped := bytes.TrimRight(line, "\r\n")
	rest := strings.TrimSpace(string(stripped[len(prefix):]))

	// Bounce address — leave alone.
	if rest == "<>" || strings.HasPrefix(rest, "<> ") {
		return line
	}

	// Find all <addr@domain> substrings. If there is exactly one, the
	// address is already well-formed; leave it alone.
	matches := emailRe.FindAllStringSubmatch(rest, -1)
	if len(matches) == 1 {
		return line
	}

	// More than one match means nested brackets, e.g. <Name <addr@domain>>.
	// Zero matches means bare "Name <addr@domain>" without outer bracket.
	var email string
	if len(matches) > 1 {
		// Take the last (innermost) match.
		email = matches[len(matches)-1][1]
	} else {
		// No angle brackets at all; find a plain addr@domain.
		plainRe := regexp.MustCompile(`([a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+(?:\.[a-zA-Z0-9\-]+)+)`)
		m := plainRe.FindString(rest)
		if m == "" {
			return line // unparseable; let go-smtp produce the error
		}
		email = m
	}

	// Re-attach any ESMTP parameters that follow the address section.
	// They start after the last '>' in the original rest string.
	params := ""
	if idx := strings.LastIndex(rest, ">"); idx >= 0 {
		params = strings.TrimSpace(rest[idx+1:])
	}

	result := "MAIL FROM:<" + email + ">"
	if params != "" {
		result += " " + params
	}
	result += "\r\n"
	return []byte(result)
}
