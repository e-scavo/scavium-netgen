# Step 2.1.0 — Persistencia SQLite MVP + migraciones

## Objetivo

Agregar persistencia local MVP, suficiente para desarrollo y beta chica.

## Implementar

- paquete `internal/store/sqlite` o equivalente
- migraciones bajo `cmd/scavium-faucet/migrations`
- tablas mínimas:
  - `requests`
  - `transactions`
  - `rate_limits`
  - `config`
- constraints de idempotencia
- índices por address/estado/fecha
- tests con DB temporal

## Dependencias

Usar SQLite implica dependencia. Elegir una opción razonable y justificar en commit/nota si se agrega al `go.mod`.

## Validación

```bash
gofmt -w cmd/scavium-faucet
go mod tidy
go test ./...
go build ./cmd/scavium-faucet
```

## Git

```bash
git checkout main
git pull --ff-only
git checkout -b faucet/step2.1.0-sqlite-store
git add go.mod go.sum cmd/scavium-faucet
git commit -m "faucet: step 2.1.0 add sqlite store"
git checkout main
git merge --no-ff faucet/step2.1.0-sqlite-store
git branch -d faucet/step2.1.0-sqlite-store
```
