# Step 10.1.0 — Deployment VPS/nginx/systemd/certbot

## Objetivo

Preparar despliegue productivo al final del desarrollo, separado de la codificación.

## Implementar

- systemd unit template
- nginx server block template
- guía certbot/ACME
- firewall guide
- deploy script seguro
- rollback guide
- environment file example sin secretos reales

## Regla

No ejecutar comandos productivos automáticamente desde Codex. Generar archivos/scripts y documentación revisable.

## Validación

```bash
go test ./...
go build ./cmd/scavium-faucet
```

## Git

```bash
git checkout main
git pull --ff-only
git checkout -b faucet/step10.1.0-deployment
git add docs scripts cmd/scavium-faucet
git commit -m "faucet: step 10.1.0 add deployment assets"
git checkout main
git merge --no-ff faucet/step10.1.0-deployment
git branch -d faucet/step10.1.0-deployment
```
