# Contributing and releases

Create short-lived branches from `main` using `feat/<topic>`, `fix/<topic>`, or `chore/<topic>`. Open a pull request; do not merge until the Go, dependency, secret, and integration checks pass. Prefer squash merges so `main` stays bisectable.

Local checks:

```sh
gofmt -w .
go vet ./...
make test build integration
```

Tags matching `v*` create GitHub release archives and multi-platform images in GHCR for `blackark`, `blackark-control`, and `blackark-agent`. Create annotated semantic-version tags from a clean `main` branch:

```sh
git tag -a v0.1.0 -m 'BlackArk v0.1.0'
git push origin v0.1.0
```

Configure branch protection on `main` to require a pull request, one approval, and all four CI jobs. GitHub repository settings are intentionally not mutated by workflow code.

For edge HTTPS routing, copy `deploy/compose.edge.yml` and `deploy/Caddyfile` to the control host, set `ACME_EMAIL`, `BLACKARK_DOMAIN`, and optionally `BLACKARK_UPSTREAM`, then run `docker compose -f deploy/compose.edge.yml up -d`.

The final runtime smoke test is `scripts/verify-mvp.sh`. It requires `curl`, `jq`, a control-plane token, two healthy registered workers, and a DNS name pointed at Caddy. Its endpoint contract is explicit in the script and should be updated alongside runtime API changes.
