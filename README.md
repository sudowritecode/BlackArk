# BlackArk

BlackArk is a lightweight multi-node Docker orchestrator. Workers poll the control plane and access only their local Docker Unix socket; the Docker API is never exposed over the network.

To deploy and operate a simple app on an existing cluster, see the [user guide](docs/USER_GUIDE.md).
For production installation, upgrades, rollback, and release verification, see the [operations runbook](docs/OPERATIONS.md).

## Local setup

Requirements: Go 1.23+, Docker with Compose.

```sh
cp .env.example .env
# Set BLACKARK_API_TOKEN in .env to a local secret.
set -a; . ./.env; set +a
docker compose up --build -d
go run ./cmd/blackark health
go run ./cmd/blackark status
```

The control process applies embedded, transactional migrations at startup. To run them without serving HTTP:

```sh
go run ./cmd/blackark-control migrate
```

Build and test all components:

```sh
make build
make test
```

Endpoints:

- `GET /healthz` — unauthenticated liveness and database readiness.
- `GET /api/v1/status` — requires `Authorization: Bearer $BLACKARK_API_TOKEN`.
- `POST /v1/join-tokens` — create a single-use, expiring worker join token.
- `POST /v1/nodes/join` and `POST /v1/nodes/{id}/heartbeat` — worker enrollment and reconciliation.
- `/v1/apps` — create, list, inspect, scale, deploy, retrieve logs, and delete applications.

Create a worker credential and start an agent:

```sh
JOIN_TOKEN=$(curl -sS -X POST -H "Authorization: Bearer $BLACKARK_API_TOKEN" \
  -H 'Content-Type: application/json' -d '{"ttl_seconds":600}' \
  http://localhost:8080/v1/join-tokens | jq -r .token)
BLACKARK_JOIN_TOKEN=$JOIN_TOKEN BLACKARK_CONTROL_URL=http://localhost:8080 \
  go run ./cmd/blackark-agent
```

Persist the node ID and credential printed on first join in a secret store before restarting the agent.

Do not commit `.env` or production credentials. The Compose defaults are development-only.
