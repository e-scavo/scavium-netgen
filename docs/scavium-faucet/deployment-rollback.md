# Deployment rollback guide

This rollback assumes the release layout described in [deployment.md](deployment.md): immutable release directories plus a `current` symlink.

## Preconditions

- keep at least one previously known-good release in `APP_PATH/releases/`
- keep the environment file outside the release directory
- do not delete the previous nginx or systemd files until the new release is confirmed

## Fast rollback

1. identify the last known-good `RELEASE_ID`
2. repoint `APP_PATH/current` to that release
3. restart `SERVICE_NAME`
4. verify local health before changing anything else

Example:

```bash
sudo ln -sfn APP_PATH/releases/PREVIOUS_RELEASE_ID APP_PATH/current
sudo systemctl restart SERVICE_NAME.service
curl -fsS http://127.0.0.1:18080/health
```

## If the problem is config, not code

- restore the previous environment file
- restore the previous reviewed systemd unit if it changed
- restore the previous nginx site if it changed
- validate nginx before reload

Example:

```bash
sudo nginx -t
sudo systemctl reload nginx
```

## When not to roll back only the binary

Do a wider rollback if the new release also changed:

- environment variables
- nginx routing or TLS configuration
- systemd sandboxing or path layout

Those surfaces should roll back together to avoid mixed-state failures.

## Post-rollback review

- inspect `journalctl -u SERVICE_NAME.service --no-pager -n 200`
- confirm `/health` and `/ready`
- record the failing `RELEASE_ID` and keep it for analysis instead of deleting it immediately
