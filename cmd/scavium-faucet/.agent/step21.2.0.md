# Step 21.2.0 — Queue Blockchain Abuse Metrics Expansion

## Goal

Expand operator metrics around queue, worker, watcher, blockchain, abuse, blocklist, and token behavior without changing public contracts.

## Scope

- Add counters/gauges that can be derived safely from runtime/store state.
- Avoid high-cardinality unbounded labels.
- Do not expose wallet addresses, fingerprints, IPs, request bodies, idempotency keys, or secrets.
- Add tests for metrics increments/snapshots.
- Update docs and runbook.

## Validation

```bash
gofmt -w <go-files-changed>
go test ./cmd/scavium-faucet/internal/observability/... ./cmd/scavium-faucet/internal/worker/... ./cmd/scavium-faucet/internal/chain/... -count=1 -timeout 300s
go test ./... -timeout 300s
make build -B
```
