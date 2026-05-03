# Step 0.1.1 — Configuración externa y error envelope

## Objetivo

Agregar configuración externa y modelo de errores normalizado.

## Implementar

- paquete `internal/config`
- carga desde variables de entorno con defaults seguros para desarrollo
- validación de configuración crítica
- paquete `internal/httpapi` o equivalente para respuestas JSON
- error envelope `{code,message,details,request_id}`
- request ID middleware simple
- tests de config y errores

## Variables mínimas

- `SCAVIUM_FAUCET_BIND_ADDR`
- `SCAVIUM_FAUCET_PUBLIC_BASE_URL`
- `SCAVIUM_FAUCET_RPC_URL`
- `SCAVIUM_FAUCET_CHAIN_ID`
- `SCAVIUM_FAUCET_NETWORK_NAME`
- `SCAVIUM_FAUCET_SYMBOL`
- `SCAVIUM_FAUCET_EXPLORER_TX_URL`
- `SCAVIUM_FAUCET_AMOUNT_WEI`
- `SCAVIUM_FAUCET_COOLDOWN_SECONDS`
- `SCAVIUM_FAUCET_DRY_RUN`

## No implementar todavía

- lectura de archivos productivos
- secretos reales

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
git checkout -b faucet/step0.1.1-config-errors
git add cmd/scavium-faucet
git commit -m "faucet: step 0.1.1 add config and error envelope"
git checkout main
git merge --no-ff faucet/step0.1.1-config-errors
git branch -d faucet/step0.1.1-config-errors
```
