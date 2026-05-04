# Deployment certbot / ACME guide

This guide is intentionally manual. Review every command before using it on a production VPS.

## Preconditions

- `DOMAIN` already resolves to the VPS
- nginx site file has been reviewed and installed
- port `80/tcp` is reachable for the ACME HTTP-01 challenge
- you are ready to terminate TLS in nginx

## Recommended order

1. Validate nginx syntax first.
2. Start with a staging or dry-run ACME flow.
3. Request the live certificate only after the dry run works.
4. Reload nginx after certbot updates the TLS files.

## Example package install

Package names vary by distribution. Review for your VPS OS first.

```bash
sudo apt update
sudo apt install nginx certbot python3-certbot-nginx
```

## Dry run first

```bash
sudo certbot --nginx \
  --domain DOMAIN \
  --agree-tos \
  --email OPS_EMAIL \
  --redirect \
  --dry-run
```

Notes:

- use a real operator mailbox for `OPS_EMAIL`
- keep `DOMAIN` aligned with the nginx `server_name`
- if you prefer not to let certbot edit nginx, use `certonly` and update the server block manually

## Live certificate request

```bash
sudo certbot --nginx \
  --domain DOMAIN \
  --agree-tos \
  --email OPS_EMAIL \
  --redirect
```

## Manual certonly alternative

If you want tighter control over the nginx file:

```bash
sudo certbot certonly \
  --webroot \
  --webroot-path /var/www/certbot \
  --domain DOMAIN \
  --agree-tos \
  --email OPS_EMAIL
```

Then point nginx to:

```text
/etc/letsencrypt/live/DOMAIN/fullchain.pem
/etc/letsencrypt/live/DOMAIN/privkey.pem
```

## Renewal

Check the renewal path manually:

```bash
sudo certbot renew --dry-run
```

Typical post-renew hook:

```bash
sudo systemctl reload nginx
```

## Production verification (May 2026)

The production host `faucet.testnet.scavium.network` has been verified with:

- successful `certbot renew --dry-run` execution (no timeout)
- active `certbot.timer`
- deploy hook present at `/etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh`
- nginx reload path confirmed via deploy hook

TLS auto-renewal is validated and operational.

## Common failure points

- DNS still points elsewhere
- port 80 blocked by VPS or cloud firewall
- nginx syntax error before the challenge starts
- another virtual host answers for `DOMAIN`
