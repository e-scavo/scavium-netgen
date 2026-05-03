# Step 6.1.0 — Admin API mínimo

## Objetivo

Agregar administración mínima operable por API, sin frontend admin elaborado.

## Implementar

- autenticación admin simple por token fuerte desde config para MVP
- roles mínimos si el costo es bajo: `admin`, `operator`, `viewer`
- endpoints internos para:
  - dashboard resumen
  - listar claims
  - ver detalle claim
  - pausar/reanudar faucet
  - mantenimiento on/off
  - reintentar claim
  - cancelar claim antes del envío
  - gestionar blocklist básica
- audit log básico
- tests de auth y permisos

## Criterio

2FA y sesiones completas pueden quedar para fase profesional posterior si comprometen el límite de 24 h.

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
git checkout -b faucet/step6.1.0-admin-api
git add cmd/scavium-faucet
git commit -m "faucet: step 6.1.0 add minimal admin api"
git checkout main
git merge --no-ff faucet/step6.1.0-admin-api
git branch -d faucet/step6.1.0-admin-api
```
