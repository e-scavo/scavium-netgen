# SCAVIUM Faucet — Agent Roadmap

## Objetivo operativo

Implementar `cmd/scavium-faucet` desde cero dentro de `scavium-netgen`, en la menor cantidad razonable de iteraciones, manteniendo microsteps secuenciales y testeables.

## Prioridad 24 h

Bloque crítico para tener faucet funcional:

1. Phase 0 — bootstrap arquitectónico
2. Phase 1 — API pública + claim core
3. Phase 2 — persistencia + rate limits + cola
4. Phase 3 — blockchain worker + nonce + receipts
5. Phase 4 — captcha/anti-abuse base
6. Phase 5 — integración wallet-friendly

Luego:

7. Phase 6 — admin mínimo
8. Phase 7 — frontend público mínimo
9. Phase 8 — documentación de código
10. Phase 9 — documentación de proyecto
11. Phase 10 — VPS/nginx/systemd/certbot

## Regla de avance

No avanzar al siguiente step si el anterior no compila y no pasa tests mínimos.
