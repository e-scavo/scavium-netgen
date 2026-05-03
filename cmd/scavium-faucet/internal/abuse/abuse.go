package abuse

import (
	"strings"
	"sync"
	"time"
)

// KeyType classifies the type of a blocklist entry key.
type KeyType string

const (
	KeyTypeIP          KeyType = "ip"
	KeyTypeAddress     KeyType = "address"
	KeyTypeFingerprint KeyType = "fingerprint"
)

// BlocklistEntry is a single blocked key with metadata.
type BlocklistEntry struct {
	Key       string
	KeyType   KeyType
	Reason    string
	BlockedAt time.Time
}

// blocklistKey builds the canonical storage key for a (type, value) pair.
func blocklistKey(kt KeyType, value string) string {
	return string(kt) + ":" + strings.ToLower(strings.TrimSpace(value))
}

// Blocklist is a concurrent in-memory blocklist keyed by IP address,
// Ethereum address, or browser fingerprint.
type Blocklist struct {
	mu      sync.RWMutex
	entries map[string]BlocklistEntry // key → entry
}

// NewBlocklist creates an empty Blocklist.
func NewBlocklist() *Blocklist {
	return &Blocklist{
		entries: make(map[string]BlocklistEntry),
	}
}

// Block adds or replaces the entry for the given key.
func (bl *Blocklist) Block(kt KeyType, value, reason string) {
	k := blocklistKey(kt, value)
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.entries[k] = BlocklistEntry{
		Key:       value,
		KeyType:   kt,
		Reason:    reason,
		BlockedAt: time.Now().UTC(),
	}
}

// Unblock removes the entry for the given key.  It is a no-op if the key is
// not present.
func (bl *Blocklist) Unblock(kt KeyType, value string) {
	k := blocklistKey(kt, value)
	bl.mu.Lock()
	defer bl.mu.Unlock()
	delete(bl.entries, k)
}

// IsBlocked reports whether the given key is currently blocked and returns
// the associated reason.
func (bl *Blocklist) IsBlocked(kt KeyType, value string) (blocked bool, reason string) {
	k := blocklistKey(kt, value)
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	e, ok := bl.entries[k]
	if !ok {
		return false, ""
	}
	return true, e.Reason
}

// Entries returns a snapshot of all current entries (for diagnostics/admin).
func (bl *Blocklist) Entries() []BlocklistEntry {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	out := make([]BlocklistEntry, 0, len(bl.entries))
	for _, e := range bl.entries {
		out = append(out, e)
	}
	return out
}

// CircuitBreaker trips after Count errors within Window and stays open until
// Reset is called or the window rolls over.  Safe for concurrent use.
type CircuitBreaker struct {
	mu        sync.Mutex
	threshold int
	window    time.Duration
	now       func() time.Time
	events    []time.Time // timestamps of recent error events
	open      bool
	openedAt  time.Time
}

// NewCircuitBreaker creates a CircuitBreaker that opens after threshold events
// within window.
func NewCircuitBreaker(threshold int, window time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		window:    window,
		now:       time.Now,
	}
}

// withClock replaces the time source.  Useful for testing.
func (cb *CircuitBreaker) withClock(now func() time.Time) *CircuitBreaker {
	cb.now = now
	return cb
}

// Record adds a new error event.  Returns true if the circuit just tripped or
// was already open.
func (cb *CircuitBreaker) Record() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := cb.now()
	cutoff := now.Add(-cb.window)

	// Evict events outside the window.
	fresh := cb.events[:0]
	for _, t := range cb.events {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	cb.events = append(fresh, now)

	if len(cb.events) >= cb.threshold {
		cb.open = true
		cb.openedAt = now
	}
	return cb.open
}

// IsOpen reports whether the circuit is currently open (tripped).
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if !cb.open {
		return false
	}
	// Auto-reset when the window has fully elapsed since the circuit opened.
	if cb.now().After(cb.openedAt.Add(cb.window)) {
		cb.open = false
		cb.events = cb.events[:0]
		return false
	}
	return true
}

// Reset manually resets the circuit to closed and clears all recorded events.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.open = false
	cb.events = cb.events[:0]
}

// FaucetMode represents the operational mode of the faucet.
type FaucetMode string

const (
	FaucetModeActive      FaucetMode = "active"
	FaucetModePaused      FaucetMode = "paused"
	FaucetModeMaintenance FaucetMode = "maintenance"
)

// ModeGetter is a lightweight interface for reading the current faucet mode.
// Implementations may read from memory, config, or a database.
type ModeGetter interface {
	GetMode() FaucetMode
}

// StaticMode always returns the same mode.  Useful for configuration-driven
// deployments where the mode is set at startup and never changes.
type StaticMode struct {
	mode FaucetMode
}

// NewStaticMode creates a StaticMode with the given mode.  An empty or
// unrecognised value defaults to FaucetModeActive.
func NewStaticMode(mode FaucetMode) *StaticMode {
	switch mode {
	case FaucetModePaused, FaucetModeMaintenance:
		return &StaticMode{mode: mode}
	default:
		return &StaticMode{mode: FaucetModeActive}
	}
}

func (s *StaticMode) GetMode() FaucetMode { return s.mode }

// AtomicMode is a mutable mode that can be updated at runtime without
// restarting the service.  Safe for concurrent use.
type AtomicMode struct {
	mu   sync.RWMutex
	mode FaucetMode
}

// NewAtomicMode creates an AtomicMode starting in the given mode.
func NewAtomicMode(initial FaucetMode) *AtomicMode {
	m := &AtomicMode{}
	m.Set(initial)
	return m
}

func (a *AtomicMode) GetMode() FaucetMode {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.mode
}

// Set updates the mode.  An empty or unrecognised value sets FaucetModeActive.
func (a *AtomicMode) Set(mode FaucetMode) {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch mode {
	case FaucetModePaused, FaucetModeMaintenance:
		a.mode = mode
	default:
		a.mode = FaucetModeActive
	}
}
