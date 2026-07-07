# BlackArk

BlackArk is a lightweight multi-node Docker orchestrator. This repository currently contains the MVP foundation: the control-plane API, PostgreSQL schema, CLI, and agent entry point.

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

Do not commit `.env` or production credentials. The Compose defaults are development-only.
