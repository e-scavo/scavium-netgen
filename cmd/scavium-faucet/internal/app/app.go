package app

import (
	"net/http"

	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/faucet"
	"scavium-netgen/cmd/scavium-faucet/internal/httpapi"
	"scavium-netgen/cmd/scavium-faucet/internal/ready"
)

type App struct {
	Config  config.Config
	Handler http.Handler
}

func New(cfg config.Config) *App {
	return &App{
		Config: cfg,
		Handler: httpapi.NewHandler(httpapi.Dependencies{
			ReadinessChecks: ready.DefaultChecks(),
			ReadService:     faucet.NewInMemoryReadService(cfg),
		}),
	}
}
