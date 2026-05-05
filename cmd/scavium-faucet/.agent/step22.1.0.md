# Step 22.1.0 — RPC Failover Foundation

## Goal

Add conservative RPC failover support without changing transaction semantics.

## Scope

- Add config for optional secondary RPC URLs.
- Validate chain ID for any selected RPC before use.
- Keep primary RPC behavior unchanged when no secondary is configured.
- Add tests using fake clients where possible.
- Document limitations.

## Constraints

- No load balancing complexity.
- No multi-instance/HA redesign.
- No production RPC secrets in repo.

## Validation

```bash
gofmt -w <go-files-changed>
go test ./cmd/scavium-faucet/internal/config/... ./cmd/scavium-faucet/internal/chain/... ./cmd/scavium-faucet/internal/app/... -count=1 -timeout 300s
go test ./... -timeout 300s
make build -B
```
