# Install, upgrade, and rollback

This runbook installs BlackArk on one Linux control host and two independent Linux worker hosts. Commands use `v1.0.3`; change the version for another published release.

## Prerequisites

- Linux hosts with `systemd`; Docker Engine on each worker; Docker Compose on the control host.
- `curl`, `jq`, and `tar` on the operator machine.
- A DNS record for the control-plane hostname pointing to the control host.
- A strong API token kept in a secret manager, not in shell history or source control.

## Install

The release publishes one bundle containing `blackark`, `blackark-control`, `blackark-agent`, and the installer. The installer downloads that bundle, installs the binaries, and can write systemd units for the control plane or worker agent.

Install only the operator CLI on a workstation:

```sh
VERSION=v1.0.3
curl -fsSL https://raw.githubusercontent.com/sudowritecode/BlackArk/main/scripts/install.sh | \
  VERSION=$VERSION ROLE=cli sh
```

On the control host, install the CLI plus control service. The script writes `/etc/blackark/control.env`, creates a dedicated `blackark` system user, installs `blackark-control.service`, starts it, and enables it at boot:

```sh
VERSION=v1.0.3
curl -fsSL https://raw.githubusercontent.com/sudowritecode/BlackArk/main/scripts/install.sh | \
  VERSION=$VERSION \
  ROLE=control \
  BLACKARK_API_TOKEN='<strong-token>' \
  BLACKARK_DATABASE_URL='postgres://blackark:password@127.0.0.1:5432/blackark?sslmode=disable' \
  BLACKARK_LISTEN_ADDR=':8080' \
  sh
```

Configure Caddy from `deploy/Caddyfile` and confirm `https://<control-domain>/healthz` returns `{"status":"ok"}`.

Create one single-use join token per worker:

```sh
curl -fsS -X POST -H "Authorization: Bearer $BLACKARK_API_TOKEN" \
  -H 'Content-Type: application/json' -d '{"ttl_seconds":600}' \
  "https://$BLACKARK_DOMAIN/v1/join-tokens"
```

On each worker, install and start the agent with a unique `BLACKARK_NODE_NAME`, `BLACKARK_CONTROL_URL=https://<control-domain>`, and either its own join token or the admin API token. The script writes `/etc/blackark/agent.env`, installs `blackark-agent.service`, starts it, and enables it at boot. On first start the agent exchanges the bootstrap secret for durable node credentials, rewrites `/etc/blackark/agent.env` with `BLACKARK_NODE_ID` and `BLACKARK_NODE_TOKEN`, and removes `BLACKARK_JOIN_TOKEN`/`BLACKARK_API_TOKEN` from that file:

```sh
VERSION=v1.0.3
curl -fsSL https://raw.githubusercontent.com/sudowritecode/BlackArk/main/scripts/install.sh | \
  VERSION=$VERSION \
  ROLE=agent \
  BLACKARK_CONTROL_URL="https://$BLACKARK_DOMAIN" \
  BLACKARK_JOIN_TOKEN='<single-use-join-token>' \
  BLACKARK_NODE_NAME="$(hostname)" \
  sh
```

For single-node installs, `ROLE=single-node` can use the control-plane `BLACKARK_API_TOKEN` and defaults the local agent to `BLACKARK_CONTROL_URL=http://127.0.0.1:8080` when no control URL is supplied. After enrollment, store the generated `/etc/blackark/agent.env` values in the host secret store and confirm that `GET /v1/nodes` reports both independent hosts as healthy.

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
