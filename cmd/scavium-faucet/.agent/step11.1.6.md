# Step 11.1.6 — Full Integration Regression & Cleanup

Recommended executor: Copilot CLI first for mechanical audit, Codex only for fixes.

## Goal

Run a final integration audit for Phase 11 and fix only concrete defects found by tests or direct code inspection.

## Required checks

1. Verify no production runtime path still defaults to `faucet.NewInMemoryReadService`.
2. Verify SQLite is opened from config in `app.New`.
3. Verify migrations run automatically.
4. Verify public claims are persisted and enqueued.
5. Verify worker starts in dry-run and can ack queued claims.
6. Verify admin token is passed into admin middleware.
7. Verify `/ready` is not only stub checks.
8. Verify no secrets are logged.
9. Verify no code was copied from `cmd/scavium-faucet-v0`.

## Commands

```bash
git checkout -b faucet/step11.1.6-runtime-regression-cleanup
git status --short
grep -R "NewInMemoryReadService" -n cmd/scavium-faucet || true
grep -R "StubOK" -n cmd/scavium-faucet/internal || true
grep -R "PrivateKeyHex\|SCAVIUM_FAUCET_PRIVATE_KEY" -n cmd/scavium-faucet/internal | cat
go test ./...
```

## Fix policy

- If all checks pass and no files change, do not commit.
- If fixes are required, keep them minimal and test-driven.
- Do not modify documentation.

## Optional smoke test

If the binary can be run locally:

```bash
SCAVIUM_FAUCET_DRY_RUN=true \
SCAVIUM_FAUCET_DATABASE_PATH=/tmp/scavium-faucet-smoke.db \
go run ./cmd/scavium-faucet
```

Then in another terminal:

```bash
curl -s http://127.0.0.1:18080/health
curl -s http://127.0.0.1:18080/ready
curl -s http://127.0.0.1:18080/api/v1/config
curl -s -X POST http://127.0.0.1:18080/api/v1/claim \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: smoke-1' \
  -d '{"address":"0x0000000000000000000000000000000000000001"}'
```

## Git finalization

If files changed:

```bash
git status --short
git add cmd/scavium-faucet
git commit -m "Stabilize faucet persistent runtime integration"
```

## Expected output

- Full checklist result.
- Tests run and results.
- Whether any fixes were needed.
- Commit hash if applicable.
