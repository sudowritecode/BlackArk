#!/usr/bin/env bash
set -euo pipefail

: "${BLACKARK_URL:?set BLACKARK_URL}"
: "${BLACKARK_TOKEN:?set BLACKARK_TOKEN}"
: "${BLACKARK_IMAGE:=nginx:1.27-alpine}"
: "${BLACKARK_DOMAIN:?set BLACKARK_DOMAIN}"

api() {
  local method=$1 path=$2 data=${3:-}
  curl --fail-with-body --silent --show-error \
    -X "$method" -H "Authorization: Bearer $BLACKARK_TOKEN" \
    -H 'Content-Type: application/json' ${data:+--data "$data"} \
    "$BLACKARK_URL$path"
}

nodes=$(api GET /v1/nodes)
test "$(jq '[.[] | select(.status == "healthy")] | length' <<<"$nodes")" -ge 2

app=$(api POST /v1/apps "{\"name\":\"mvp-smoke\",\"image\":\"$BLACKARK_IMAGE\",\"replicas\":2,\"domain\":\"$BLACKARK_DOMAIN\"}")
app_id=$(jq -er '.id' <<<"$app")
cleanup() { api DELETE "/v1/apps/$app_id" >/dev/null || true; }
trap cleanup EXIT

api POST "/v1/apps/$app_id/deploy" '{}' >/dev/null
for _ in {1..60}; do
  state=$(api GET "/v1/apps/$app_id")
  [ "$(jq '[.instances[] | select(.status == "running")] | length' <<<"$state")" -eq 2 ] && break
  sleep 2
done
test "$(jq '[.instances[].node_id] | unique | length' <<<"$state")" -eq 2
curl --fail --silent --show-error "https://$BLACKARK_DOMAIN" >/dev/null

api PATCH "/v1/apps/$app_id" '{"replicas":3}' >/dev/null
echo "MVP verification passed: two workers, deploy, HTTPS route, and scale request"

