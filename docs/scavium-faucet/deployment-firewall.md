# Deployment firewall guide

The public surface should be **nginx on 80/443 only**. Keep the Go process and RPC ports off the public interface.

## Target exposure

| Service | Public internet | Notes |
|---|---|---|
| SSH `22/tcp` | restricted | prefer office IPs, VPN, or bastion |
| HTTP `80/tcp` | allowed | needed for ACME HTTP-01 and redirect |
| HTTPS `443/tcp` | allowed | primary public entrypoint |
| Faucet backend `18080/tcp` | denied | loopback-only service |
| RPC HTTP `18545/tcp` | denied | never expose publicly unless separately protected |
| RPC WS `18546/tcp` | denied | same rule as RPC HTTP |
| Metrics `19545/tcp` | denied | expose only via trusted network if needed |

## UFW example

Review and adapt before use:

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing

sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

sudo ufw deny 18080/tcp
sudo ufw deny 18545/tcp
sudo ufw deny 18546/tcp
sudo ufw deny 19545/tcp

sudo ufw enable
sudo ufw status verbose
```

If SSH should be restricted further, replace `allow OpenSSH` with a source-specific rule.

## Cloud firewall or provider security group

Mirror the same policy at the VPS provider layer:

1. allow `22/tcp` only from trusted sources
2. allow `80/tcp` from anywhere
3. allow `443/tcp` from anywhere
4. deny or omit `18080`, `18545`, `18546`, and `19545`

## Local binding still matters

Even with a firewall, keep:

```ini
SCAVIUM_FAUCET_BIND_ADDR=127.0.0.1:18080
SCAVIUM_FAUCET_RPC_URL=http://127.0.0.1:18545
```

The firewall is a second control, not the primary one.

## Verification checklist

- nginx answers on `https://DOMAIN`
- backend does not answer on `http://DEPLOY_HOST:18080`
- RPC does not answer on `http://DEPLOY_HOST:18545`
- certbot challenge path is reachable on port 80 before the HTTPS redirect is enforced
