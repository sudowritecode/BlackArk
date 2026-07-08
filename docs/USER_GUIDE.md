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

You can also apply the manifest directly from a URL or generate it inline — the CLI accepts a local file path with `-f`.

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

## 4. YAML App manifest reference

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
  "$BLACKARK_URL/v1/join-tokens"
```

On the worker host, start an agent that can reach the control URL and local Docker socket:

```sh
BLACKARK_CONTROL_URL="$BLACKARK_URL" \
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
