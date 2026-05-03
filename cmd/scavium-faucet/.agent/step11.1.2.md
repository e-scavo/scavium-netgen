# Step 11.1.2 — Persistent Faucet Read Service

Recommended executor: Codex in VSCode.

## Goal

Create a store-backed implementation of `faucet.ReadService` so public API claims persist in SQLite instead of memory.

## Required reads

- `cmd/scavium-faucet/internal/faucet/service.go`
- `cmd/scavium-faucet/internal/domain/interfaces.go`
- `cmd/scavium-faucet/internal/domain/models.go`
- `cmd/scavium-faucet/internal/store/sqlite/store.go`
- `cmd/scavium-faucet/internal/httpapi/handler.go`
- relevant tests in `cmd/scavium-faucet/internal/faucet` and `cmd/scavium-faucet/internal/store/sqlite`

## Required implementation

1. Add a persistent implementation, preferably in a new file:

   - `cmd/scavium-faucet/internal/faucet/persistent_service.go`

2. The service must implement `faucet.ReadService` and use:

   - `domain.ClaimStore`
   - `domain.RateLimiter`
   - `domain.QueueStore`
   - `config.Config`
   - clock injection for tests if useful.

3. `CreateClaim` must:

   - validate that faucet mode is active.
   - enforce address cooldown via `LastClaimByAddress`.
   - enforce rate limits using persistent `RateLimiter`.
   - create claim with amount/config.
   - use persisted idempotency if available through `CreateClaimWithIdempotency` or a small interface extension.
   - enqueue the claim persistently.
   - return the same response format as the in-memory service.

4. `AddressStatus` must:

   - read last claim by address.
   - calculate cooldown remaining.
   - report eligibility accurately.

5. `GetClaim` must:

   - read from SQL store.
   - map store not-found to `(ClaimResponse{}, false, nil)`.

6. Keep `InMemoryReadService` for tests/fallback only.

7. Add tests for:

   - create claim persists.
   - get claim after service recreation works.
   - idempotency returns same claim after service recreation.
   - cooldown blocks repeated claims.
   - address eligibility reflects cooldown.

## Allowed interface addition

If needed, add a narrow optional interface in `faucet` package:

```go
type idempotentClaimStore interface {
    CreateClaimWithIdempotency(ctx context.Context, claim domain.Claim, idempotencyKey string) (domain.Claim, error)
}
```

Do not force unrelated stores to implement it unless needed.

## Commands

```bash
git checkout -b faucet/step11.1.2-persistent-faucet-service
git status --short
go test ./cmd/scavium-faucet/internal/faucet ./cmd/scavium-faucet/internal/store/sqlite
go test ./...
```

## Git finalization

```bash
git status --short
git add cmd/scavium-faucet/internal/faucet cmd/scavium-faucet/internal/domain cmd/scavium-faucet/internal/store/sqlite
git commit -m "Add persistent faucet read service"
```

## Expected output

- Files read.
- Files modified.
- Tests run and results.
- Commit hash.
