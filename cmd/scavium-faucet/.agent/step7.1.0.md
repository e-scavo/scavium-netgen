# Step 7.1.0 — Frontend público mínimo embebido

## Objetivo

Agregar web pública mínima para solicitar fondos sin bloquear la integración wallet.

## Implementar

- servir frontend estático desde el binario o carpeta `web/`
- landing simple
- formulario address + captcha/challenge
- consulta status/config
- seguimiento de claim
- link a explorer
- mensajes de cooldown/pausa/mantenimiento
- tests básicos de assets/handlers si aplica

## Criterio

UI simple y funcional. El refinamiento visual puede quedar para posterior.

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
git checkout -b faucet/step7.1.0-public-frontend
git add cmd/scavium-faucet
git commit -m "faucet: step 7.1.0 add public frontend"
git checkout main
git merge --no-ff faucet/step7.1.0-public-frontend
git branch -d faucet/step7.1.0-public-frontend
```
