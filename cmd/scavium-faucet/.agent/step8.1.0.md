# Step 8.1.0 — Documentación de código

## Objetivo

Agregar comentarios GoDoc útiles en paquetes, tipos e interfaces principales sin sobre-documentar.

## Implementar

- comentarios de paquetes principales
- comentarios de interfaces públicas internas
- comentarios en modelos/estados críticos
- ejemplos mínimos si corresponde

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
git checkout -b faucet/step8.1.0-code-docs
git add cmd/scavium-faucet
git commit -m "faucet: step 8.1.0 document code"
git checkout main
git merge --no-ff faucet/step8.1.0-code-docs
git branch -d faucet/step8.1.0-code-docs
```
