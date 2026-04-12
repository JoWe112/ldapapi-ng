// Command ldapapi-ng is the entrypoint for the LDAPS REST API service.
//
//	@title			ldapapi-ng
//	@version		0.1.0
//	@description	REST API that authenticates users and looks up attributes over LDAPS.
//	@BasePath		/
//
//	@securityDefinitions.basic	BasicAuth
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JoWe112/ldapapi-ng/internal/config"
	"github.com/JoWe112/ldapapi-ng/internal/handler"
	ldapclient "github.com/JoWe112/ldapapi-ng/internal/ldap"
	"github.com/JoWe112/ldapapi-ng/internal/version"
)

func main() {
	// Create a preliminary logger so config-load errors are visible.
	// Once config is loaded, the logger is replaced with the configured level.
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	if err := run(log); err != nil {
		log.Error("fatal", "error", err.Error())
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Replace the logger with the configured level.
	log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	log.Info("starting ldapapi-ng",
		"version", version.Version,
		"commit", version.Commit,
		"date", version.Date,
		"log_level", cfg.LogLevel.String(),
	)

	ldap, err := ldapclient.New(ldapclient.Options{
		Host:       cfg.LDAPHost,
		Port:       cfg.LDAPPort,
		BaseDN:     cfg.LDAPBaseDN,
		BindDN:     cfg.LDAPBindDN,
		BindPass:   cfg.LDAPBindPass,
		UserFilter: cfg.LDAPUserFilter,
		CACertPath: cfg.LDAPCACertPath,
		Timeout:    cfg.LDAPTimeout,
		Log:        log,
	})
	if err != nil {
		return err
	}

	router := &handler.Router{
		Config: cfg,
		LDAP:   ldap,
		Log:    log,
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router.Build(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Run the HTTP server in a goroutine so the main goroutine can wait
	// for shutdown signals.
	serverErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", cfg.ListenAddr, "auth_mode", string(cfg.AuthMode))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	// Wait for an interrupt or a server error.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case sig := <-stop:
		log.Info("shutdown signal received", "signal", sig.String())
	}

	// Graceful shutdown.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Info("http server stopped cleanly")
	return nil
}
