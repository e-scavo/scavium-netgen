# Step 1.1.2 — Claim público in-memory + idempotencia

## Objetivo

Implementar flujo de claim sin blockchain real ni DB real, con contratos HTTP definitivos.

## Implementar

- `POST /api/v1/claim`
- `GET /api/v1/claim/{id}`
- aliases wallet-friendly:
  - `POST /api/v1/faucet/claim`
  - `GET /api/v1/faucet/claim/{id}`
- soporte de `Idempotency-Key`
- validación de address
- estados `received`, `validated`, `queued`, `rejected`
- store in-memory thread-safe para desarrollo/tests
- tests de claim, errores, idempotencia y address inválida

## No implementar todavía

- tx real
- DB real
- captcha real

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
git checkout -b faucet/step1.1.2-claim-api
git add cmd/scavium-faucet
git commit -m "faucet: step 1.1.2 add claim api"
git checkout main
git merge --no-ff faucet/step1.1.2-claim-api
git branch -d faucet/step1.1.2-claim-api
```
