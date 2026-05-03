# Step 2.1.2 — Cola persistente y worker stub

## Objetivo

Hacer que los claims aceptados queden en cola persistente y puedan ser procesados por worker, todavía sin envío real.

## Implementar

- estados `queued`, `sending`, `sent`, `failed`
- worker configurable y apagado limpio con context
- batch control simple
- retry count y next attempt
- dead-letter por exceso de reintentos
- sender fake/stub para tests
- tests de transición de estados, retry y persistencia tras restart simulado

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
git checkout -b faucet/step2.1.2-persistent-queue
git add cmd/scavium-faucet
git commit -m "faucet: step 2.1.2 add persistent queue worker"
git checkout main
git merge --no-ff faucet/step2.1.2-persistent-queue
git branch -d faucet/step2.1.2-persistent-queue
```
