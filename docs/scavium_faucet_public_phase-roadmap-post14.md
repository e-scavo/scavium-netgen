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

### 15.1 — Captcha Integration
- hCaptcha or Cloudflare Turnstile
- Frontend widget integration
- Backend verification

### 15.2 — Claim Abuse Signals
- Durable SQLite-backed abuse signal capture
- IP + User-Agent + address + fingerprint correlation
- Captcha, risk, cooldown, rate-limit, budget, and accepted-claim observations
- Non-blocking telemetry layer for later adaptive policy and admin review

### 15.3 — Blacklisting
- IP blocking
- Address blocking
- CIDR rules

### 15.4 — Adaptive Rate Limiting
- Dynamic throttling
- Escalation logic

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
- Abuse prevention
- Observability
- Feature expansion
- Mainnet readiness
