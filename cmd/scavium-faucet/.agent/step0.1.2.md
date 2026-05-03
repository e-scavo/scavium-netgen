# Step 0.1.2 — Health, readiness, version y observabilidad base

## Objetivo

Completar superficie operativa mínima sin dependencias productivas.

## Implementar

- `GET /health`
- `GET /ready`
- `GET /api/v1/version`
- build info con variables sobreescribibles por `-ldflags`
- logger JSON mínimo usando biblioteca estándar o wrapper propio
- readiness con checks inyectables/mocks
- tests HTTP

## Criterio

`/ready` debe poder reportar degradación de DB/RPC/wallet/queue aunque aún sean checks stub.

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
git checkout -b faucet/step0.1.2-health-ready-version
git add cmd/scavium-faucet
git commit -m "faucet: step 0.1.2 add health readiness and version"
git checkout main
git merge --no-ff faucet/step0.1.2-health-ready-version
git branch -d faucet/step0.1.2-health-ready-version
```
