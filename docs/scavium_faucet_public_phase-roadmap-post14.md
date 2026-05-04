# SCAVIUM Faucet — Post Phase 14 Roadmap

## Current Status

The SCAVIUM Faucet is fully deployed and production-ready on testnet:

- Backend (Go) operational
- nginx reverse proxy (HTTPS)
- TLS with certbot (auto-renew verified)
- systemd service active
- SQLite persistence
- RPC connected (txpool enabled)
- Claims working end-to-end
- Rate limiting active
- Frontend operational (CSP-safe)
- Firewall (UFW) active

---

## Phase 15 — Public Faucet Hardening & Abuse Protection

**Status:** CLOSED. Phase 15 is complete as the faucet abuse-protection baseline for the current public testnet deployment. The phase introduced human verification, durable abuse telemetry, conservative enforcement, and operational retention without changing the existing public API contract or exposing the backend directly.

### 15.1 — Captcha Integration
- hCaptcha or Cloudflare Turnstile
- Frontend widget integration
- Backend verification

### 15.2 — Claim Abuse Signals
- Durable SQLite-backed abuse signal capture
- IP + User-Agent + address + fingerprint correlation
- Captcha, risk, cooldown, rate-limit, budget, and accepted-claim observations
- Non-blocking telemetry layer for later adaptive policy and admin review

### 15.3 — Progressive Abuse Enforcement
- Conservative signal-based enforcement using the Phase 15.2 abuse signal ledger
- Temporary claim rejection when recent negative signals exceed configured IP, address, or fingerprint thresholds
- Runtime-configurable enforcement window and thresholds
- No public API contract changes; reuses existing `claim_rejected` error envelope

### 15.4 — Abuse Operations & Retention
- Configurable abuse signal retention window
- Startup pruning of expired abuse signals
- Internal aggregate summaries by signal kind for future operator surfaces
- Operational tuning documentation before Phase 16 metrics/admin exposure

### 15.close — Phase Closure
- Abuse Protection consolidated as a closed production-ready phase
- 15.1 through 15.4 recorded as cumulative, deploy-compatible increments
- No code, API, schema, nginx, systemd, or runtime configuration changes required for closure
- Phase 16 explicitly starts from the structured abuse dataset, request logs, and operational boundaries already established

---

## Phase 16 — Observability & Operations

### 16.1 — Metrics
- Prometheus integration
- Claims/sec
- Failures
- RPC latency

### 16.2 — Alerts
- Faucet empty
- RPC down
- High failure rate

### 16.3 — Logging Improvements
- Structured logs expansion
- Correlation IDs

---

## Phase 17 — Token Faucet Extension

### 17.1 — Multi-token Support
- Config-driven tokens
- ERC20 send support

### 17.2 — Token Registration (testnet)
- API endpoint
- Basic validation

### 17.3 — Frontend Token Selection
- Token selector UI
- Dynamic routing

---

## Phase 18 — Admin & Control Plane

### 18.1 — Admin Interface
- Pause/resume faucet
- Stats overview

### 18.2 — Dynamic Budget Control
- Runtime config changes

### 18.3 — Claim Audit
- History
- Export

---

## Phase 19 — Production Readiness (Mainnet)

### 19.1 — RPC Failover
- Multiple node support

### 19.2 — Wallet Hardening
- Hot/cold separation

### 19.3 — Funding Automation
- Auto refill logic

### 19.4 — Security Hardening
- Final production posture

---

## Conclusion

The faucet has reached a stable production baseline.

Remaining phases focus on:
- Observability and operator feedback loops
- Feature expansion
- Admin/control-plane maturation
- Mainnet readiness
