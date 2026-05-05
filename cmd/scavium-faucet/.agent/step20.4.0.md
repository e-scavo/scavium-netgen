# Step 20.4.0 — Persisted Blocklist and Claim-Path Enforcement

## Goal

Move admin blocklist storage to SQLite and enforce persisted blocklist entries during public claim intake.

## Must read first

- `.agent/rules.md`
- `cmd/scavium-faucet/internal/abuse/*`
- `cmd/scavium-faucet/internal/admin/admin.go`
- `cmd/scavium-faucet/internal/faucet/persistent_service.go`
- `cmd/scavium-faucet/internal/store/sqlite/store.go`
- `cmd/scavium-faucet/internal/httpapi/handler.go`
- relevant tests

## Scope

- Add persisted blocklist table if not present.
- Support `ip`, `address`, and `fingerprint` key types only.
- Canonicalize values consistently with existing abuse rules.
- Enforce blocklist before expensive downstream processing where safe.
- Return existing `claim_rejected` envelope with safe reason; do not expose raw blocklist key.
- Record safe abuse signal/metric if consistent with current patterns.
- Add tests for admin add/list/remove and claim-path rejection.
- Update security/API/runbook docs.

## Constraints

- No ASN/datacenter/VPN detection in this step.
- No allowlist in this step.
- No raw sensitive values in logs.

## Validation

```bash
gofmt -w <go-files-changed>
go test ./cmd/scavium-faucet/internal/abuse/... ./cmd/scavium-faucet/internal/faucet/... -count=1 -timeout 300s
go test ./cmd/scavium-faucet/internal/store/sqlite/... ./cmd/scavium-faucet/internal/httpapi/... -count=1 -timeout 300s
go test ./... -timeout 300s
make build -B
```

## Delivery

Partial ZIP only; include complete Git commands.
