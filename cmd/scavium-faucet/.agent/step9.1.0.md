# Step 9.1.0 — Documentación del proyecto

## Objetivo

Crear documentación troncal del faucet en `docs/scavium-faucet/`.

## Implementar

- `docs/scavium-faucet/index.md`
- `docs/scavium-faucet/architecture.md`
- `docs/scavium-faucet/api.md`
- `docs/scavium-faucet/configuration.md`
- `docs/scavium-faucet/runbook.md`
- `docs/scavium-faucet/security.md`
- actualizar incrementalmente `README.md` si corresponde

## Regla

No borrar ni resumir `docs/scavium_faucet_public_features.md`; tratarlo como documento fuente troncal.

## Validación

```bash
go test ./...
go build ./cmd/scavium-faucet
```

## Git

```bash
git checkout main
git pull --ff-only
git checkout -b faucet/step9.1.0-project-docs
git add README.md docs cmd/scavium-faucet
git commit -m "faucet: step 9.1.0 add project documentation"
git checkout main
git merge --no-ff faucet/step9.1.0-project-docs
git branch -d faucet/step9.1.0-project-docs
```
