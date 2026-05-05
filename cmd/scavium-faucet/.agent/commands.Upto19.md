# SCAVIUM Faucet — Commands

## Lectura inicial obligatoria

```bash
git status --short
git branch --show-current
cat go.mod
cat Makefile
find cmd/scavium-faucet -maxdepth 4 -type f | sort
find internal -maxdepth 3 -type f | sort
sed -n '1,260p' docs/scavium_faucet_public_features.md
```

## Validación estándar por step

```bash
gofmt -w <archivos-go-intervenidos>
go test ./...
make build
```

Si `make build` falla por una herramienta existente no relacionada, reportar exactamente el error y mantener como validación mínima:

```bash
go test ./...
go build ./cmd/scavium-faucet
```

## Validación focal recomendada

```bash
go test ./cmd/scavium-faucet/...
go build ./cmd/scavium-faucet
```

## Go mod

Solo ejecutar cuando se agreguen dependencias nuevas:

```bash
go mod tidy
go test ./...
make build
```

Preferir biblioteca estándar siempre que sea razonable. Agregar dependencias solo si reducen riesgo y costo.

## Prohibido en codificación inicial

No ejecutar comandos productivos de VPS/nginx/certbot/systemd.
No usar claves privadas reales.
No hacer requests a RPC productivo como parte de tests.

## Git por step

Inicio:

```bash
git checkout main
git pull --ff-only
git checkout -b faucet/stepX.Y.Z-short-name
```

Cierre:

```bash
git status --short
git add <archivos>
git commit -m "faucet: step X.Y.Z short description"
git checkout main
git merge --no-ff faucet/stepX.Y.Z-short-name
git branch -d faucet/stepX.Y.Z-short-name
```
