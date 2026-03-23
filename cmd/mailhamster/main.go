package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	gosmtp "github.com/emersion/go-smtp"

	"github.com/Tiboriuss/MailHamster/internal/config"
	"github.com/Tiboriuss/MailHamster/internal/server"
)

// version is set at build time via -ldflags "-X main.version=x.y.z".
var version = "dev"

func main() {
	configPath := flag.String("config", "/etc/mailhamster/mailhamster.yaml", "path to config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("mailhamster", version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mailhamster: %v\n", err)
		os.Exit(1)
	}

	logger := buildLogger(cfg)

	be := server.New(cfg, logger)

	s := gosmtp.NewServer(be)
	s.Addr = cfg.Listen.Addr
	s.Domain = "localhost"
	s.AllowInsecureAuth = true

	logger.Info("mailhamster starting",
		"version", version,
		"listen", cfg.Listen.Addr,
		"upstream", fmt.Sprintf("%s:%d", cfg.Upstream.Host, cfg.Upstream.Port),
		"upstream_tls", cfg.Upstream.TLS,
		"rewrite_enabled", cfg.Rewrite.Enabled,
		"lenient_mail_from", cfg.Listen.LenientMailFrom,
	)

	errCh := make(chan error, 1)
	go func() {
		if cfg.Listen.LenientMailFrom {
			ln, err := net.Listen("tcp", cfg.Listen.Addr)
			if err != nil {
				errCh <- err
				return
			}
			errCh <- s.Serve(server.NewLenientListener(ln))
		} else {
			errCh <- s.ListenAndServe()
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("shutting down", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.Shutdown(ctx); err != nil {
			logger.Error("shutdown error", "err", err)
		}
	case err := <-errCh:
		if err != nil {
			logger.Error("server stopped unexpectedly", "err", err)
			os.Exit(1)
		}
	}
}

func buildLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch cfg.Logging.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if cfg.Logging.Format == "json" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(h)
}
