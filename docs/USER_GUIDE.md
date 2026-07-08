# BlackArk user guide

This guide deploys a simple nginx container to an existing BlackArk cluster. BlackArk v1.0.3 is an API-first MVP, so the examples use `curl` and `jq`.

## What you need

- A running BlackArk control plane and at least one healthy worker.
- The control-plane URL, for example `https://blackark.example.com`.
- The API token configured as `BLACKARK_API_TOKEN` on the control plane.
- `curl` and `jq` on your computer.
- A container image that the workers can pull. Public images work without registry credentials.

Set the client variables without putting the token in shell history:

```sh
export BLACKARK_URL=https://blackark.example.com
read -rsp 'BlackArk API token: ' BLACKARK_TOKEN; echo
export BLACKARK_TOKEN
```

Use this helper for the remaining examples:

```sh
blackark_api() {
  method=$1
  path=$2
  data=${3:-}
  curl --fail-with-body --silent --show-error \
    -X "$method" \
    -H "Authorization: Bearer $BLACKARK_TOKEN" \
    -H 'Content-Type: application/json' \
    ${data:+--data "$data"} \
    "$BLACKARK_URL$path"
}
```

## 1. Check the cluster

The public health endpoint checks the control plane and database:

```sh
curl --fail --silent "$BLACKARK_URL/healthz" | jq
```

Check that a worker is healthy:

```sh
blackark_api GET /v1/nodes | jq
```

Do not deploy until at least one node reports `"status": "healthy"`.

## 2. Create and deploy nginx

Create a two-replica app and save its ID:

```sh
APP_ID=$(
  blackark_api POST /v1/apps \
    '{"name":"my-nginx","image":"nginx:1.27-alpine","replicas":2}' \
  | jq -r .id
)
printf 'app id: %s\n' "$APP_ID"

blackark_api POST "/v1/apps/$APP_ID/deploy" '{}' | jq
```

Workers poll every five seconds. Wait briefly, then inspect the app:

```sh
blackark_api GET "/v1/apps/$APP_ID" | jq
```

The deployment is ready when both instances report `"status": "running"`. With two or more healthy workers, BlackArk spreads replicas across nodes when capacity permits.

## 3. Operate the app

List all apps:

```sh
blackark_api GET /v1/apps | jq
```

Read the most recent combined container logs:

```sh
blackark_api GET "/v1/apps/$APP_ID/logs?tail=100"
```

Scale to three replicas:

```sh
blackark_api PATCH "/v1/apps/$APP_ID" '{"replicas":3}' | jq
```

Scale to zero while keeping the app record:

```sh
blackark_api PATCH "/v1/apps/$APP_ID" '{"replicas":0}' | jq
```

Delete the app's deployed containers:

```sh
blackark_api DELETE "/v1/apps/$APP_ID"
unset APP_ID
```

A successful delete returns HTTP 204 with no response body. In v1.0.3, the app record remains for audit purposes with zero desired replicas.

## Important MVP limitations

BlackArk v1.0.3 only accepts `name`, `image`, and `replicas` for an app. It does not yet accept container ports, environment variables, commands, volumes, health checks, resource limits, secrets, or registry credentials.

Consequently, the nginx example proves that a web-server container can be scheduled and managed, but it does **not** publish nginx to the internet. The included Caddy configuration terminates HTTPS for the BlackArk control API. Publishing each deployed web app needs a future routing/port-mapping feature or manual worker-side networking outside BlackArk.

Workers access the local Docker socket. Treat worker hosts and the BlackArk API token as privileged infrastructure, use HTTPS outside local development, and do not expose the Docker API over TCP.

## Add a worker (administrator)

Create a single-use join token on the control plane:

```sh
JOIN_TOKEN=$(
  blackark_api POST /v1/join-tokens '{"ttl_seconds":600}' \
  | jq -r .token
)
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

- `401 unauthorized`: the client token does not match the control plane's `BLACKARK_API_TOKEN`.
- No healthy nodes: start a worker agent, check its control-plane connectivity, and inspect its logs.
- Image pull failure: verify the image name and that every eligible worker can pull it.
- App stays pending: ensure workers remain healthy and wait for the next five-second heartbeat.
- `409` when creating an app: app names are unique; choose another name.

