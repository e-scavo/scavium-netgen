package faucet

import (
	"context"
	"errors"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/abuse"
	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"
	"scavium-netgen/cmd/scavium-faucet/internal/store/sqlite"

	"github.com/ethereum/go-ethereum/common"
)

func TestPersistentReadServiceCreateClaimPersists(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	service := newPersistentTestService(t, store, persistentTestConfig(), persistentTestNow())

	created, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address: persistentTestAddress(),
	})
	if err != nil {
		t.Fatalf("create claim: %v", err)
	}
	if created.Status != domain.ClaimStatusQueued {
		t.Fatalf("status = %q, want queued", created.Status)
	}

	stored, err := store.GetClaim(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get stored claim: %v", err)
	}
	if stored.ID != created.ID {
		t.Fatalf("stored id = %q, want %q", stored.ID, created.ID)
	}
	if stored.AmountWei.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("stored amount = %s, want 42", stored.AmountWei)
	}
}

func TestPersistentReadServiceGetClaimAfterRecreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "faucet.db")
	cfg := persistentTestConfig()

	store := openPersistentTestStore(t, path)
	service := newPersistentTestService(t, store, cfg, persistentTestNow())
	created, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address: persistentTestAddress(),
	})
	if err != nil {
		t.Fatalf("create claim: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}

	reopened := openPersistentTestStore(t, path)
	defer reopened.Close()
	recreatedService := newPersistentTestService(t, reopened, cfg, persistentTestNow().Add(time.Minute))

	got, found, err := recreatedService.GetClaim(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	if !found {
		t.Fatal("claim not found after service recreation")
	}
	if got.ID != created.ID {
		t.Fatalf("got id = %q, want %q", got.ID, created.ID)
	}
}

func TestPersistentReadServiceIdempotencyAfterRecreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "faucet.db")
	cfg := persistentTestConfig()
	cfg.CooldownSeconds = 3600

	store := openPersistentTestStore(t, path)
	service := newPersistentTestService(t, store, cfg, persistentTestNow())
	first, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:        persistentTestAddress(),
		IdempotencyKey: "same-key",
	})
	if err != nil {
		t.Fatalf("create first claim: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}

	reopened := openPersistentTestStore(t, path)
	defer reopened.Close()
	recreatedService := newPersistentTestService(t, reopened, cfg, persistentTestNow().Add(5*time.Minute))
	second, err := recreatedService.CreateClaim(context.Background(), ClaimRequest{
		Address:        persistentTestAddress(),
		IdempotencyKey: "same-key",
	})
	if err != nil {
		t.Fatalf("create idempotent claim after recreation: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("ids differ: %q != %q", first.ID, second.ID)
	}
	if second.IdempotencyKey != "same-key" {
		t.Fatalf("idempotency key = %q", second.IdempotencyKey)
	}
}

func TestPersistentReadServiceCooldownBlocksRepeatedClaims(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	cfg := persistentTestConfig()
	cfg.CooldownSeconds = 600
	service := newPersistentTestService(t, store, cfg, persistentTestNow())

	if _, err := service.CreateClaim(context.Background(), ClaimRequest{Address: persistentTestAddress()}); err != nil {
		t.Fatalf("create first claim: %v", err)
	}

	_, err := service.CreateClaim(context.Background(), ClaimRequest{Address: persistentTestAddress()})
	if err == nil {
		t.Fatal("second claim returned nil error")
	}
	if !errors.Is(err, ErrCooldownActive) {
		t.Fatalf("error = %v, want ErrCooldownActive", err)
	}
	var claimErr *ClaimError
	if !errors.As(err, &claimErr) {
		t.Fatalf("error = %T, want ClaimError", err)
	}
	if claimErr.RetryAfterSeconds != 600 {
		t.Fatalf("retry after = %d, want 600", claimErr.RetryAfterSeconds)
	}
}

func TestPersistentReadServiceAddressStatusReflectsCooldown(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	cfg := persistentTestConfig()
	cfg.CooldownSeconds = 600
	now := persistentTestNow()
	service := newPersistentTestService(t, store, cfg, now)

	if _, err := service.CreateClaim(context.Background(), ClaimRequest{Address: persistentTestAddress()}); err != nil {
		t.Fatalf("create claim: %v", err)
	}

	laterService := newPersistentTestService(t, store, cfg, now.Add(2*time.Minute))
	status, err := laterService.AddressStatus(context.Background(), persistentTestAddress())
	if err != nil {
		t.Fatalf("address status: %v", err)
	}
	if status.Eligible {
		t.Fatal("eligible = true, want false")
	}
	if status.CooldownRemainingSeconds != 480 {
		t.Fatalf("cooldown remaining = %d, want 480", status.CooldownRemainingSeconds)
	}
	if status.NextEligibleTime != now.Add(10*time.Minute).Format(time.RFC3339) {
		t.Fatalf("next eligible = %q", status.NextEligibleTime)
	}
}

func TestPersistentReadServiceRejectsInactiveMode(t *testing.T) {
	cfg := persistentTestConfig()
	cfg.FaucetMode = "paused"
	service := NewPersistentReadService(cfg, nil, nil, nil)

	_, err := service.CreateClaim(context.Background(), ClaimRequest{Address: persistentTestAddress()})
	if err == nil {
		t.Fatal("create claim returned nil error")
	}
	if !errors.Is(err, ErrFaucetUnavailable) {
		t.Fatalf("error = %v, want ErrFaucetUnavailable", err)
	}
}

func TestPersistentReadServiceDisabledCaptchaAllowsClaim(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	service := newPersistentTestService(t, store, persistentTestConfig(), persistentTestNow())

	_, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:      persistentTestAddress(),
		RemoteIP:     "203.0.113.10",
		CaptchaToken: "",
	})
	if err != nil {
		t.Fatalf("create claim with disabled captcha: %v", err)
	}
}

func TestPersistentReadServiceFailedCaptchaRejectsClaim(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	service := newPersistentTestService(t, store, persistentTestConfig(), persistentTestNow())
	service.SetCaptchaVerifier(fakeCaptchaVerifier{passed: false, reason: "captcha failed"})

	_, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:      persistentTestAddress(),
		RemoteIP:     "203.0.113.10",
		CaptchaToken: "bad-token",
	})
	if err == nil {
		t.Fatal("create claim returned nil error")
	}
	if !errors.Is(err, ErrCaptchaFailed) {
		t.Fatalf("error = %v, want ErrCaptchaFailed", err)
	}
}

func TestPersistentReadServiceDailyBudgetBlocksExceededClaims(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	cfg := persistentTestConfig()
	cfg.DailyBudgetWei = big.NewInt(84)
	service := newPersistentTestService(t, store, cfg, persistentTestNow())

	if _, err := service.CreateClaim(context.Background(), ClaimRequest{Address: persistentTestAddress()}); err != nil {
		t.Fatalf("create first claim: %v", err)
	}
	if _, err := service.CreateClaim(context.Background(), ClaimRequest{Address: persistentSecondTestAddress()}); err != nil {
		t.Fatalf("create second claim: %v", err)
	}

	_, err := service.CreateClaim(context.Background(), ClaimRequest{Address: common.HexToAddress("0xde709f2102306220921060314715629080e2fb77")})
	if err == nil {
		t.Fatal("third claim returned nil error")
	}
	if !errors.Is(err, ErrDailyBudgetExceeded) {
		t.Fatalf("error = %v, want ErrDailyBudgetExceeded", err)
	}
}

func TestPersistentReadServiceDailyBudgetSurvivesRecreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "faucet.db")
	cfg := persistentTestConfig()
	cfg.DailyBudgetWei = big.NewInt(42)

	store := openPersistentTestStore(t, path)
	service := newPersistentTestService(t, store, cfg, persistentTestNow())
	if _, err := service.CreateClaim(context.Background(), ClaimRequest{Address: persistentTestAddress()}); err != nil {
		t.Fatalf("create first claim: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}

	reopened := openPersistentTestStore(t, path)
	defer reopened.Close()
	recreatedService := newPersistentTestService(t, reopened, cfg, persistentTestNow().Add(time.Hour))
	_, err := recreatedService.CreateClaim(context.Background(), ClaimRequest{Address: persistentSecondTestAddress()})
	if err == nil {
		t.Fatal("second claim returned nil error")
	}
	if !errors.Is(err, ErrDailyBudgetExceeded) {
		t.Fatalf("error = %v, want ErrDailyBudgetExceeded", err)
	}
}

func TestPersistentReadServiceDailyBudgetUsesCurrentUTCDay(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	cfg := persistentTestConfig()
	cfg.DailyBudgetWei = big.NewInt(42)

	yesterday := domain.Claim{
		ID:        "yesterday",
		Address:   persistentTestAddress(),
		AmountWei: big.NewInt(42),
		Status:    domain.ClaimStatusQueued,
		CreatedAt: persistentTestNow().Add(-24 * time.Hour),
		UpdatedAt: persistentTestNow().Add(-24 * time.Hour),
	}
	if _, err := store.CreateClaim(context.Background(), yesterday); err != nil {
		t.Fatalf("create yesterday claim: %v", err)
	}

	service := newPersistentTestService(t, store, cfg, persistentTestNow())
	if _, err := service.CreateClaim(context.Background(), ClaimRequest{Address: persistentSecondTestAddress()}); err != nil {
		t.Fatalf("create current day claim: %v", err)
	}
}

func TestPersistentReadServiceAddressRateLimit(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	cfg := persistentTestConfig()
	cfg.RateLimitAddrPerDay = 1
	cfg.RateLimitIPPerHour = 100
	service := newPersistentTestService(t, store, cfg, persistentTestNow())

	request := ClaimRequest{Address: persistentTestAddress(), RemoteIP: "203.0.113.10"}
	if _, err := service.CreateClaim(context.Background(), request); err != nil {
		t.Fatalf("create first claim: %v", err)
	}

	request.RemoteIP = "203.0.113.11"
	_, err := service.CreateClaim(context.Background(), request)
	if err == nil {
		t.Fatal("second claim returned nil error")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("error = %v, want ErrRateLimited", err)
	}
}

func TestPersistentReadServiceIPRateLimit(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	cfg := persistentTestConfig()
	cfg.RateLimitAddrPerDay = 100
	cfg.RateLimitIPPerHour = 1
	service := newPersistentTestService(t, store, cfg, persistentTestNow())

	if _, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:  persistentTestAddress(),
		RemoteIP: "203.0.113.10",
	}); err != nil {
		t.Fatalf("create first claim: %v", err)
	}

	_, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:  persistentSecondTestAddress(),
		RemoteIP: "203.0.113.10",
	})
	if err == nil {
		t.Fatal("second claim returned nil error")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("error = %v, want ErrRateLimited", err)
	}
}

func TestPersistentReadServiceFingerprintRateLimit(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	cfg := persistentTestConfig()
	cfg.RateLimitAddrPerDay = 100
	cfg.RateLimitIPPerHour = 1
	service := newPersistentTestService(t, store, cfg, persistentTestNow())

	if _, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:     persistentTestAddress(),
		Fingerprint: "browser-1",
	}); err != nil {
		t.Fatalf("create first claim: %v", err)
	}

	_, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:     persistentSecondTestAddress(),
		Fingerprint: "browser-1",
	})
	if err == nil {
		t.Fatal("second claim returned nil error")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("error = %v, want ErrRateLimited", err)
	}
}

func TestPersistentReadServiceRiskEngineRejectsClaim(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	service := newPersistentTestService(t, store, persistentTestConfig(), persistentTestNow())
	service.SetRiskEngine(fakeRiskEngine{allowed: false, reason: "risk rejected"})

	_, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:     persistentTestAddress(),
		RemoteIP:    "203.0.113.10",
		UserAgent:   "wallet-test/1.0",
		Fingerprint: "browser-1",
	})
	if err == nil {
		t.Fatal("create claim returned nil error")
	}
	if !errors.Is(err, ErrClaimRejected) {
		t.Fatalf("error = %v, want ErrClaimRejected", err)
	}
}

func TestPersistentReadServiceRecordsAbuseSignals(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	service := newPersistentTestService(t, store, persistentTestConfig(), persistentTestNow())
	service.SetAbuseSignalRecorder(store)
	service.SetCaptchaVerifier(fakeCaptchaVerifier{passed: true})

	_, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:      persistentTestAddress(),
		RemoteIP:     "203.0.113.10",
		UserAgent:    "wallet-test/1.0",
		CaptchaToken: "ok-token",
		Fingerprint:  "browser-1",
	})
	if err != nil {
		t.Fatalf("create claim: %v", err)
	}

	signals, err := store.ListAbuseSignals(context.Background(), 10)
	if err != nil {
		t.Fatalf("list abuse signals: %v", err)
	}
	if len(signals) != 2 {
		t.Fatalf("signals len = %d, want 2", len(signals))
	}
	if signals[0].Kind != domain.AbuseSignalCaptchaPassed {
		t.Fatalf("first signal kind = %q", signals[0].Kind)
	}
	if signals[1].Kind != domain.AbuseSignalClaimAccepted {
		t.Fatalf("second signal kind = %q", signals[1].Kind)
	}
	if signals[1].ClaimID == "" {
		t.Fatal("accepted signal claim id is empty")
	}
	if signals[1].RemoteIP != "203.0.113.10" || signals[1].Fingerprint != "browser-1" {
		t.Fatalf("unexpected signal metadata: %+v", signals[1])
	}
}

func TestPersistentReadServiceRecordsFailedCaptchaSignal(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	service := newPersistentTestService(t, store, persistentTestConfig(), persistentTestNow())
	service.SetAbuseSignalRecorder(store)
	service.SetCaptchaVerifier(fakeCaptchaVerifier{passed: false, reason: "captcha failed"})

	_, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:      persistentTestAddress(),
		RemoteIP:     "203.0.113.10",
		CaptchaToken: "bad-token",
	})
	if err == nil {
		t.Fatal("create claim returned nil error")
	}
	if !errors.Is(err, ErrCaptchaFailed) {
		t.Fatalf("error = %v, want ErrCaptchaFailed", err)
	}

	signals, err := store.ListAbuseSignals(context.Background(), 10)
	if err != nil {
		t.Fatalf("list abuse signals: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("signals len = %d, want 1", len(signals))
	}
	if signals[0].Kind != domain.AbuseSignalCaptchaFailed || signals[0].Reason != "captcha failed" {
		t.Fatalf("unexpected signal: %+v", signals[0])
	}
}

func TestPersistentReadServiceRecordsRateLimitSignal(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	cfg := persistentTestConfig()
	cfg.RateLimitAddrPerDay = 100
	cfg.RateLimitIPPerHour = 1
	service := newPersistentTestService(t, store, cfg, persistentTestNow())
	service.SetAbuseSignalRecorder(store)

	if _, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:  persistentTestAddress(),
		RemoteIP: "203.0.113.10",
	}); err != nil {
		t.Fatalf("create first claim: %v", err)
	}

	_, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:  persistentSecondTestAddress(),
		RemoteIP: "203.0.113.10",
	})
	if err == nil {
		t.Fatal("second claim returned nil error")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("error = %v, want ErrRateLimited", err)
	}

	signals, err := store.ListAbuseSignals(context.Background(), 10)
	if err != nil {
		t.Fatalf("list abuse signals: %v", err)
	}
	if len(signals) != 2 {
		t.Fatalf("signals len = %d, want 2", len(signals))
	}
	if signals[1].Kind != domain.AbuseSignalRateLimited {
		t.Fatalf("second signal kind = %q", signals[1].Kind)
	}
	if signals[1].Reason == "" {
		t.Fatal("rate limit signal reason is empty")
	}
}

func TestPersistentReadServiceProgressiveAbuseEnforcementRejects(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()

	now := persistentTestNow()
	cfg := persistentTestConfig()
	cfg.AbuseEnforcementEnabled = true
	cfg.AbuseEnforcementWindowSeconds = 3600
	cfg.AbuseEnforcementIPThreshold = 2
	cfg.AbuseEnforcementAddressThreshold = 0
	cfg.AbuseEnforcementFingerprintThreshold = 0

	for i := 0; i < 2; i++ {
		if err := store.RecordAbuseSignal(context.Background(), domain.AbuseSignal{
			Kind:      domain.AbuseSignalCaptchaFailed,
			RemoteIP:  "203.0.113.10",
			CreatedAt: now.Add(-time.Duration(i+1) * time.Minute),
		}); err != nil {
			t.Fatalf("record previous abuse signal: %v", err)
		}
	}

	service := newPersistentTestService(t, store, cfg, now)
	service.SetAbuseSignalRecorder(store)
	service.SetRiskEngine(abuse.NewProgressiveEnforcer(cfg, store).WithClock(func() time.Time { return now }))

	_, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:  persistentTestAddress(),
		RemoteIP: "203.0.113.10",
	})
	if err == nil {
		t.Fatal("create claim returned nil error")
	}
	if !errors.Is(err, ErrClaimRejected) {
		t.Fatalf("error = %v, want ErrClaimRejected", err)
	}

	signals, err := store.ListAbuseSignals(context.Background(), 10)
	if err != nil {
		t.Fatalf("list abuse signals: %v", err)
	}
	if got := signals[len(signals)-1]; got.Kind != domain.AbuseSignalRiskRejected || got.Score != 2 {
		t.Fatalf("last signal = %+v, want risk_rejected score 2", got)
	}
}

func openPersistentTestStore(t *testing.T, path string) *sqlite.Store {
	t.Helper()
	if path == "" {
		path = filepath.Join(t.TempDir(), "faucet.db")
	}
	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store
}

func newPersistentTestService(t *testing.T, store *sqlite.Store, cfg config.Config, now time.Time) *PersistentReadService {
	t.Helper()
	service := NewPersistentReadServiceWithClock(cfg, store, store, store, func() time.Time { return now })
	nextID := 0
	service.SetClaimIDGenerator(func() (string, error) {
		nextID++
		return "claim_test_" + string(rune('a'+nextID-1)), nil
	})
	return service
}

func persistentTestConfig() config.Config {
	cfg := testConfig()
	cfg.FaucetMode = "active"
	cfg.CooldownSeconds = 0
	cfg.RateLimitAddrPerDay = 100
	return cfg
}

func persistentTestNow() time.Time {
	return time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
}

func persistentTestAddress() common.Address {
	return domain.MustValidateAddress("0x52908400098527886E0F7030069857D2E4169EE7")
}

func persistentSecondTestAddress() common.Address {
	return domain.MustValidateAddress("0x8617E340B3D01FA5F11F306F4090FD50E238070D")
}

type fakeCaptchaVerifier struct {
	passed bool
	reason string
}

func (v fakeCaptchaVerifier) Verify(context.Context, string, string) (domain.CaptchaDecision, error) {
	return domain.CaptchaDecision{Passed: v.passed, Reason: v.reason}, nil
}

type fakeRiskEngine struct {
	allowed bool
	reason  string
}

func (e fakeRiskEngine) Evaluate(context.Context, domain.RiskInput) (domain.RiskDecision, error) {
	return domain.RiskDecision{Allowed: e.allowed, Reason: e.reason}, nil
}

func TestPersistentReadServiceRejectsInvalidTokenBeforeCaptcha(t *testing.T) {
	store := openPersistentTestStore(t, "")
	defer store.Close()
	service := newPersistentTestService(t, store, persistentTestConfig(), persistentTestNow())
	service.SetCaptchaVerifier(fakeCaptchaVerifier{passed: true, reason: "captcha should not be needed"})
	service.SetAbuseSignalRecorder(store)

	_, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:      persistentTestAddress(),
		TokenID:      "missing-token",
		RemoteIP:     "203.0.113.10",
		CaptchaToken: "ok-token",
	})
	if err == nil {
		t.Fatal("create claim returned nil error")
	}
	if !errors.Is(err, ErrClaimRejected) {
		t.Fatalf("error = %v, want ErrClaimRejected", err)
	}
	var claimErr *ClaimError
	if !errors.As(err, &claimErr) {
		t.Fatalf("error = %T, want ClaimError", err)
	}
	if claimErr.Reason != "invalid_token" {
		t.Fatalf("reason = %q, want invalid_token", claimErr.Reason)
	}

	signals, err := store.ListAbuseSignals(context.Background(), 10)
	if err != nil {
		t.Fatalf("list signals: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("signals = %d, want 1", len(signals))
	}
	if signals[0].Kind != domain.AbuseSignalInvalidToken {
		t.Fatalf("signal kind = %q, want %q", signals[0].Kind, domain.AbuseSignalInvalidToken)
	}
}
