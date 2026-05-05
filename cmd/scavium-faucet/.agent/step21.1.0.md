# Step 21.1.0 — Metrics Inventory and Export Plan

## Goal

Define and implement the smallest safe operator metrics export plan after Phase 20.

## Scope

- Inventory existing metrics and admin metrics JSON.
- Decide whether to add a protected Prometheus-compatible text endpoint without dependencies.
- Do not expose metrics publicly without admin protection unless explicitly documented and safe.
- Add tests for auth and output stability.
- Update observability docs.

## Must read first

- `.agent/rules.md`
- `cmd/scavium-faucet/internal/observability/metrics.go`
- `cmd/scavium-faucet/internal/httpapi/handler.go`
- `docs/scavium-faucet/runbook.md`
- `docs/scavium-faucet/security.md`

## Validation

```bash
gofmt -w <go-files-changed>
go test ./cmd/scavium-faucet/internal/observability/... ./cmd/scavium-faucet/internal/httpapi/... -count=1 -timeout 300s
go test ./... -timeout 300s
make build -B
```
