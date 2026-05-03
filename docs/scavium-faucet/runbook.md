# Runbook

Operational procedures for running `scavium-faucet` in production.

---

## Service management

```bash
# Status
systemctl status scavium-faucet

# Start / stop / restart
systemctl start  scavium-faucet
systemctl stop   scavium-faucet
systemctl restart scavium-faucet

# Follow live logs
journalctl -u scavium-faucet -f

# Last 200 lines
journalctl -u scavium-faucet -n 200 --no-pager
```

---

## Health checks

```bash
# Liveness
curl -s http://127.0.0.1:18080/health | jq .

# Readiness (checks DB, RPC, wallet, queue, balance)
curl -s http://127.0.0.1:18080/ready | jq .

# Faucet status (public)
curl -s http://127.0.0.1:18080/api/v1/faucet/status | jq .

# Version
curl -s http://127.0.0.1:18080/api/v1/version | jq .
```

---

## Changing operational mode

```bash
# Pause — stops accepting new claims, read-only endpoints remain up
curl -s -X POST http://127.0.0.1:18080/api/v1/admin/faucet/mode \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"mode":"paused"}'

# Maintenance — all endpoints return maintenance notice
curl -s -X POST http://127.0.0.1:18080/api/v1/admin/faucet/mode \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"mode":"maintenance"}'

# Restore normal operation
curl -s -X POST http://127.0.0.1:18080/api/v1/admin/faucet/mode \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"mode":"active"}'
```

---

## Blocking an IP or address

```bash
# Block an IP
curl -s -X POST http://127.0.0.1:18080/api/v1/admin/blocklist \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type":"ip","value":"1.2.3.4","reason":"bot traffic"}'

# Block an address
curl -s -X POST http://127.0.0.1:18080/api/v1/admin/blocklist \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type":"address","value":"0xAbCd…","reason":"abuse"}'
```

---

## Deploy / update binary

```bash
# 1. Build the new binary
go build -o /tmp/scavium-faucet ./cmd/scavium-faucet

# 2. Stop the service
systemctl stop scavium-faucet

# 3. Replace the binary
cp /opt/scavium-faucet/bin/scavium-faucet /opt/scavium-faucet/bin/scavium-faucet.bak
cp /tmp/scavium-faucet /opt/scavium-faucet/bin/scavium-faucet
chmod 750 /opt/scavium-faucet/bin/scavium-faucet

# 4. Start the service
systemctl start scavium-faucet

# 5. Verify health
curl -s http://127.0.0.1:18080/health
```

---

## Rollback

```bash
systemctl stop scavium-faucet
cp /opt/scavium-faucet/bin/scavium-faucet.bak /opt/scavium-faucet/bin/scavium-faucet
systemctl start scavium-faucet
```

---

## Backup and restore

### Backup (SQLite)

```bash
# Online backup (safe while the service is running)
sqlite3 /var/lib/scavium-faucet/faucet.db ".backup '/var/lib/scavium-faucet/faucet.db.bak'"
```

### Restore

```bash
systemctl stop scavium-faucet
cp /var/lib/scavium-faucet/faucet.db.bak /var/lib/scavium-faucet/faucet.db
systemctl start scavium-faucet
```

---

## Incident response

### Faucet wallet low on funds

1. Check balance: `curl -s http://127.0.0.1:18080/ready | jq .`
2. The faucet auto-pauses when balance is below the configured guard threshold.
3. Transfer funds to the faucet wallet from the treasury.
4. Resume: `POST /api/v1/admin/faucet/mode {"mode":"active"}`.

### RPC node unreachable

1. Verify Besu node: `systemctl status besu@rpc`.
2. Check `/ready` — it will report `rpc: timeout`.
3. The faucet will queue claims but not process them until RPC recovers.
4. If RPC is down for more than a few minutes, pause the faucet to avoid user confusion.

### Nonce stuck / transactions not confirming

1. Check pending tx count on the faucet wallet via RPC.
2. If a nonce is stuck, the worker will retry with backoff.
3. If the issue persists, pause the faucet, wait for mempool to clear, then resume.
4. As a last resort, restart the service — the worker reconciles pending claims on startup.

### Abuse / bot attack

1. Pause the faucet immediately: `POST /api/v1/admin/faucet/mode {"mode":"paused"}`.
2. Identify offending IPs/addresses from logs: `journalctl -u scavium-faucet | grep RATE_LIMIT`.
3. Block them via the admin blocklist API.
4. Consider tightening `SCAVIUM_FAUCET_RATE_LIMIT_IP_PER_HOUR` and restarting.
5. Resume when the attack subsides.

### Database corruption

1. Stop the service.
2. Restore from the most recent backup (see above).
3. If no backup is available, the service can restart with an empty DB — historical claims are lost but the service recovers.

---

## Directory layout

```
/opt/scavium-faucet/bin/scavium-faucet   # binary
/etc/scavium-faucet/env                  # EnvironmentFile (secrets, chmod 640)
/var/lib/scavium-faucet/faucet.db        # SQLite database
/var/log/scavium-faucet/                 # log files (if not using journald)
```
