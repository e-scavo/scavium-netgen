# Step 3.1.2 — Receipt watcher y reconciliación

## Objetivo

Confirmar transacciones y corregir estados atascados.

## Implementar

- receipt watcher periódico
- confirmaciones mínimas configurables
- estados `confirmed` y `failed`
- reconciliación de claims `sending/sent` atascados
- pending transaction tracking básico
- tests con fake receipts y fake block height

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
git checkout -b faucet/step3.1.2-receipts-reconcile
git add cmd/scavium-faucet
git commit -m "faucet: step 3.1.2 add receipt watcher and reconciliation"
git checkout main
git merge --no-ff faucet/step3.1.2-receipts-reconcile
git branch -d faucet/step3.1.2-receipts-reconcile
```
