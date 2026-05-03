package abuse

import (
	"testing"
	"time"
)

// ---- Blocklist tests --------------------------------------------------------

func TestBlocklistBlockAndIsBlocked(t *testing.T) {
	bl := NewBlocklist()
	bl.Block(KeyTypeIP, "1.2.3.4", "spam")

	blocked, reason := bl.IsBlocked(KeyTypeIP, "1.2.3.4")
	if !blocked {
		t.Fatal("expected IP to be blocked")
	}
	if reason != "spam" {
		t.Fatalf("reason = %q, want spam", reason)
	}
}

func TestBlocklistUnblock(t *testing.T) {
	bl := NewBlocklist()
	bl.Block(KeyTypeIP, "1.2.3.4", "spam")
	bl.Unblock(KeyTypeIP, "1.2.3.4")

	blocked, _ := bl.IsBlocked(KeyTypeIP, "1.2.3.4")
	if blocked {
		t.Fatal("expected IP to be unblocked after Unblock")
	}
}

func TestBlocklistNotBlocked(t *testing.T) {
	bl := NewBlocklist()
	blocked, reason := bl.IsBlocked(KeyTypeIP, "9.9.9.9")
	if blocked {
		t.Fatal("expected IP to not be blocked")
	}
	if reason != "" {
		t.Fatalf("reason = %q, want empty", reason)
	}
}

func TestBlocklistDifferentKeyTypes(t *testing.T) {
	bl := NewBlocklist()
	bl.Block(KeyTypeAddress, "0xDEAD", "abuse")

	if b, _ := bl.IsBlocked(KeyTypeIP, "0xDEAD"); b {
		t.Fatal("address blocked under IP key type; types must be independent")
	}
	if b, _ := bl.IsBlocked(KeyTypeAddress, "0xDEAD"); !b {
		t.Fatal("address should be blocked under address key type")
	}
}

func TestBlocklistCaseInsensitive(t *testing.T) {
	bl := NewBlocklist()
	bl.Block(KeyTypeFingerprint, "AbCdEf", "fp-abuse")

	if b, _ := bl.IsBlocked(KeyTypeFingerprint, "abcdef"); !b {
		t.Fatal("fingerprint lookup should be case-insensitive")
	}
}

func TestBlocklistEntries(t *testing.T) {
	bl := NewBlocklist()
	bl.Block(KeyTypeIP, "10.0.0.1", "r1")
	bl.Block(KeyTypeIP, "10.0.0.2", "r2")

	entries := bl.Entries()
	if len(entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(entries))
	}
}

// ---- CircuitBreaker tests ---------------------------------------------------

func TestCircuitBreakerOpenAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Minute)

	cb.Record()
	cb.Record()
	if cb.IsOpen() {
		t.Fatal("circuit should not be open after 2 events (threshold 3)")
	}
	cb.Record()
	if !cb.IsOpen() {
		t.Fatal("circuit should be open after 3 events")
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cb := NewCircuitBreaker(2, 5*time.Minute)
	cb.Record()
	cb.Record()
	if !cb.IsOpen() {
		t.Fatal("expected circuit to be open")
	}
	cb.Reset()
	if cb.IsOpen() {
		t.Fatal("expected circuit to be closed after Reset")
	}
}

func TestCircuitBreakerAutoResetAfterWindow(t *testing.T) {
	now := time.Now()
	cb := NewCircuitBreaker(2, 100*time.Millisecond).withClock(func() time.Time { return now })
	cb.Record()
	cb.Record() // trips the circuit at `now`

	if !cb.IsOpen() {
		t.Fatal("expected circuit open just after threshold")
	}

	// Advance clock past the window.
	now = now.Add(200 * time.Millisecond)
	if cb.IsOpen() {
		t.Fatal("expected circuit to auto-reset after window elapsed")
	}
}

func TestCircuitBreakerEventsOutsideWindowNotCounted(t *testing.T) {
	now := time.Now()
	cb := NewCircuitBreaker(3, 1*time.Minute).withClock(func() time.Time { return now })
	cb.Record() // at T=0

	// Advance 2 minutes — first event falls outside window.
	now = now.Add(2 * time.Minute)
	cb.Record() // at T=2m
	cb.Record() // at T=2m

	// Only 2 events inside window, threshold is 3 → should NOT be open.
	if cb.IsOpen() {
		t.Fatal("circuit should not be open; old event is outside window")
	}
}

// ---- StaticMode tests -------------------------------------------------------

func TestStaticModeActive(t *testing.T) {
	m := NewStaticMode(FaucetModeActive)
	if m.GetMode() != FaucetModeActive {
		t.Fatalf("mode = %q, want %q", m.GetMode(), FaucetModeActive)
	}
}

func TestStaticModePaused(t *testing.T) {
	m := NewStaticMode(FaucetModePaused)
	if m.GetMode() != FaucetModePaused {
		t.Fatalf("mode = %q, want %q", m.GetMode(), FaucetModePaused)
	}
}

func TestStaticModeMaintenance(t *testing.T) {
	m := NewStaticMode(FaucetModeMaintenance)
	if m.GetMode() != FaucetModeMaintenance {
		t.Fatalf("mode = %q, want %q", m.GetMode(), FaucetModeMaintenance)
	}
}

func TestStaticModeUnknownDefaultsToActive(t *testing.T) {
	m := NewStaticMode("unknown-mode")
	if m.GetMode() != FaucetModeActive {
		t.Fatalf("mode = %q, want %q", m.GetMode(), FaucetModeActive)
	}
}

// ---- AtomicMode tests -------------------------------------------------------

func TestAtomicModeSetAndGet(t *testing.T) {
	m := NewAtomicMode(FaucetModeActive)
	if m.GetMode() != FaucetModeActive {
		t.Fatalf("initial mode = %q", m.GetMode())
	}
	m.Set(FaucetModePaused)
	if m.GetMode() != FaucetModePaused {
		t.Fatalf("after Set(paused): mode = %q", m.GetMode())
	}
}

func TestAtomicModeUnknownDefaultsToActive(t *testing.T) {
	m := NewAtomicMode("garbage")
	if m.GetMode() != FaucetModeActive {
		t.Fatalf("mode = %q, want active", m.GetMode())
	}
}
