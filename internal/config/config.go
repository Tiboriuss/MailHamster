package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// User is a local SMTP credential pair.
type User struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// Auth holds the list of local users allowed to relay mail.
type Auth struct {
	Users []User `yaml:"users"`
}

// Upstream describes the remote SMTP server to relay messages to.
type Upstream struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	// TLS controls the connection security: none | starttls | tls
	TLS string `yaml:"tls"`
}

// Rewrite controls optional From-header rewriting.
type Rewrite struct {
	Enabled  bool   `yaml:"enabled"`
	From     string `yaml:"from"`
	FromName string `yaml:"from_name"`
}

// Logging controls log verbosity and output format.
type Logging struct {
	// Level: debug | info | warn | error (default: info)
	Level string `yaml:"level"`
	// Format: text | json (default: text)
	Format string `yaml:"format"`
}

// Listen describes the local listener address.
type Listen struct {
	// Addr is the TCP address to bind (default: 127.0.0.1:25).
	Addr string `yaml:"addr"`
	// LenientMailFrom strips display names from MAIL FROM commands before
	// parsing. Enable this when clients (e.g. Ruby on Rails / ActionMailer)
	// send "MAIL FROM:<Display Name <addr@domain>>" instead of the RFC 5321
	// compliant "MAIL FROM:<addr@domain>".
	LenientMailFrom bool `yaml:"lenient_mail_from"`
}

// Config is the top-level configuration structure.
type Config struct {
	Listen   Listen   `yaml:"listen"`
	Auth     Auth     `yaml:"auth"`
	Upstream Upstream `yaml:"upstream"`
	Rewrite  Rewrite  `yaml:"rewrite"`
	Logging  Logging  `yaml:"logging"`
}

// Load reads and parses the YAML config at path, applies defaults, and validates.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	cfg := &Config{}
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.applyDefaults(); err != nil {
		return nil, err
	}
	return cfg, cfg.validate()
}

func (c *Config) applyDefaults() error {
	if c.Listen.Addr == "" {
		c.Listen.Addr = "127.0.0.1:25"
	}
	if c.Upstream.Port == 0 {
		c.Upstream.Port = 587
	}
	if c.Upstream.TLS == "" {
		c.Upstream.TLS = "starttls"
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "text"
	}
	return nil
}

func (c *Config) validate() error {
	if len(c.Auth.Users) == 0 {
		return errors.New("auth.users must not be empty")
	}
	for i, u := range c.Auth.Users {
		if u.Username == "" {
			return fmt.Errorf("auth.users[%d].username must not be empty", i)
		}
		if u.Password == "" {
			return fmt.Errorf("auth.users[%d].password must not be empty", i)
		}
	}
	if c.Upstream.Host == "" {
		return errors.New("upstream.host is required")
	}
	switch c.Upstream.TLS {
	case "none", "starttls", "tls":
	default:
		return fmt.Errorf("upstream.tls must be none, starttls, or tls (got %q)", c.Upstream.TLS)
	}
	if c.Rewrite.Enabled && c.Rewrite.From == "" {
		return errors.New("rewrite.from must be set when rewrite.enabled is true")
	}
	switch c.Logging.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("logging.level must be debug, info, warn, or error (got %q)", c.Logging.Level)
	}
	switch c.Logging.Format {
	case "text", "json":
	default:
		return fmt.Errorf("logging.format must be text or json (got %q)", c.Logging.Format)
	}
	return nil
}
