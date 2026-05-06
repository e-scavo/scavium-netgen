# Step 29.3.0 — Phase 29 Campaign Enforcement and Public Claim Integration

## Goal

Integrate campaigns/allowlists/invitation codes into claim flow safely.

## Mandatory files to read first

Confirm the exact files read in the final response. At minimum read:

```bash
git status --short
cat go.mod
cat Makefile
sed -n '1,260p' docs/scavium_faucet_public_features.md
sed -n '1,320p' docs/scavium-faucet/implementation-roadmap-after-phase19.md
find cmd/scavium-faucet -maxdepth 5 -type f | sort
find docs/scavium-faucet -maxdepth 4 -type f | sort
find scripts -maxdepth 2 -type f | sort
```

Then read every code, test, migration, web, docs, deployment, and script file touched by this step before editing it.


## Non-negotiable guardrails

- The ZIP/worktree is the only source of truth.
- Keep changes small, production-safe, and backward compatible.
- Do not break `POST /api/v1/claim` or normalized error envelopes.
- Do not introduce heavy dependencies, secrets, broad refactors, or unplanned features.
- Keep admin-only surfaces protected.
- Sanitize data at domain/service boundaries, not only in HTTP handlers.
- Keep metrics labels bounded and never derived directly from untrusted free-form input.
- Update documentation under `docs/scavium-faucet/*.md` whenever behavior changes.
- Package only changed/new files into the partial ZIP.

## Scope

- Add optional campaign/invitation inputs without breaking existing claims.
- Enforce campaign budgets/scopes deterministically and token-aware where applicable.
- Preserve normalized error envelopes and do not leak whether unrelated private allowlist entries exist.
- Add tests for public campaigns, private campaigns, invalid codes, exhausted budgets, idempotency, and legacy clients.

## Explicitly out of scope

- Do not skip ahead to later phases.
- Do not implement deferred Stage 4/professional-scale items unless this step explicitly says so.
- Do not include unchanged files in the delivery ZIP.

## Required validation

Run all applicable focused tests for the changed packages, then run the full baseline:

```bash
gofmt -w <go-files-changed>
go test ./... -timeout 300s
go build ./cmd/scavium-faucet
bash -n scripts/*.sh
./scripts/scavium-faucet-backup.sh --plan

TMP_DIR=$(mktemp -d)
if command -v sqlite3 >/dev/null 2>&1; then
  sqlite3 "$TMP_DIR/scavium-faucet.db" 'VACUUM;'
else
  printf 'fallback\n' > "$TMP_DIR/scavium-faucet.db"
fi

SCAVIUM_FAUCET_DATABASE_PATH="$TMP_DIR/scavium-faucet.db" \
SCAVIUM_FAUCET_BACKUP_DIR="$TMP_DIR/backups" \
SCAVIUM_FAUCET_BACKUP_ID="test" \
./scripts/scavium-faucet-backup.sh --execute

SCAVIUM_FAUCET_RESTORE_BUNDLE="$TMP_DIR/backups/scavium-faucet-backup-test.tar.gz" \
./scripts/scavium-faucet-restore.sh --plan
rm -rf "$TMP_DIR"
```

If `make build -B` is relevant and succeeds, report it too. Never run restore-plan with a fake bundle path.


## Git workflow

Use a dedicated branch and include the exact commands in the final response:

```bash
git checkout main
git pull --ff-only
git checkout -b <branch>

git status --short
git add <changed-or-created-files>
git commit -m "<message>"

git checkout main
git merge <branch>
git branch -d <branch>
```

Recommended branch name: `phase-29.3-29-campaign-enforcement-public-claim-integ`.

## Delivery

Return a partial ZIP containing only files changed or created by this step. The final response must include:

1. Files read.
2. Short analysis.
3. Implementation summary.
4. Validation commands and results.
5. Partial ZIP link.
6. Complete Git commands.

