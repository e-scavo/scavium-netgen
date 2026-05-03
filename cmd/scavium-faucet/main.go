// Package main starts the SCAVIUM faucet HTTP server.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/app"
	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/observability"
)

func main() {
	logger := observability.DefaultLogger()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		logger.Error("load config failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}

	application, err := app.New(cfg)
	if err != nil {
		logger.Error("create application failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
	server := &http.Server{
		Addr:              application.Config.BindAddr,
		Handler:           application.Handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errs := make(chan error, 1)
	go func() {
		logger.Info("scavium faucet listening", map[string]any{"addr": server.Addr})
		errs <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errs:
		if err != nil && err != http.ErrServerClosed {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if closeErr := application.Close(ctx); closeErr != nil {
				logger.Error("application close failed", map[string]any{"error": closeErr.Error()})
			}
			logger.Error("server failed", map[string]any{"error": err.Error()})
			os.Exit(1)
		}
	case <-stop:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("server shutdown failed", map[string]any{"error": err.Error()})
			os.Exit(1)
		}
		if err := application.Close(ctx); err != nil {
			logger.Error("application close failed", map[string]any{"error": err.Error()})
			os.Exit(1)
		}
		logger.Info("server stopped", nil)
	}
}
