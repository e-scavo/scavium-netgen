# Deployment

This package prepares a **manual, review-first** VPS deployment for `scavium-faucet`.

It does **not** assume automatic production execution from this repository. All files use placeholders such as `DOMAIN`, `DEPLOY_USER`, `APP_PATH`, and `SERVICE_NAME` so an operator can review and adapt them before use.

## Current runtime constraints

Keep the deployment aligned with the current binary, not the future roadmap:

- claim state is in memory only
- restarting the process clears in-memory claims
- admin routes are still disabled in the shipped binary
- readiness checks are shallow

Because of that, treat this as a careful MVP deployment with strong outer controls.

## Suggested server layout

```text
APP_PATH/
  current -> APP_PATH/releases/RELEASE_ID
  releases/
    RELEASE_ID/
      scavium-faucet
  review/
    RELEASE_ID/
      scavium-faucet.env.example
      scavium-faucet.service.template
      scavium-faucet.nginx.conf.template
```

Recommended placeholder mapping:

| Placeholder | Meaning |
|---|---|
| `DOMAIN` | public hostname, for example `faucet.example.com` |
| `DEPLOY_HOST` | target VPS hostname or IP |
| `DEPLOY_USER` | non-root SSH user used for deploys |
| `DEPLOY_GROUP` | service group on the VPS |
| `APP_PATH` | release root, for example `/opt/scavium-faucet` |
| `SERVICE_NAME` | systemd unit name, for example `scavium-faucet` |

## Files in this package

| File | Purpose |
|---|---|
| `deployment/scavium-faucet.service.template` | systemd unit template |
| `deployment/scavium-faucet.nginx.conf.template` | nginx server block template |
| `deployment/scavium-faucet.env.example` | example environment file with placeholders only |
| `deployment-certbot.md` | manual ACME and certbot guide |
| `deployment-firewall.md` | VPS and edge firewall guide |
| `deployment-rollback.md` | rollback procedure |
| `../../scripts/deploy-scavium-faucet-safe.sh` | safe deploy helper; review mode by default |

## Review-first deployment flow

1. Build the binary outside the server and decide a `RELEASE_ID`.
2. Review and fill the environment example with real values **outside the repository**.
3. Review and render the systemd and nginx templates with your final paths and domain.
4. Use `scripts/deploy-scavium-faucet-safe.sh --plan` to inspect the exact staging commands.
5. If the plan looks correct, run the same script with `--execute`.
6. Install the reviewed systemd and nginx files manually on the VPS.
7. Follow the certbot guide only after DNS points to the VPS and nginx is syntactically valid.
8. Keep the Go service bound to loopback and expose only nginx on the public interface.

## Manual systemd installation

Template source:

```text
docs/scavium-faucet/deployment/scavium-faucet.service.template
```

Manual operator steps after review:

```bash
sudo install -o root -g root -m 0644 \
  ./scavium-faucet.service \
  /etc/systemd/system/SERVICE_NAME.service

sudo systemctl daemon-reload
sudo systemctl enable SERVICE_NAME.service
sudo systemctl restart SERVICE_NAME.service
sudo systemctl status SERVICE_NAME.service --no-pager
```

## Manual nginx installation

Template source:

```text
docs/scavium-faucet/deployment/scavium-faucet.nginx.conf.template
```

Manual operator steps after review:

```bash
sudo install -o root -g root -m 0644 \
  ./scavium-faucet.nginx.conf \
  /etc/nginx/sites-available/SERVICE_NAME.conf

sudo ln -sfn \
  /etc/nginx/sites-available/SERVICE_NAME.conf \
  /etc/nginx/sites-enabled/SERVICE_NAME.conf

sudo nginx -t
sudo systemctl reload nginx
```

Do not enable HSTS until HTTPS is working correctly and you are sure the hostname will stay permanent.

## Environment handling

Keep the real environment file outside the repository, for example:

```text
/etc/scavium-faucet/scavium-faucet.env
```

Start from:

```text
docs/scavium-faucet/deployment/scavium-faucet.env.example
```

The checked-in example intentionally keeps secret values as placeholders.

## Related guides

- [Certbot / ACME](deployment-certbot.md)
- [Firewall](deployment-firewall.md)
- [Rollback](deployment-rollback.md)
