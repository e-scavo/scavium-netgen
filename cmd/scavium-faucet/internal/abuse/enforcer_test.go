package abuse

import (
	"context"
	"testing"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"

	"github.com/ethereum/go-ethereum/common"
)

type fakeSignalCounter struct {
	count int
	last  domain.AbuseSignalFilter
}

func (f *fakeSignalCounter) CountRecentAbuseSignals(_ context.Context, filter domain.AbuseSignalFilter) (int, error) {
	f.last = filter
	return f.count, nil
}

func TestProgressiveEnforcerAllowsBelowThreshold(t *testing.T) {
	cfg := config.Defaults()
	cfg.AbuseEnforcementEnabled = true
	cfg.AbuseEnforcementIPThreshold = 3
	cfg.AbuseEnforcementAddressThreshold = 0
	cfg.AbuseEnforcementFingerprintThreshold = 0
	counter := &fakeSignalCounter{count: 2}
	enforcer := NewProgressiveEnforcer(cfg, counter).WithClock(func() time.Time {
		return time.Date(2026, 5, 4, 15, 0, 0, 0, time.UTC)
	})

	decision, err := enforcer.Evaluate(context.Background(), domain.RiskInput{
		Address:  common.HexToAddress("0x52908400098527886E0F7030069857D2E4169EE7"),
		RemoteIP: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("allowed = false, reason=%q", decision.Reason)
	}
	if counter.last.RemoteIP != "203.0.113.10" {
		t.Fatalf("remote ip filter = %q", counter.last.RemoteIP)
	}
}

func TestProgressiveEnforcerRejectsAtThreshold(t *testing.T) {
	cfg := config.Defaults()
	cfg.AbuseEnforcementEnabled = true
	cfg.AbuseEnforcementIPThreshold = 3
	cfg.AbuseEnforcementAddressThreshold = 0
	cfg.AbuseEnforcementFingerprintThreshold = 0
	counter := &fakeSignalCounter{count: 3}
	enforcer := NewProgressiveEnforcer(cfg, counter)

	decision, err := enforcer.Evaluate(context.Background(), domain.RiskInput{
		Address:  common.HexToAddress("0x52908400098527886E0F7030069857D2E4169EE7"),
		RemoteIP: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if decision.Allowed {
		t.Fatal("allowed = true, want false")
	}
	if decision.Score != 3 {
		t.Fatalf("score = %d, want 3", decision.Score)
	}
	if decision.Reason == "" {
		t.Fatal("reason is empty")
	}
}

func TestProgressiveEnforcerDisabledAllows(t *testing.T) {
	cfg := config.Defaults()
	cfg.AbuseEnforcementEnabled = false
	counter := &fakeSignalCounter{count: 999}
	enforcer := NewProgressiveEnforcer(cfg, counter)

	decision, err := enforcer.Evaluate(context.Background(), domain.RiskInput{RemoteIP: "203.0.113.10"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("allowed = false, reason=%q", decision.Reason)
	}
}
