# Step 2.1.1 — Cooldown, rate limits y presupuesto base

## Objetivo

Implementar controles mínimos anti-drenaje.

## Implementar

- cooldown por address
- rate limit por IP
- rate limit por address
- límite diario global básico
- extracción segura de IP real detrás de proxy confiable
- config externa de límites
- tests de ventanas, límites y cooldown

## Criterio

La política debe ser simple, testeable y barata. Features avanzadas quedan para Phase 4.

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
git checkout -b faucet/step2.1.1-rate-limits
git add cmd/scavium-faucet
git commit -m "faucet: step 2.1.1 add cooldown and rate limits"
git checkout main
git merge --no-ff faucet/step2.1.1-rate-limits
git branch -d faucet/step2.1.1-rate-limits
```
