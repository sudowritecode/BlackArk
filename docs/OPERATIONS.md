# Install, upgrade, and rollback

This runbook installs BlackArk release binaries on one Linux control host and two independent Linux worker hosts. Commands use `v1.0.3` and `linux-amd64`; change both values together for another published release or architecture.

## Prerequisites

- Linux hosts with `systemd`; Docker Engine on each worker; Docker Compose on the control host.
- `curl`, `jq`, and `tar` on the operator machine.
- A DNS record for the control-plane hostname pointing to the control host.
- A strong API token kept in a secret manager, not in shell history or source control.

## Install

Download the three archives from the GitHub release, verify their published checksums or provenance, and extract the appropriate binary on each host:

```sh
VERSION=v1.0.3
ARCH=linux-amd64
curl -fLO "https://github.com/sudowritecode/BlackArk/releases/download/$VERSION/blackark-control-$VERSION-$ARCH.tar.gz"
curl -fLO "https://github.com/sudowritecode/BlackArk/releases/download/$VERSION/blackark-agent-$VERSION-$ARCH.tar.gz"
curl -fLO "https://github.com/sudowritecode/BlackArk/releases/download/$VERSION/blackark-$VERSION-$ARCH.tar.gz"
tar -xzf "blackark-control-$VERSION-$ARCH.tar.gz"
tar -xzf "blackark-agent-$VERSION-$ARCH.tar.gz"
tar -xzf "blackark-$VERSION-$ARCH.tar.gz"
sudo install -m 0755 blackark-control blackark-agent blackark /usr/local/bin/
```

On the control host, store `BLACKARK_API_TOKEN`, `BLACKARK_DB_PATH`, and the listen address in a root-readable environment file. Run `blackark-control migrate`, then run `blackark-control serve` under a dedicated `systemd` service account. Configure Caddy from `deploy/Caddyfile` and confirm `https://<control-domain>/healthz` returns `{"status":"ok"}`.

Create one single-use join token per worker:

```sh
curl -fsS -X POST -H "Authorization: Bearer $BLACKARK_API_TOKEN" \
  -H 'Content-Type: application/json' -d '{"ttl_seconds":600}' \
  "https://$BLACKARK_DOMAIN/v1/join-tokens"
```

On each worker, run `blackark-agent` under `systemd` with a unique `BLACKARK_NODE_NAME`, `BLACKARK_CONTROL_URL=https://<control-domain>`, and its own join token. After enrollment, replace the join token with the node ID and credential printed by the agent and store those values in the host secret store. Confirm that `GET /v1/nodes` reports both independent hosts as healthy.

From a clean operator machine, run the release gate:

```sh
BLACKARK_URL="https://$BLACKARK_DOMAIN" \
BLACKARK_DOMAIN="$BLACKARK_DOMAIN" \
BLACKARK_TOKEN="$BLACKARK_API_TOKEN" \
BLACKARK_IMAGE=nginx:1.27-alpine \
./scripts/verify-mvp.sh
```

Preserve the release version, commit or tag, host topology, exact command, script output, node list before and after, worker logs, and confirmation that no app containers remain.

## Upgrade

1. Back up the control database and environment/service files. Record the currently installed binary checksums.
2. Download and verify the target release before changing a host.
3. Stop the control service, run the target `blackark-control migrate`, atomically replace the control and CLI binaries, and restart the service.
4. Confirm `/healthz` and `/api/v1/status`, then replace and restart one worker agent at a time. Wait for it to become healthy before continuing.
5. Run `scripts/verify-mvp.sh` from the operator machine and retain the evidence.

## Rollback

1. Stop rollout and preserve control/worker logs. Do not restore a database while the control service is running.
2. If the release only changed binaries and its migrations are backward compatible, atomically restore the previous binaries and restart workers one at a time, then the control service.
3. If a migration is not backward compatible, stop the control service, restore the pre-upgrade database backup and previous control binary together, then restart it. Restore workers to the matching release.
4. Confirm both nodes are healthy and rerun `scripts/verify-mvp.sh`. If verification fails, keep the release blocked and attach the logs and exact failed step to a follow-up issue.

Never roll back only the database or only the control binary across an incompatible schema boundary.
