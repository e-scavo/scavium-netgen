// Package app wires faucet configuration into the top-level HTTP application.
package app

import (
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
}

// New constructs the faucet application with its configured HTTP handler tree.
func New(cfg config.Config) *App {
	return &App{
		Config: cfg,
		Handler: httpapi.NewHandler(httpapi.Dependencies{
			ReadinessChecks: ready.DefaultChecks(),
			ReadService:     faucet.NewInMemoryReadService(cfg),
		}),
	}
}
