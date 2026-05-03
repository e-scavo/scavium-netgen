// Package app wires faucet configuration into the top-level HTTP application.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/chain"
	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"
	"scavium-netgen/cmd/scavium-faucet/internal/faucet"
	"scavium-netgen/cmd/scavium-faucet/internal/httpapi"
	"scavium-netgen/cmd/scavium-faucet/internal/ready"
	"scavium-netgen/cmd/scavium-faucet/internal/store/sqlite"
	"scavium-netgen/cmd/scavium-faucet/internal/worker"

	"github.com/ethereum/go-ethereum/common"
)

// App holds the runtime configuration and root HTTP handler for the faucet.
type App struct {
	Config  config.Config
	Handler http.Handler

	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	closeFuncs []func(context.Context) error
}

// New constructs the faucet application with its configured HTTP handler tree.
func New(cfg config.Config) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())

	store, err := openStore(cfg.DatabasePath)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open sqlite store: %w", err)
	}

	sender, chainClient, err := newSender(ctx, cfg)
	if err != nil {
		_ = store.Close()
		cancel()
		return nil, err
	}

	readService := faucet.NewPersistentReadService(cfg, store, store, store)
	app := &App{
		Config: cfg,
		Handler: httpapi.NewHandler(httpapi.Dependencies{
			ReadinessChecks: ready.DefaultChecks(),
			ReadService:     readService,
			AdminToken:      cfg.AdminToken,
		}),
		ctx:    ctx,
		cancel: cancel,
	}

	app.closeFuncs = append(app.closeFuncs, func(context.Context) error {
		return store.Close()
	})
	if chainClient != nil {
		app.closeFuncs = append(app.closeFuncs, func(context.Context) error {
			chainClient.Close()
			return nil
		})
	}

	if cfg.WorkerEnabled {
		workerCfg := worker.DefaultConfig()
		workerCfg.PollInterval = time.Duration(cfg.WorkerPollSeconds) * time.Second
		w := worker.New(store, sender, workerCfg, nil)
		app.start("worker", w.Run)
	}

	if !cfg.DryRun && cfg.WatcherEnabled && chainClient != nil {
		watcherCfg := chain.DefaultWatcherConfig()
		watcherCfg.PollInterval = time.Duration(cfg.WatcherPollSeconds) * time.Second
		watcherCfg.MinConfirmations = cfg.MinConfirmations
		w := chain.NewWatcher(store, chainClient, watcherCfg, nil)
		app.start("watcher", w.Run)
	}

	return app, nil
}

// Close stops background work and releases resources owned by the app.
func (a *App) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if a.cancel != nil {
		a.cancel()
	}

	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	var errs []error
	select {
	case <-done:
	case <-ctx.Done():
		errs = append(errs, ctx.Err())
	}

	for i := len(a.closeFuncs) - 1; i >= 0; i-- {
		closeFn := a.closeFuncs[i]
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

func (a *App) start(name string, run func(context.Context) error) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := run(a.ctx); err != nil && !errors.Is(err, context.Canceled) {
			// Step 11.1.3 keeps background supervision minimal. Worker/watcher internals
			// log per-cycle failures; unexpected terminal errors are intentionally ignored
			// here to preserve existing HTTP server behavior.
			_ = name
		}
	}()
}

func openStore(path string) (*sqlite.Store, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	return sqlite.Open(path)
}

func newSender(ctx context.Context, cfg config.Config) (domain.Sender, *chain.Client, error) {
	if cfg.DryRun {
		return chain.NewDryRunSender(common.Address{}), nil, nil
	}

	startupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := chain.NewClient(startupCtx, cfg.RPCURL)
	if err != nil {
		return nil, nil, fmt.Errorf("create chain client: %w", err)
	}
	if err := chain.ValidateChainID(startupCtx, client, cfg.ChainID); err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("validate chain id: %w", err)
	}

	signer, err := chain.NewPrivateKeySigner(cfg.PrivateKeyHex)
	if err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("create signer: %w", err)
	}

	return chain.NewEthSender(client, signer, cfg.ChainID, chain.GasPolicy{}), client, nil
}
