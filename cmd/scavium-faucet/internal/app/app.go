// Package app wires faucet configuration into the top-level HTTP application.
package app

import (
	"context"
	"errors"
	"net/http"

	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/faucet"
	"scavium-netgen/cmd/scavium-faucet/internal/httpapi"
	"scavium-netgen/cmd/scavium-faucet/internal/ready"
)

// App holds the runtime configuration and root HTTP handler for the faucet.
type App struct {
	Config  config.Config
	Handler http.Handler

	ctx        context.Context
	cancel     context.CancelFunc
	closeFuncs []func(context.Context) error
}

// New constructs the faucet application with its configured HTTP handler tree.
func New(cfg config.Config) *App {
	ctx, cancel := context.WithCancel(context.Background())

	return &App{
		Config: cfg,
		Handler: httpapi.NewHandler(httpapi.Dependencies{
			ReadinessChecks: ready.DefaultChecks(),
			ReadService:     faucet.NewInMemoryReadService(cfg),
		}),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Close stops background work and releases resources owned by the app.
func (a *App) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if a.cancel != nil {
		a.cancel()
	}

	var errs []error
	for _, closeFn := range a.closeFuncs {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		if closeFn == nil {
			continue
		}
		if err := closeFn(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
