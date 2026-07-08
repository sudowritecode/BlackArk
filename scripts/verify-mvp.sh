#!/usr/bin/env bash
set -euo pipefail

: "${BLACKARK_URL:?set BLACKARK_URL}"
: "${BLACKARK_TOKEN:?set BLACKARK_TOKEN}"
: "${BLACKARK_IMAGE:=nginx:1.27-alpine}"
: "${BLACKARK_DOMAIN:?set BLACKARK_DOMAIN}"
: "${BLACKARK_SKIP_RESTART_CHECK:=}"
: "${BLACKARK_CURL_RESOLVE:=}"
: "${BLACKARK_CURL_INSECURE:=}"
: "${BLACKARK_APP_NAME:=mvp-smoke-$(date +%s)}"

api() {
  local method=$1 path=$2 data=${3:-}
  curl --fail-with-body --silent --show-error \
    -X "$method" -H "Authorization: Bearer $BLACKARK_TOKEN" \
    -H 'Content-Type: application/json' ${data:+--data "$data"} \
    "$BLACKARK_URL$path"
}

nodes=$(api GET /v1/nodes)
test "$(jq '[.[] | select(.status == "healthy")] | length' <<<"$nodes")" -ge 2

app=$(api POST /v1/apps "{\"name\":\"$BLACKARK_APP_NAME\",\"image\":\"$BLACKARK_IMAGE\",\"replicas\":2}")
app_id=$(jq -er '.id' <<<"$app")
cleanup() {
  api DELETE "/v1/apps/$app_id" >/dev/null || true
  for _ in {1..30}; do
    state=$(api GET "/v1/apps/$app_id" 2>/dev/null || true)
    [ -n "$state" ] && [ "$(jq '[.instances[] | select(.status != "delete")] | length' <<<"$state")" -eq 0 ] && return
    sleep 2
  done
  echo "cleanup did not remove all app containers" >&2
  return 1
}
trap cleanup EXIT

api POST "/v1/apps/$app_id/deploy" '{}' >/dev/null
for _ in {1..60}; do
  state=$(api GET "/v1/apps/$app_id")
  [ "$(jq '[.instances[] | select(.status == "running")] | length' <<<"$state")" -eq 2 ] && break
  sleep 2
done
test "$(jq '[.instances[].node_id] | unique | length' <<<"$state")" -eq 2
curl_args=(--fail --silent --show-error)
if [ -n "$BLACKARK_CURL_RESOLVE" ]; then
  curl_args+=(--resolve "$BLACKARK_CURL_RESOLVE")
fi
if [ -n "$BLACKARK_CURL_INSECURE" ]; then
  curl_args+=(--insecure)
fi
curl "${curl_args[@]}" "https://$BLACKARK_DOMAIN/healthz" >/dev/null

api PATCH "/v1/apps/$app_id" '{"replicas":3}' >/dev/null
for _ in {1..60}; do
  state=$(api GET "/v1/apps/$app_id")
  [ "$(jq '[.instances[] | select(.status == "running")] | length' <<<"$state")" -eq 3 ] && break
  sleep 2
done
test "$(jq '[.instances[] | select(.status == "running")] | length' <<<"$state")" -eq 3

api GET "/v1/apps/$app_id/logs?tail=50" >/dev/null

if [ -z "$BLACKARK_SKIP_RESTART_CHECK" ]; then
  container_id=$(jq -er '[.instances[] | select(.status == "running" and .container_id != null)][0].container_id' <<<"$state")
  docker stop "$container_id" >/dev/null
  for _ in {1..30}; do
    running=$(docker inspect -f '{{.State.Running}}' "$container_id" 2>/dev/null || true)
    [ "$running" = "true" ] && break
    sleep 2
  done
  test "$running" = "true"
fi

echo "MVP verification passed: two workers, deploy, inspect, logs, HTTPS route, scale, restart, and cleanup"
