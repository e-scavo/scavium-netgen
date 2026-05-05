# Faucet Server Pública SCAVIUM — Features por prioridad

## 1. Core público indispensable

| Feature | Descripción |
|---|---|
| **API pública versionada** | Endpoints bajo `/api/v1`. |
| **Frontend público** | Pantalla web simple para solicitar fondos. |
| **HTTPS obligatorio vía nginx** | El backend Go queda interno; nginx expone el sitio. |
| **Backend Go aislado** | Escucha en `127.0.0.1:<puerto-no-standard>`. |
| **systemd daemon** | Servicio persistente con restart automático. |
| **Configuración externa** | RPC, chain ID, límites, montos, captcha, DB, puertos, dominios. |
| **Healthcheck** | `GET /health`. |
| **Readiness check** | `GET /ready` validando DB, RPC, wallet, cola y saldo. |
| **Endpoint público de estado** | Faucet activo, pausado, mantenimiento o sin fondos. |
| **Endpoint público de parámetros** | Monto, cooldown, red, símbolo, explorer, límites visibles. |
| **Solicitud pública de faucet** | Usuario ingresa wallet y pide fondos. |
| **Transferencia on-chain** | Firma y envío desde wallet faucet controlada. |
| **Respuesta con tracking** | Devuelve ID de solicitud, estado y `txHash` si corresponde. |

---

## 2. Seguridad pública prioritaria

| Feature | Descripción |
|---|---|
| **Captcha obligatorio** | Turnstile, hCaptcha o reCAPTCHA. |
| **Rate limit por IP** | Primera barrera contra bots. |
| **Rate limit por address** | Evita drenar usando la misma wallet. |
| **Rate limit por fingerprint** | Reduce abuso por rotación de IP. |
| **Cooldown público** | Tiempo mínimo entre reclamos. |
| **Límite diario por IP** | Cuota máxima por origen. |
| **Límite diario por address** | Cuota máxima por wallet. |
| **Límite global diario** | Máximo total entregado por día. |
| **Límite global horario** | Evita drenajes rápidos. |
| **Circuit breaker** | Pausa automática ante actividad anormal. |
| **Modo pausa** | Detiene entregas sin apagar el servicio. |
| **Modo mantenimiento** | Sitio online, faucet inactivo. |
| **Balance guard** | Pausa si la wallet baja de un umbral. |
| **Validación estricta de address** | Formato, checksum y red compatible. |
| **Validación de chain ID** | Impide enviar en red incorrecta. |
| **Request body limit** | Evita payloads abusivos. |
| **Timeouts HTTP** | Protege contra conexiones lentas. |
| **CORS restringido** | Solo dominio oficial. |
| **Headers seguros** | CSP, HSTS, X-Frame-Options, X-Content-Type-Options. |
| **Trusted proxy** | El backend solo confía en headers de nginx. |
| **IP real segura** | Obtención correcta desde `X-Forwarded-For`. |
| **Bloqueo de IP** | Manual y automático. |
| **Bloqueo de address** | Manual y automático. |
| **Bloqueo por ASN/país opcional** | Para abuso masivo o ataques. |
| **Greylist** | Solicitudes sospechosas quedan retenidas. |
| **Lista blanca opcional** | Para campañas cerradas o testers. |
| **Lista negra de dominios/referrers** | Si se integra con origen web. |

---

## 3. Anti-bot y anti-abuso avanzado

| Feature | Descripción |
|---|---|
| **Fingerprint liviano** | Señal adicional sin depender solo de IP. |
| **Detección de VPN/proxy/datacenter** | Score de riesgo opcional. |
| **Score de riesgo** | Combina IP, address, fingerprint, frecuencia y comportamiento. |
| **Reglas por score** | Permitir, reducir monto, demorar, rechazar o mandar a revisión. |
| **Proof-of-work opcional** | Desafío liviano contra bots. |
| **Queue delay aleatorio** | Hace menos rentable automatizar reclamos. |
| **Device cooldown** | Cooldown por dispositivo/fingerprint. |
| **Address clustering** | Detecta muchas wallets desde un mismo origen. |
| **Burst detection** | Detecta picos repentinos. |
| **Rotating IP detection** | Relaciona patrones entre múltiples IPs. |
| **Reputación de address** | Penaliza wallets abusivas. |
| **Reputación de IP** | Penaliza orígenes problemáticos. |
| **Reputación de fingerprint** | Penaliza dispositivos recurrentes. |
| **Reglas dinámicas** | Cambios de política sin redeploy. |
| **Cooldown adaptativo** | Más riesgo implica más espera. |
| **Monto adaptativo** | Más riesgo implica menor entrega. |
| **Manual review** | Admin aprueba solicitudes sospechosas. |
| **Honeypot fields** | Campos invisibles para detectar bots simples. |
| **JS challenge opcional** | Validación adicional en frontend. |
| **User-agent policy** | Rechazo de agentes claramente automatizados. |

---

## 4. Modelo sin login obligatorio

| Feature | Descripción |
|---|---|
| **Faucet sin cuenta** | Uso público con wallet + captcha. |
| **Tracking por solicitud** | ID público consultable. |
| **Historial por address** | Estado de reclamos recientes. |
| **Login opcional** | Para usuarios confiables, testers o admins. |
| **Login admin separado** | Administración protegida. |
| **Login con wallet opcional** | Firma de mensaje para obtener mayor cuota. |
| **Cuenta verificada opcional** | Email/GitHub/Discord para campañas o límites especiales. |
| **Roles administrativos** | `admin`, `operator`, `viewer`. |
| **2FA admin** | TOTP obligatorio para administración. |
| **Sesiones admin seguras** | Cookies `Secure`, `HttpOnly`, `SameSite`. |
| **Revocación de sesiones admin** | Logout global. |
| **Auditoría de login admin** | Intentos exitosos/fallidos. |

---

## 5. Motor faucet público

| Feature | Descripción |
|---|---|
| **Monto base configurable** | Entrega estándar pública. |
| **Monto mínimo/máximo** | Límites duros. |
| **Monto por red** | Devnet/testnet/mainnet privada si aplica. |
| **Monto por token** | Nativo SCAVIUM y tokens compatibles. |
| **Límite por ventana** | Minuto, hora, día, semana. |
| **Límite por address** | Control principal. |
| **Límite por IP** | Control secundario. |
| **Límite por fingerprint** | Control adicional. |
| **Límite global** | Presupuesto total del faucet. |
| **Presupuesto diario** | Máximo entregado por día. |
| **Presupuesto por campaña** | Para eventos o lanzamientos. |
| **Cola persistente** | Solicitudes sobreviven reinicios. |
| **Worker de transacciones** | Procesa envíos de forma ordenada. |
| **Nonce manager robusto** | Evita nonces duplicados. |
| **Lock transaccional de nonce** | Necesario incluso con un solo proceso. |
| **Retry controlado** | Reintento ante error temporal. |
| **Backoff exponencial** | Evita saturar RPC. |
| **Estados normalizados** | `received`, `validated`, `queued`, `sending`, `sent`, `confirmed`, `failed`, `rejected`, `paused`. |
| **Idempotencia** | Evita doble envío por refresh/retry. |
| **Receipt watcher** | Verifica confirmaciones. |
| **Confirmaciones mínimas** | Configurable. |
| **Reconciliación periódica** | Corrige estados atascados. |
| **Dead-letter queue** | Solicitudes que fallan repetidamente. |
| **Reproceso administrativo** | Reintentar manualmente. |
| **Cancelación administrativa** | Cancelar antes del envío. |
| **Dry-run mode** | Simular sin enviar fondos. |
| **Drip scheduling** | Entregas distribuidas en el tiempo. |
| **Batch control** | Evita enviar demasiadas tx simultáneas. |

---

## 6. Blockchain / RPC

| Feature | Descripción |
|---|---|
| **RPC primario configurable** | Nodo principal SCAVIUM. |
| **RPC secundario** | Failover. |
| **RPC healthcheck** | Detecta caída o degradación. |
| **Chain ID enforcement** | Seguridad crítica. |
| **Gas estimation** | Estimación antes de enviar. |
| **Gas policy** | Máximo gas permitido. |
| **Fee policy** | Legacy gas price o EIP-1559 si aplica. |
| **Balance checker** | Verifica fondos antes de enviar. |
| **Native transfer** | Envío de moneda nativa. |
| **ERC-20 transfer** | Envío de token si aplica. |
| **ABI versionada** | Para contratos. |
| **Token metadata** | Símbolo, decimals, contract address. |
| **Explorer URL** | Links públicos a transacciones. |
| **Receipt parser** | Estado, gas usado, bloque. |
| **Pending transaction tracking** | Control de tx no confirmadas. |
| **Nonce recovery** | Recalcular nonce ante desincronización. |
| **Replacement transaction policy** | Reemplazar tx atascada si la red lo permite. |
| **Reorg tolerance** | Esperar confirmaciones suficientes. |

---

## 7. Persistencia

| Feature | Descripción |
|---|---|
| **PostgreSQL recomendado** | Mejor para producción pública. |
| **SQLite solo MVP** | Aceptable para beta chica. |
| **Migraciones versionadas** | Schema controlado. |
| **Tabla `requests`** | Solicitudes públicas. |
| **Tabla `transactions`** | Tx on-chain asociadas. |
| **Tabla `rate_limits`** | Contadores por ventana. |
| **Tabla `fingerprints`** | Señales anti-abuso. |
| **Tabla `risk_events`** | Eventos sospechosos. |
| **Tabla `blocklist`** | IP, address, fingerprint, ASN. |
| **Tabla `allowlist`** | Excepciones y campañas cerradas. |
| **Tabla `campaigns`** | Campañas públicas/privadas. |
| **Tabla `admin_users`** | Usuarios administrativos. |
| **Tabla `admin_sessions`** | Sesiones admin. |
| **Tabla `audit_logs`** | Acciones críticas. |
| **Tabla `config`** | Config dinámica. |
| **Tabla `daily_budgets`** | Presupuesto consumido. |
| **Índices por address/IP/estado/fecha** | Performance operativa. |
| **Constraints de unicidad** | Idempotencia real. |
| **Retención de datos** | Limpieza automática de datos antiguos. |
| **Backups automáticos** | Dumps programados. |
| **Restore documentado** | Procedimiento probado. |

---

## 8. API pública

| Endpoint | Descripción |
|---|---|
| `GET /api/v1/status` | Estado público del faucet. |
| `GET /api/v1/config` | Monto, cooldown, red, símbolo, explorer. |
| `POST /api/v1/claim` | Solicitud de fondos. |
| `GET /api/v1/claim/{id}` | Estado de una solicitud. |
| `GET /api/v1/address/{address}/status` | Cooldown y elegibilidad. |
| `GET /api/v1/address/{address}/history` | Historial limitado por address. |
| `GET /api/v1/version` | Versión, commit, build date. |
| `GET /health` | Liveness. |
| `GET /ready` | Readiness. |
| `GET /metrics` | Métricas internas protegidas. |

| Feature | Descripción |
|---|---|
| **Errores normalizados** | `code`, `message`, `details`, `request_id`. |
| **Paginación** | Para historial. |
| **OpenAPI** | Contrato formal. |
| **API versioning** | Compatibilidad futura. |
| **Idempotency header** | `Idempotency-Key`. |

---

## 9. Admin privado

| Feature | Descripción |
|---|---|
| **Panel admin** | UI privada o API interna. |
| **Dashboard** | Saldo, entregado hoy, cola, errores, actividad. |
| **Listado de claims** | Filtros por estado, address, IP, riesgo. |
| **Detalle de claim** | Toda la trazabilidad. |
| **Aprobar/rechazar claim** | Para greylist/manual review. |
| **Reintentar claim** | Reproceso controlado. |
| **Cancelar claim** | Antes del envío. |
| **Pausar faucet** | Acción inmediata. |
| **Cambiar modo mantenimiento** | Estado público controlado. |
| **Editar monto** | Config dinámica. |
| **Editar cooldown** | Config dinámica. |
| **Editar presupuesto diario** | Control de gasto. |
| **Editar reglas de riesgo** | Anti-abuso dinámico. |
| **Gestionar blocklist** | IP/address/fingerprint/ASN. |
| **Gestionar allowlist** | Excepciones y campañas. |
| **Gestionar campañas** | Crear, pausar, cerrar. |
| **Ver auditoría** | Acciones admin. |
| **Export CSV** | Claims, tx, auditoría. |
| **Forzar reconciliación** | Job manual. |
| **Ver estado RPC** | Latencia, último bloque, errores. |
| **Ver estado wallet** | Saldo, nonce, pending tx. |

### Phase 18 admin-control closure status

Phase 18 closes a production-safe subset of the broader admin feature list without changing public faucet contracts. Implemented and validated scope:

| Capability | Phase 18 status | Notes |
|---|---|---|
| Runtime-effective faucet mode | Implemented | `active`, `paused`, and `maintenance` are validated and propagated to the live claim path. |
| Runtime and metrics visibility | Implemented | `/api/v1/admin/runtime`, `/api/v1/admin/metrics`, and `/api/v1/admin/dashboard` remain admin-token protected. |
| Queue visibility | Implemented with current scope | Admin-safe queue snapshots expose counts and limited items; the broader SQLite-backed admin service remains deferred. |
| Queue/claim retry and cancel | Implemented with current scope | Control endpoints exist and keep existing error semantics; production SQLite hydration is deferred. |
| Admin audit trail | Implemented | Structured audit logs and in-memory audit history avoid bearer-token and secret exposure. |
| Blocklist management | Implemented with current scope | `key_type` is validated as `ip`, `address`, or `fingerprint`; persisted abuse-enforcement integration remains deferred. |
| Dynamic budget/config editing | Deferred | Explicitly out of Phase 18 closure. |
| Allowlist/campaign management | Deferred | Explicitly out of Phase 18 closure. |
| CSV export and durable admin audit persistence | Deferred | Explicitly out of Phase 18 closure. |

---

## 10. Observabilidad

### Implemented observability baseline

The current production faucet has already implemented the first operational observability layer: structured JSON access logs, request and correlation IDs, safe claim-flow logs, admin-protected process-local runtime counters at `/api/v1/admin/metrics`, and enriched `/health` and `/ready` responses. The remaining backlog in this section still tracks future external monitoring, Prometheus-style scraping, alert routing, and longer-term operator surfaces.

| Feature | Descripción |
|---|---|
| **Logs JSON** | Producción operable. |
| **Request ID** | Trazabilidad por request. |
| **Correlation ID** | Desde nginx hasta backend. |
| **Audit logs** | Separados de logs técnicos. |
| **Métricas Prometheus** | Claims, errores, latencias, cola, saldo. |
| **Métricas de abuso** | Captcha fail, blocklist hits, risk score. |
| **Métricas blockchain** | Tx enviadas, fallidas, confirmadas, gas. |
| **Alertas** | Bajo saldo, RPC caído, cola trabada, drenaje anormal. |
| **Structured error logs** | Código, causa, request ID. |
| **Slow request logging** | Requests lentas. |
| **Nginx access logs correlacionables** | Con request ID. |
| **Journald integration** | Logs desde systemd. |
| **Log rotation** | Retención controlada. |
| **Uptime monitoring** | Externo e interno. |

---

## 11. Frontend público

| Feature | Descripción |
|---|---|
| **Landing faucet** | Explica red, monto y cooldown. |
| **Formulario de claim** | Address + captcha. |
| **Validación inmediata** | Address inválida, cooldown, faucet pausado. |
| **Estado de claim** | Seguimiento visual. |
| **Link a explorer** | Tx pública. |
| **Historial por address** | Últimos reclamos. |
| **Indicador de saldo/estado** | Sin exponer información sensible de más. |
| **Mensajes anti-abuso claros** | Cooldown, límite alcanzado, captcha inválido. |
| **Responsive mobile** | Uso desde celular. |
| **Tema SCAVIUM** | Identidad visual. |
| **Modo mantenimiento visible** | Comunicación clara. |
| **Privacidad y términos** | Links públicos. |
| **Accesibilidad básica** | Labels, contraste, navegación teclado. |

---

## 12. Nginx, Linux y systemd

| Feature | Descripción |
|---|---|
| **Nginx TLS** | Certbot/ACME. |
| **HSTS** | HTTPS estricto. |
| **HTTP to HTTPS redirect** | Redirección automática. |
| **Rate limit nginx** | Primera capa anti-abuso. |
| **Connection limit nginx** | Control por IP. |
| **Proxy buffering ajustado** | Según API. |
| **Body size limit** | Payload pequeño. |
| **Timeouts nginx** | Seguridad ante slow clients. |
| **Firewall** | Solo 80/443 públicos. |
| **Backend solo localhost** | Puerto Go no expuesto. |
| **Usuario Linux dedicado** | `scavium-faucet`. |
| **Directorios estándar** | `/opt/scavium-faucet`, `/etc/scavium-faucet`, `/var/lib/scavium-faucet`. |
| **systemd hardening** | `NoNewPrivileges`, `ProtectSystem`, `PrivateTmp`, `ReadWritePaths`. |
| **Restart policy** | `Restart=always`. |
| **Watchdog** | Detección de cuelgues. |
| **EnvironmentFile** | Config segura. |
| **Deploy script** | Instala binario, config, systemd, nginx. |
| **Rollback** | Volver a versión anterior. |

---

## 13. Gestión de secretos

| Feature | Descripción |
|---|---|
| **Private key fuera del repo** | Nunca embebida. |
| **Keystore cifrado** | Mejor que private key plana. |
| **Passphrase separada** | No junto al keystore. |
| **Permisos estrictos** | Solo usuario del servicio. |
| **Rotación de wallet** | Procedimiento definido. |
| **Treasury separada** | Faucet con fondos limitados. |
| **Refill manual seguro** | Recarga controlada. |
| **Refill automático opcional** | Solo con límites duros. |
| **Secret scanning en CI** | Evitar filtraciones. |
| **No loguear secretos** | Sanitización de logs. |

---

## 14. Testing y QA

| Feature | Descripción |
|---|---|
| **Unit tests** | Validadores, límites, scoring, nonce. |
| **Integration tests** | API + DB + worker. |
| **RPC mock** | Simulación blockchain. |
| **Captcha mock** | Tests sin depender de proveedor. |
| **Rate limit tests** | IP/address/fingerprint. |
| **Idempotency tests** | Sin doble envío. |
| **Nonce concurrency tests** | Claims simultáneos. |
| **Reconciliation tests** | Tx pendientes. |
| **Admin auth tests** | Roles, 2FA, permisos. |
| **Migration tests** | DB limpia y upgrades. |
| **Load tests** | Picos públicos. |
| **Abuse simulation** | Bots, IP rotation, address rotation. |
| **Security smoke tests** | CORS, headers, auth, payloads. |

---

## 15. CI/CD

| Feature | Descripción |
|---|---|
| **Go test** | `go test ./...`. |
| **Lint** | `golangci-lint`. |
| **Build Linux** | `amd64`/`arm64`. |
| **Version embedding** | Commit, tag, build date. |
| **Artifact release** | Binario + checksums. |
| **Docker opcional** | No obligatorio si va con systemd. |
| **SBOM** | Inventario de dependencias. |
| **Dependency scanning** | Vulnerabilidades. |
| **Secret scanning** | Protección de claves. |
| **Deploy manual controlado** | Script reproducible. |
| **Rollback documentado** | Cambio rápido de versión. |

---

## 16. Documentación operativa

| Feature | Descripción |
|---|---|
| **README** | Qué es, cómo correr, cómo configurar. |
| **Install guide Debian/Ubuntu** | Usuario, carpetas, binario, permisos. |
| **systemd unit** | Archivo completo documentado. |
| **nginx config** | Server block completo. |
| **Firewall guide** | UFW/nftables. |
| **Config reference** | Todas las variables. |
| **Runbook operativo** | Reinicio, logs, health, backups. |
| **Runbook de incidentes** | Sin fondos, RPC caído, nonce trabado, abuso. |
| **Security checklist** | Antes de publicar. |
| **Production checklist** | Checklist final. |
| **Backup/restore guide** | DB y config. |
| **Wallet rotation guide** | Cambio seguro de faucet wallet. |
| **Admin guide** | Uso del panel/API admin. |
| **API docs** | OpenAPI + ejemplos. |

---

## 17. Features de nivel profesional alto

| Feature | Descripción |
|---|---|
| **Campañas públicas** | Faucet por evento, partner o testnet. |
| **Códigos de invitación** | Claims con cupo especial. |
| **Cuotas por campaña** | Presupuesto separado. |
| **Multi-network** | Varias redes SCAVIUM. |
| **Multi-asset** | SCAVIUM nativo + tokens. |
| **Multi-wallet** | Varias wallets faucet. |
| **Wallet rotation automática** | Balanceo y seguridad. |
| **Alta disponibilidad** | Varias instancias. |
| **Distributed lock** | Nonce seguro en múltiples instancias. |
| **Redis opcional** | Rate limits, locks y cache. |
| **PostgreSQL advisory locks** | Alternativa robusta para locks. |
| **Webhook de confirmación** | Integraciones externas. |
| **Notificaciones admin** | Telegram/Slack/email. |
| **Reportes de uso** | Diario/semanal/mensual. |
| **Análisis de abuso** | Panel de patrones sospechosos. |
| **Tamper-evident audit log** | Auditoría encadenada por hash. |
| **Snapshot de decisión** | Guardar reglas aplicadas a cada claim. |
| **Privacy mode** | Minimizar datos personales. |
| **Data retention automática** | Limpieza programada. |
| **Legal pages** | Términos, privacidad, abuso, contacto. |
| **Status page** | Página pública de disponibilidad. |
| **Admin API interna separada** | Subdominio privado o VPN. |

---

## 18. Integración con SCAVIUM Wallet

### 18.1 Integración nativa con wallet

| Feature | Descripción |
|---|---|
| **Claim desde SCAVIUM Wallet** | La wallet puede solicitar fondos directamente sin pasar por el frontend web. |
| **Uso de API pública desde app** | La wallet consume los endpoints `/api/v1` del faucet vía HTTPS. |
| **Detección automática de faucet** | La wallet detecta si la red actual tiene faucet disponible. |
| **Experiencia embebida** | El usuario solicita fondos desde la propia UI de la wallet. |

### 18.2 Endpoints orientados a wallet

| Endpoint | Descripción |
|---|---|
| `GET /api/v1/faucet/status` | Estado del faucet optimizado para apps. |
| `GET /api/v1/faucet/config` | Parámetros visibles para la wallet. |
| `GET /api/v1/faucet/address/{address}/eligibility` | Indica si puede reclamar, cooldown restante y límites. |
| `POST /api/v1/faucet/claim` | Solicitud directa desde la wallet. |
| `GET /api/v1/faucet/claim/{id}` | Seguimiento del claim desde la wallet. |

### 18.3 Seguridad adaptada a wallet

| Feature | Descripción |
|---|---|
| **Firma de wallet opcional** | Firma de mensaje para probar control de la address. |
| **Challenge alternativo a captcha** | Para mobile/webview donde captcha tradicional es problemático. |
| **Device binding opcional** | Asociación entre address y dispositivo. |
| **App origin policy** | Permitir requests desde la app oficial sin abrir CORS global. |
| **Rate limit específico para wallet** | Diferente política para tráfico legítimo de la app. |

### 18.4 Flujo desde SCAVIUM Wallet

1. Wallet detecta red (testnet/devnet)
2. Consulta `faucet/status`
3. Consulta `faucet/config`
4. Consulta elegibilidad de address
5. Usuario solicita fondos
6. Wallet envía request (address + challenge + firma opcional)
7. Faucet valida y encola
8. Wallet recibe tracking ID
9. Wallet consulta estado
10. Faucet devuelve `txHash` cuando está disponible

### 18.5 UX dentro de la wallet

| Feature | Descripción |
|---|---|
| **Botón "Solicitar fondos"** | Disponible solo cuando aplica. |
| **Indicador de cooldown** | Tiempo restante visible. |
| **Estado del claim** | `pending`, `sent`, `confirmed`. |
| **Link a explorer** | Ver tx directamente. |
| **Mensajes claros** | Cooldown, límite, error, faucet pausado. |
| **Fallback a web** | Si el claim falla, opción de abrir faucet web. |

### 18.6 Beneficios de integración

| Feature | Descripción |
|---|---|
| **Mejor UX** | Usuario no abandona la wallet. |
| **Menos fricción** | No requiere copiar/pegar address. |
| **Menos errores** | Address siempre válida. |
| **Mayor control anti-abuso** | Se pueden usar señales del dispositivo. |
| **Mayor adopción** | Onboarding directo desde la wallet. |

### 18.7 Consideraciones importantes

| Feature | Descripción |
|---|---|
| **No reemplaza frontend web** | Ambos conviven (web + wallet). |
| **No usa RPC directo** | Siempre pasa por API del faucet. |
| **Debe respetar rate limits** | Igual que cualquier cliente. |
| **Debe soportar captcha/challenge** | Alternativa válida para mobile. |
| **Debe ser backward compatible** | No romper API pública existente. |

---

## Orden recomendado para construirlo

### Fase 1 — MVP público seguro

- Backend Go
- Config externa
- systemd
- nginx HTTPS
- DB
- Endpoint `status`/`config`
- Claim público
- Captcha
- Rate limit IP/address
- Cooldown
- Wallet signer
- Nonce manager
- Cola persistente
- Health/readiness
- Logs JSON

### Fase 2 — Producción pública

- Admin privado
- Auditoría
- Confirmaciones on-chain
- Reconciliación
- Métricas
- Alertas
- Circuit breaker
- Presupuesto diario/global
- Blocklist/allowlist
- Backup/restore

### Fase 3 — Anti-abuso fuerte

- Fingerprint
- Risk scoring
- Greylist
- VPN/proxy detection
- Cooldown adaptativo
- Monto adaptativo
- Manual review
- Burst detection
- Address clustering

### Fase 4 — Profesional completo

- Multi-network
- Multi-asset
- Multi-wallet
- Campaigns
- Invitations
- Reports
- Webhooks
- HA/distributed lock
- Treasury/refill seguro
- Auditoría reforzada
- Runbooks completos
