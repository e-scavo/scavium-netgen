# Step 0.1.0 — Bootstrap compilable del faucet

## Objetivo

Crear la base compilable de `cmd/scavium-faucet` sin implementar aún lógica real de faucet.

## Leer antes de editar

- `go.mod`
- `Makefile`
- `internal/ethutil/*`
- `docs/scavium_faucet_public_features.md`

## Implementar

- `cmd/scavium-faucet/main.go`
- estructura interna mínima bajo `cmd/scavium-faucet/internal/`
- servidor HTTP básico con `net/http`
- wiring inicial de aplicación
- endpoint `GET /health`
- tests básicos del handler health

## No implementar todavía

- DB
- RPC real
- signer
- claim real
- nginx/systemd/certbot

## Validación

```bash
gofmt -w cmd/scavium-faucet
go test ./...
go build ./cmd/scavium-faucet
make build
```

## Git

```bash
git checkout main
git pull --ff-only
git checkout -b faucet/step0.1.0-bootstrap
# editar + validar
git add cmd/scavium-faucet
git commit -m "faucet: step 0.1.0 bootstrap server"
git checkout main
git merge --no-ff faucet/step0.1.0-bootstrap
git branch -d faucet/step0.1.0-bootstrap
```
