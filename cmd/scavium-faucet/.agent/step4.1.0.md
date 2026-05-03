# Step 4.1.0 — Captcha, blocklist y circuit breaker base

## Objetivo

Agregar seguridad pública prioritaria sin implementar scoring avanzado completo.

## Implementar

- interface `CaptchaVerifier`
- implementación HTTP para proveedor configurable, con modo disabled/dev
- mock de captcha para tests
- blocklist por IP/address/fingerprint
- circuit breaker por actividad anormal simple
- modo pausa y mantenimiento desde config/store
- tests de captcha fail, blocklist y pausa

## No implementar todavía

- VPN/proxy provider real
- scoring complejo
- admin UI

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
git checkout -b faucet/step4.1.0-captcha-abuse-base
git add cmd/scavium-faucet
git commit -m "faucet: step 4.1.0 add captcha and abuse base"
git checkout main
git merge --no-ff faucet/step4.1.0-captcha-abuse-base
git branch -d faucet/step4.1.0-captcha-abuse-base
```
