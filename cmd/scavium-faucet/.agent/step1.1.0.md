# Step 1.1.0 — Dominio faucet y contratos internos

## Objetivo

Definir modelos, estados e interfaces antes de implementar endpoints de claim.

## Implementar

- paquete `internal/domain` o `internal/faucet`
- `Claim`, `ClaimStatus`, `Transaction`, `FaucetConfig`, `FaucetStatus`
- validadores de address EVM/SCAVIUM usando go-ethereum/common cuando corresponda
- interfaces:
  - `ClaimStore`
  - `RateLimiter`
  - `Queue`
  - `Sender`
  - `CaptchaVerifier`
  - `RiskEngine`
- tests de estados y validación

## Criterio

No usar DB todavía. Solo contratos y tests.

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
git checkout -b faucet/step1.1.0-domain-contracts
git add cmd/scavium-faucet
git commit -m "faucet: step 1.1.0 add domain contracts"
git checkout main
git merge --no-ff faucet/step1.1.0-domain-contracts
git branch -d faucet/step1.1.0-domain-contracts
```
