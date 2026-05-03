package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/app"
)

func main() {
	log.SetFlags(0)

	cfg := app.DefaultConfig()
	if address := os.Getenv("SCAVIUM_FAUCET_ADDR"); address != "" {
		cfg.Address = address
	}

	application := app.New(cfg)
	server := &http.Server{
		Addr:              application.Config.Address,
		Handler:           application.Handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errs := make(chan error, 1)
	go func() {
		log.Printf("scavium faucet listening on %s", server.Addr)
		errs <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errs:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	case <-stop:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("server shutdown: %v", err)
		}
	}
}
