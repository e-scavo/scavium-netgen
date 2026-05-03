# SCAVIUM Faucet — Codex Rules

## Contexto

Proyecto: `scavium-netgen`
Módulo Go: `scavium-netgen`
Proyecto nuevo: `cmd/scavium-faucet`
Fuente funcional troncal: `docs/scavium_faucet_public_features.md`

El faucet público SCAVIUM debe implementarse como aplicación Go dentro de `cmd/scavium-faucet`, reutilizando cuando corresponda utilidades existentes de `internal/ethutil`, pero sin romper herramientas existentes del repo.

## Reglas críticas

1. El ZIP/worktree real es la única fuente de verdad.
2. Antes de editar, leer:
   - `go.mod`
   - `Makefile`
   - `README.md`
   - `docs/scavium_faucet_public_features.md`
   - paquetes relevantes bajo `internal/`
3. No modificar herramientas existentes salvo necesidad mínima y justificada.
4. La implementación nueva vive principalmente en `cmd/scavium-faucet/`.
5. No configurar VPS, nginx, certbot ni systemd productivo durante fases de codificación.
6. No agregar secretos reales, claves privadas, RPC privados ni dominios reales.
7. No loguear secretos.
8. Todo código debe compilar con `go test ./...`.
9. Cada step debe ser pequeño, cerrado e incremental.
10. Cada step debe incluir tests razonables para la funcionalidad agregada.
11. Mantener compatibilidad con `make build`.
12. Usar configuración externa por variables de entorno y/o archivo config, sin valores productivos hardcodeados.
13. Para blockchain real, aislar detrás de interfaces testeables.
14. Para captcha/RPC/DB, usar mocks/fakes en tests cuando corresponda.
15. Si una feature profesional excede el MVP de 24 h, dejar contrato/interfaces y TODO documentado, no improvisar implementación incompleta.
16. No documentar en `docs/` durante los steps de código, salvo que el step lo indique explícitamente.
17. No reescribir documentación troncal; cuando llegue documentación, actualizar incrementalmente.
18. No leer, copiar ni derivar implementaciones de `cmd/scavium-faucet-v0` solo bajo instrucciones explícitas.

## Estilo de arquitectura

Preferir separación interna dentro de `cmd/scavium-faucet/internal/`:

```text
cmd/scavium-faucet/
├─ main.go
├─ internal/
│  ├─ app/
│  ├─ config/
│  ├─ domain/
│  ├─ httpapi/
│  ├─ store/
│  ├─ faucet/
│  ├─ chain/
│  ├─ abuse/
│  ├─ admin/
│  ├─ frontend/
│  └─ observability/
└─ migrations/
```

Puede ajustarse si el árbol real lo justifica, pero no mezclar toda la lógica en `main.go`.

## Contratos mínimos

API pública bajo `/api/v1`:

- `GET /health`
- `GET /ready`
- `GET /api/v1/status`
- `GET /api/v1/config`
- `POST /api/v1/claim`
- `GET /api/v1/claim/{id}`
- `GET /api/v1/address/{address}/status`
- `GET /api/v1/address/{address}/history`
- `GET /api/v1/version`

Endpoints wallet-friendly:

- `GET /api/v1/faucet/status`
- `GET /api/v1/faucet/config`
- `GET /api/v1/faucet/address/{address}/eligibility`
- `POST /api/v1/faucet/claim`
- `GET /api/v1/faucet/claim/{id}`

Errores normalizados:

```json
{
  "code": "string",
  "message": "string",
  "details": {},
  "request_id": "string"
}
```

Estados de claim:

```text
received, validated, queued, sending, sent, confirmed, failed, rejected, paused
```

## Git obligatorio por step

Cada step debe ejecutarse en branch propia:

```bash
git checkout main
git pull --ff-only
git checkout -b faucet/stepX.Y.Z-short-name
```

Al finalizar:

```bash
git status --short
git add <archivos>
git commit -m "faucet: step X.Y.Z short description"
git checkout main
git merge --no-ff faucet/stepX.Y.Z-short-name
git branch -d faucet/stepX.Y.Z-short-name
```

Si el repo usa otra rama base, reemplazar `main` por la rama base real detectada.
