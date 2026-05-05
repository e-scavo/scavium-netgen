// Package app wires faucet configuration into the top-level HTTP application.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/abuse"
	"scavium-netgen/cmd/scavium-faucet/internal/admin"
	"scavium-netgen/cmd/scavium-faucet/internal/captcha"
	"scavium-netgen/cmd/scavium-faucet/internal/chain"
	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"
	"scavium-netgen/cmd/scavium-faucet/internal/faucet"
	"scavium-netgen/cmd/scavium-faucet/internal/httpapi"
	"scavium-netgen/cmd/scavium-faucet/internal/observability"
	"scavium-netgen/cmd/scavium-faucet/internal/ready"
	"scavium-netgen/cmd/scavium-faucet/internal/store/sqlite"
	"scavium-netgen/cmd/scavium-faucet/internal/version"
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
	closeOnce  sync.Once
	closeErr   error
	closeFuncs []func(context.Context) error
}

type senderBundle struct {
	sender      domain.Sender
	chainClient *chain.Client
	signer      chain.Signer
}

// New constructs the faucet application with its configured HTTP handler tree.
func New(cfg config.Config) (*App, error) {
	return NewWithLogger(cfg, nil)
}

// NewWithLogger constructs the faucet application and wires request logging when logger is provided.
func NewWithLogger(cfg config.Config, logger *observability.Logger) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())

	store, err := openStore(cfg.DatabasePath)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open sqlite store: %w", err)
	}
	if _, err := abuse.PruneSignalsByRetention(ctx, store, cfg.AbuseSignalRetentionDays, time.Now().UTC()); err != nil {
		_ = store.Close()
		cancel()
		return nil, fmt.Errorf("prune abuse signals: %w", err)
	}

	senderBundle, err := newSender(ctx, cfg)
	if err != nil {
		_ = store.Close()
		cancel()
		return nil, err
	}

	readService := faucet.NewPersistentReadService(cfg, store, store, store)
	readService.SetAbuseSignalRecorder(store)
	readService.SetRiskEngine(abuse.NewProgressiveEnforcer(cfg, store))
	captchaVerifier, err := newCaptchaVerifier(cfg)
	if err != nil {
		_ = store.Close()
		if senderBundle.chainClient != nil {
			senderBundle.chainClient.Close()
		}
		cancel()
		return nil, err
	}
	readService.SetCaptchaVerifier(captchaVerifier)
	adminService := admin.NewInMemoryAdminService()
	adminService.SetModeController(readService)
	readinessChecks := runtimeChecks(cfg, store, senderBundle.chainClient, senderBundle.signer)
	metrics := observability.NewRuntimeMetrics(version.Current())
	app := &App{
		Config: cfg,
		Handler: httpapi.NewHandler(httpapi.Dependencies{
			ReadinessChecks: readinessChecks,
			ReadService:     readService,
			AdminService:    adminService,
			AdminToken:      cfg.AdminToken,
			TrustedProxy:    cfg.TrustedProxy,
			CORSOrigins:     cfg.CORSAllowedOrigins,
			Logger:          logger,
			Metrics:         metrics,
		}),
		ctx:    ctx,
		cancel: cancel,
	}

	app.closeFuncs = append(app.closeFuncs, func(context.Context) error {
		return store.Close()
	})
	if senderBundle.chainClient != nil {
		app.closeFuncs = append(app.closeFuncs, func(context.Context) error {
			senderBundle.chainClient.Close()
			return nil
		})
	}

	if cfg.WorkerEnabled {
		workerCfg := worker.DefaultConfig()
		workerCfg.PollInterval = time.Duration(cfg.WorkerPollSeconds) * time.Second
		w := worker.New(store, senderBundle.sender, workerCfg, nil)
		app.start("worker", w.Run)
	}

	if !cfg.DryRun && cfg.WatcherEnabled && senderBundle.chainClient != nil {
		watcherCfg := chain.DefaultWatcherConfig()
		watcherCfg.PollInterval = time.Duration(cfg.WatcherPollSeconds) * time.Second
		watcherCfg.MinConfirmations = cfg.MinConfirmations
		w := chain.NewWatcher(store, senderBundle.chainClient, watcherCfg, nil)
		app.start("watcher", w.Run)
	}

	return app, nil
}

// Close stops background work and releases resources owned by the app.
func (a *App) Close(ctx context.Context) error {
	a.closeOnce.Do(func() {
		a.closeErr = a.close(ctx)
	})
	return a.closeErr
}

func (a *App) close(ctx context.Context) error {
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

func newCaptchaVerifier(cfg config.Config) (domain.CaptchaVerifier, error) {
	switch strings.TrimSpace(strings.ToLower(cfg.CaptchaProvider)) {
	case "", "disabled":
		return nil, nil
	case "dev":
		return captcha.DevAlwaysPass{}, nil
	case "hcaptcha", "recaptcha", "turnstile":
		verifyURL := strings.TrimSpace(cfg.CaptchaVerifyURL)
		if verifyURL == "" {
			verifyURL = defaultCaptchaVerifyURL(strings.TrimSpace(strings.ToLower(cfg.CaptchaProvider)))
		}
		if strings.TrimSpace(cfg.CaptchaSecret) == "" {
			return nil, fmt.Errorf("captcha provider %q requires secret", cfg.CaptchaProvider)
		}
		return captcha.NewHTTPVerifier(verifyURL, cfg.CaptchaSecret), nil
	default:
		return nil, fmt.Errorf("unsupported captcha provider %q", cfg.CaptchaProvider)
	}
}

func newSender(ctx context.Context, cfg config.Config) (senderBundle, error) {
	if cfg.DryRun {
		return senderBundle{sender: chain.NewDryRunSender(common.Address{})}, nil
	}

	startupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := chain.NewClient(startupCtx, cfg.RPCURL)
	if err != nil {
		return senderBundle{}, fmt.Errorf("create chain client: %w", err)
	}
	if err := chain.ValidateChainID(startupCtx, client, cfg.ChainID); err != nil {
		client.Close()
		return senderBundle{}, fmt.Errorf("validate chain id: %w", err)
	}

	signer, err := chain.NewPrivateKeySigner(cfg.PrivateKeyHex)
	if err != nil {
		client.Close()
		return senderBundle{}, fmt.Errorf("create signer: %w", err)
	}

	return senderBundle{
		sender:      chain.NewEthSender(client, signer, cfg.ChainID, chain.GasPolicy{}),
		chainClient: client,
		signer:      signer,
	}, nil
}

func runtimeChecks(cfg config.Config, store *sqlite.Store, chainClient *chain.Client, signer chain.Signer) []ready.Check {
	checks := []ready.Check{
		ready.DBCheck(store),
		ready.QueueCheck(store),
	}
	if !cfg.DryRun && chainClient != nil {
		checks = append(checks, ready.RPCCheck(chainClient))
		if signer != nil {
			checks = append(checks, ready.WalletCheck(chainClient, signer))
		}
	}
	return checks
}

func defaultCaptchaVerifyURL(provider string) string {
	switch provider {
	case "hcaptcha":
		return "https://hcaptcha.com/siteverify"
	case "recaptcha":
		return "https://www.google.com/recaptcha/api/siteverify"
	case "turnstile":
		return "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	default:
		return ""
	}
}
