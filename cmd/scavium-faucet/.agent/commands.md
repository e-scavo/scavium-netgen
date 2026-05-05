# SCAVIUM Faucet — Agent Commands After Phase 19

## Initial inventory for every phase

```bash
git status --short
git branch --show-current
cat go.mod
cat Makefile
sed -n '1,260p' docs/scavium_faucet_public_features.md
sed -n '1,460p' docs/scavium_faucet_public_phase-roadmap-post14.md
sed -n '1,260p' docs/scavium-faucet/implementation-roadmap-after-phase19.md
find cmd/scavium-faucet -maxdepth 5 -type f | sort
find docs/scavium-faucet -maxdepth 4 -type f | sort
find scripts -maxdepth 3 -type f | sort
```

## Focused package inventory

Admin persistence:

```bash
sed -n '1,260p' cmd/scavium-faucet/internal/admin/admin.go
sed -n '1,260p' cmd/scavium-faucet/internal/httpapi/handler.go
sed -n '1,260p' cmd/scavium-faucet/internal/store/sqlite/store.go
sed -n '1,260p' cmd/scavium-faucet/internal/domain/interfaces.go
find cmd/scavium-faucet/migrations -type f | sort -V
```

Observability:

```bash
sed -n '1,260p' cmd/scavium-faucet/internal/observability/metrics.go
sed -n '1,260p' cmd/scavium-faucet/internal/observability/logger.go
sed -n '1,260p' cmd/scavium-faucet/internal/httpapi/logging.go
sed -n '1,260p' docs/scavium-faucet/runbook.md
```

Blockchain/runtime:

```bash
sed -n '1,280p' cmd/scavium-faucet/internal/chain/chain.go
sed -n '1,280p' cmd/scavium-faucet/internal/chain/sender.go
sed -n '1,280p' cmd/scavium-faucet/internal/chain/watcher.go
sed -n '1,260p' cmd/scavium-faucet/internal/config/config.go
sed -n '1,260p' cmd/scavium-faucet/internal/app/app.go
```

Deployment/scripts:

```bash
find docs/scavium-faucet/deployment -type f -maxdepth 2 -print -exec sed -n '1,220p' {} \;
find scripts -type f -maxdepth 2 -print -exec sed -n '1,220p' {} \;
```

## Standard validation

```bash
gofmt -w <go-files-changed>
go test ./...
make build -B
```

If `make build -B` fails because of an unrelated existing tool, run and report:

```bash
go test ./...
go build ./cmd/scavium-faucet
```

For long SQLite-backed tests, the expected safe timeout is:

```bash
go test ./... -timeout 300s
```

## Focused validation examples

```bash
go test ./cmd/scavium-faucet/internal/admin/... -count=1 -timeout 300s
go test ./cmd/scavium-faucet/internal/store/sqlite/... -count=1 -timeout 300s
go test ./cmd/scavium-faucet/internal/httpapi/... -count=1 -timeout 300s
go test ./cmd/scavium-faucet/internal/app/... ./cmd/scavium-faucet/internal/faucet/... -count=1 -timeout 300s
```

## Packaging partial ZIP only

From repo root, after determining changed files:

```bash
git status --short
mkdir -p /tmp/scavium-partial
# Copy changed/created files preserving paths, then:
(cd /tmp/scavium-partial && zip -r /tmp/scavium-phaseXX.Y-name.zip .)
```

Never package the full project.

## Git commands template

```bash
git checkout main
git pull --ff-only
git checkout -b phase-XX.Y-short-name

git add <changed-files>
git commit -m "phase XX.Y short description"

git checkout main
git merge phase-XX.Y-short-name
git branch -d phase-XX.Y-short-name
```
