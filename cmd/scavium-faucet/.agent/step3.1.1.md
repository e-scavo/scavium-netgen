# Step 3.1.1 — Envío nativo SCAVIUM + nonce manager

## Objetivo

Implementar envío real de moneda nativa con nonce manager robusto.

## Implementar

- native transfer sender con go-ethereum
- nonce manager por wallet faucet
- lock transaccional para nonce
- gas policy mínima configurable
- balance guard antes de enviar
- registro de `txHash`
- tests con fake chain client y concurrencia de nonce

## Criterio

No depender de RPC real para tests.

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
git checkout -b faucet/step3.1.1-native-send-nonce
git add cmd/scavium-faucet
git commit -m "faucet: step 3.1.1 add native send and nonce manager"
git checkout main
git merge --no-ff faucet/step3.1.1-native-send-nonce
git branch -d faucet/step3.1.1-native-send-nonce
```
