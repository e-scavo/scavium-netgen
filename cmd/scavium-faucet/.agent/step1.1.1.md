# Step 1.1.1 — API pública status/config/eligibility

## Objetivo

Implementar endpoints públicos de solo lectura.

## Implementar

- `GET /api/v1/status`
- `GET /api/v1/config`
- `GET /api/v1/address/{address}/status`
- aliases wallet-friendly:
  - `GET /api/v1/faucet/status`
  - `GET /api/v1/faucet/config`
  - `GET /api/v1/faucet/address/{address}/eligibility`
- respuestas JSON estables
- tests de handlers

## Criterio

Sin DB real: usar servicios in-memory/fakes hasta el step de persistencia.

## Validación

```bash
gofmt -w cmd/scavium-faucet
go test ./...
go build ./cmd/scavium-faucet
```

## Git

```bash
git checkout main
git pull --ff-only
git checkout -b faucet/step1.1.1-public-read-api
git add cmd/scavium-faucet
git commit -m "faucet: step 1.1.1 add public read api"
git checkout main
git merge --no-ff faucet/step1.1.1-public-read-api
git branch -d faucet/step1.1.1-public-read-api
```
