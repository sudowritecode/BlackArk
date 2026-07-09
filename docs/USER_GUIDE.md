# BlackArk user guide

This guide deploys a simple nginx container to an existing BlackArk cluster using the `blackark` CLI. No `curl` commands are needed for the normal operator workflow.

## What you need

- A running BlackArk control plane and at least one healthy worker.
- The control-plane URL, for example `https://blackark.example.com`.
- The API token configured as `BLACKARK_API_TOKEN` on the control plane.
- The `blackark` binary (build with `make build` or download a release).
- A container image that the workers can pull. Public images work without registry credentials.

## Quick start

Log in once to save your credentials:

```sh
blackark login --url https://blackark.example.com --token "$BLACKARK_API_TOKEN"
```

Credentials are saved to `~/.config/blackark/config.yaml` (or `$BLACKARK_CONFIG`). Subsequent commands read the config file automatically.

## 1. Check the cluster

```sh
blackark health
blackark status
blackark get nodes
```

Do not deploy until at least one node reports `"status": "healthy"`.

## 2. Deploy nginx with a manifest

Create a file called `nginx.yaml`:

```yaml
apiVersion: blackark/v1
kind: App
metadata:
  name: my-nginx
spec:
  image: nginx:1.27-alpine
  replicas: 2
```

Apply it:

```sh
blackark apply -f nginx.yaml
```

Workers poll every five seconds. Wait briefly, then inspect the app:

```sh
blackark describe app my-nginx
```

The deployment is ready when all instances report `"status": "running"`. With two or more healthy workers, BlackArk spreads replicas across nodes when capacity permits.

The CLI accepts a local file path with `-f`.

## 3. Operate the app

List all apps:

```sh
blackark get apps
```

Read the most recent combined container logs:

```sh
blackark logs my-nginx
blackark logs --tail 100 my-nginx
```

Scale to three replicas:

```sh
blackark scale my-nginx 3
```

Scale to zero while keeping the app record:

```sh
blackark scale my-nginx 0
```

Restart the app's containers:

```sh
blackark restart my-nginx
```

Delete the app's deployed containers:

```sh
blackark delete app my-nginx
```

A successful delete prints `app deleted`. The app record remains for audit purposes with zero desired replicas.

## 4. Dashboard

### CLI dashboard

Display a one-time cluster summary with the credentials saved by `blackark login`:

```sh
blackark dashboard [--watch] [--interval N]

# One-time snapshot
blackark dashboard
```

Use watch mode to clear and redraw the terminal automatically. The default refresh interval is five seconds; `--interval` accepts a number of seconds and values below one are treated as one second.

```sh
# Refresh every five seconds
blackark dashboard --watch

# Refresh every ten seconds
blackark dashboard --watch --interval 10
```

The shorthand flags are `-w` and `-i`. `--interval` has no effect unless watch mode is enabled. Stop watch mode with `Ctrl-C`.

Example output:

```text
BlackArk Cluster   blackark.example.com
Version            1.0.3   Uptime   86400s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Nodes:             2 healthy   ·   0 unhealthy   ·   0 pending
Apps:              1 running   ·   0 stopped     ·   0 failed

NODE       STATUS    CPUS   MEM             APPS
worker-1   healthy   1/4    1.2GB/8.0GB     1
worker-2   healthy   0/4    900.0MB/8.0GB   0

APP        IMAGE                 REPLICAS   STATUS
my-nginx   nginx:1.27-alpine     2/2        running
```

### Web dashboard

When the dashboard bundled with the control plane is enabled, open:

```text
https://blackark.example.com/dashboard/
```

Set `BLACKARK_DASHBOARD_ENABLED=true` on `blackark-control` to register the dashboard UI and API routes. The Compose configuration enables it by default. Treat the dashboard as an administrative endpoint and expose it only over HTTPS on trusted networks.

The standalone Vue dashboard is available at `http://localhost:5173` during development. It expects the dashboard backend at `http://localhost:3001`; configure that backend with the control-plane URL and API token:

```sh
CONTROL_URL=https://blackark.example.com \
CONTROL_API_TOKEN="$BLACKARK_API_TOKEN" \
CORS_ORIGINS=http://localhost:5173 \
  bun run dev
```

`CONTROL_API_TOKEN` is the same value configured as `BLACKARK_API_TOKEN` on the control plane. It stays on the backend and is added to upstream requests as a bearer token. For deployments where the browser authenticates directly, enter the token through the dashboard's **API authentication** settings; it is stored in browser `localStorage` under `blackark.apiToken`. A build-time `VITE_API_TOKEN` can supply the browser token instead, but do not embed production secrets in a public frontend bundle.

The standalone dashboard uses an SSE connection to `/api/v1/events`. The backend sends a fresh dashboard snapshot every five seconds, so node and app cards can update without a page reload. If the stream disconnects, the SSE client reconnects automatically with exponential backoff, starting at one second and capped at 30 seconds.

## 5. YAML App manifest reference

The `blackark/v1` App manifest supports the following fields:

```yaml
apiVersion: blackark/v1      # required, must be "blackark/v1"
kind: App                     # required, must be "App"
metadata:
  name: my-app               # required, unique among apps
spec:
  image: nginx:1.27-alpine   # required, any image workers can pull
  replicas: 2                 # optional, default 1, non-negative integer
```

Unknown fields cause the manifest to be rejected with an actionable error. Use `KnownFields` validation — manifests with extra keys fail at apply time.

## Important MVP limitations

BlackArk v1.0.3 only accepts `name`, `image`, and `replicas` for an app. It does not yet accept container ports, environment variables, commands, volumes, health checks, resource limits, secrets, or registry credentials.

Consequently, the nginx example proves that a web-server container can be scheduled and managed, but it does **not** publish nginx to the internet. The included Caddy configuration terminates HTTPS for the BlackArk control API. Publishing each deployed web app needs a future routing/port-mapping feature or manual worker-side networking outside BlackArk.

Workers access the local Docker socket. Treat worker hosts and the BlackArk API token as privileged infrastructure, use HTTPS outside local development, and do not expose the Docker API over TCP.

## Add a worker (administrator)

Create a single-use join token on the control plane (requires the API token):

```sh
# The API token is still needed for admin operations
# Create a join token with timeout-limited validity:
curl -s -X POST -H "Authorization: Bearer $BLACKARK_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"ttl_seconds":600}' \
  "$BLACKARK_CONTROL_URL/v1/join-tokens"
```

On the worker host, start an agent that can reach the control URL and local Docker socket:

```sh
BLACKARK_CONTROL_URL="$BLACKARK_CONTROL_URL" \
BLACKARK_JOIN_TOKEN="$JOIN_TOKEN" \
BLACKARK_NODE_NAME=worker-1 \
  blackark-agent
```

On first join, the agent logs its node ID and credential. Store both in a secret manager and use `BLACKARK_NODE_ID` and `BLACKARK_NODE_TOKEN` on subsequent starts; join tokens are single-use and expire.

## Common failures

- `not logged in; run blackark login`: credentials are missing or the config file is not found. Run `blackark login --url <url> --token <token>`.
- `invalid config`: the config file at `~/.config/blackark/config.yaml` has syntax errors. Delete it and re-run login.
- `server returned 401 Unauthorized`: the API token does not match the control plane's `BLACKARK_API_TOKEN`. Re-login with the correct token.
- No healthy nodes: start a worker agent, check its control-plane connectivity, and inspect its logs.
- Image pull failure: verify the image name and that every eligible worker can pull it.
- App stays pending: ensure workers remain healthy and wait for the next five-second heartbeat.
- `invalid manifest: apiVersion must be blackark/v1`: check the apiVersion field in your YAML file.
- `invalid manifest: spec.image is required`: ensure the manifest has an image field under spec.
