package app

import (
	"net/http"
	"strings"

	"scavium-netgen/cmd/scavium-faucet/internal/httpapi"
)

const defaultAddress = "127.0.0.1:18080"

type Config struct {
	Address string
}

func DefaultConfig() Config {
	return Config{
		Address: defaultAddress,
	}
}

func (c Config) Normalized() Config {
	if strings.TrimSpace(c.Address) == "" {
		c.Address = defaultAddress
	}
	return c
}

type App struct {
	Config  Config
	Handler http.Handler
}

func New(config Config) *App {
	config = config.Normalized()

	return &App{
		Config:  config,
		Handler: httpapi.NewHandler(),
	}
}
