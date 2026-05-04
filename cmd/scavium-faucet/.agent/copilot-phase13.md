# Copilot Chat Prompts — Phase 13 Production Hardening

Use these prompts in VSCode Copilot Chat.

Do not use Codex for these read-only prompts unless explicitly necessary.

---

## Prompt for step13.1.0 — hardening audit

```text
Inspect the current repository only.
Focus on cmd/scavium-faucet.
Do not modify files.

Verify current implementation and tests for:
- claim error handling and HTTP status/code mapping
- captcha/risk/rate-limit/cooldown/faucet mode errors
- CORS behavior and config
- SCAVIUM_FAUCET_DAILY_BUDGET_WEI loading and enforcement
- HTTP logging / request logging / metrics
- test coverage for the above

Do not read or use cmd/scavium-faucet-v0.
Return:
1. files read
2. confirmed gaps
3. exact files likely to modify
4. step-by-step implementation order
5. risks or constraints
```

---

## Prompt for step13.1.5 — final audit

```text
Inspect the current repository only.
Focus on cmd/scavium-faucet.
Do not modify files unless there is a tiny documentation-free typo in comments.

Verify Phase 13:
- captcha failures map to a non-500 response
- rate-limit/cooldown/risk/faucet paused errors map to precise normalized errors
- CORS is configurable and safe by default
- daily budget is enforced or explicitly rejected with tests
- minimal observability/logging exists and does not log secrets
- go test ./... passes
- go build ./cmd/scavium-faucet passes

Do not read or use cmd/scavium-faucet-v0.
Return a strict final verification summary and list any remaining required fixes.
```
