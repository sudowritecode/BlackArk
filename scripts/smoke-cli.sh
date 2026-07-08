#!/usr/bin/env bash
# CLI-driven end-to-end smoke test for BlackArk.
# Exercises every blackark command against a running control plane.
# No curl calls — the blackark binary is the only API client.
set -euo pipefail

: "${BLACKARK_BIN:=./bin/blackark}"
: "${BLACKARK_URL:?set BLACKARK_URL (control-plane URL, e.g. http://localhost:8080)}"
: "${BLACKARK_TOKEN:?set BLACKARK_TOKEN (control-plane API token)}"
: "${BLACKARK_APP_NAME:=smoke-$(date +%s)}"
: "${BLACKARK_IMAGE:=nginx:1.27-alpine}"
: "${BLACKARK_REPLICAS:=2}"
: "${BLACKARK_SKIP_RESTART_CHECK:=}"

export BLACKARK_CONTROL_URL="$BLACKARK_URL"
export BLACKARK_API_TOKEN="$BLACKARK_TOKEN"
# Use a temp config path to avoid clobbering an existing user config.
export BLACKARK_CONFIG=$(mktemp /tmp/blackark-smoke-config-XXXXXX.yaml)
trap 'rm -f "$BLACKARK_CONFIG"' EXIT

step() { printf '\n=== %s ===\n' "$*"; }

# --- login ---
step "login"
$BLACKARK_BIN login --url "$BLACKARK_URL" --token "$BLACKARK_TOKEN"
unset BLACKARK_CONTROL_URL BLACKARK_API_TOKEN  # config file is authoritative now

# --- health ---
step "health"
$BLACKARK_BIN health 2>/dev/null | python3 -c "import sys,json; assert json.load(sys.stdin).get('status')=='ok'"

# --- status ---
step "status"
$BLACKARK_BIN status 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); assert d.get('status')=='ok'"

# --- get nodes ---
step "get nodes"
$BLACKARK_BIN get -o json nodes 2>/dev/null | python3 -c "
import sys,json
nodes=json.load(sys.stdin)
healthy=[n for n in nodes if n.get('status')=='healthy']
assert len(healthy)>=2, f'need >=2 healthy nodes, got {len(healthy)}'
print(f'{len(healthy)} healthy nodes')
"

# --- dashboard ---
step "dashboard"
$BLACKARK_BIN dashboard 2>/dev/null | python3 -c "
import sys
data=sys.stdin.read()
assert 'BlackArk Cluster' in data, 'dashboard should show clusters'
assert 'Nodes:' in data or 'Apps:' in data, 'dashboard should show summary'
print(f'dashboard rendered ({len(data)} chars)')
"

# --- get apps (before creation) ---
step "get apps (before)"
$BLACKARK_BIN get -o json apps 2>/dev/null | python3 -c "import sys,json; assert json.load(sys.stdin)==[]"

# --- apply manifest ---
step "apply manifest"
MANIFEST=$(mktemp /tmp/blackark-smoke-manifest-XXXXXX.yaml)
trap 'rm -f "$MANIFEST" "$BLACKARK_CONFIG"' EXIT
cat > "$MANIFEST" <<YAML
apiVersion: blackark/v1
kind: App
metadata:
  name: $BLACKARK_APP_NAME
spec:
  image: $BLACKARK_IMAGE
  replicas: $BLACKARK_REPLICAS
YAML

$BLACKARK_BIN apply -f "$MANIFEST" 2>/dev/null | python3 -c "
import sys,json
app=json.load(sys.stdin)
assert app.get('name')=='$BLACKARK_APP_NAME', f'expected name $BLACKARK_APP_NAME'
assert app.get('replicas')==$BLACKARK_REPLICAS, f'expected $BLACKARK_REPLICAS replicas'
print(f'app {app[\"id\"]} created')
"
APP_ID=$($BLACKARK_BIN get -o json apps 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin)[0]['id'])")
echo "APP_ID=$APP_ID"

# --- describe app ---
step "describe app"
$BLACKARK_BIN describe -o json app "$APP_ID" 2>/dev/null | python3 -c "
import sys,json
app=json.load(sys.stdin)
assert app['name']=='$BLACKARK_APP_NAME'
print(f'described app {app[\"id\"]}')
"

# --- wait for deployment ---
step "wait for running instances"
for _ in $(seq 1 60); do
  ready=$($BLACKARK_BIN describe -o json app "$APP_ID" 2>/dev/null | python3 -c "
import sys,json
app=json.load(sys.stdin)
instances=[i for i in app.get('instances',[]) if i.get('status')=='running']
print(len(instances))
" 2>/dev/null || echo 0)
  if [ "$ready" -ge "$BLACKARK_REPLICAS" ]; then
    echo "all $BLACKARK_REPLICAS instances running"
    break
  fi
  sleep 2
done
if [ "$ready" -lt "$BLACKARK_REPLICAS" ]; then
  echo "ERROR: only $ready/$BLACKARK_REPLICAS instances running after timeout" >&2
  exit 1
fi

# --- logs ---
step "logs"
$BLACKARK_BIN logs "$APP_ID" 2>/dev/null | python3 -c "
import sys
data=sys.stdin.read()
assert len(data)>0, 'expected non-empty logs'
print(f'got {len(data)} bytes of logs')
"

# --- scale up ---
step "scale up"
$BLACKARK_BIN scale "$APP_ID" 3 2>/dev/null | python3 -c "
import sys,json
app=json.load(sys.stdin)
assert app.get('replicas')==3, f'expected 3 replicas'
print('scaled to 3')
"
for _ in $(seq 1 60); do
  ready=$($BLACKARK_BIN describe -o json app "$APP_ID" 2>/dev/null | python3 -c "
import sys,json
app=json.load(sys.stdin)
print(len([i for i in app.get('instances',[]) if i.get('status')=='running']))
" 2>/dev/null || echo 0)
  if [ "$ready" -ge 3 ]; then
    echo "all 3 instances running"
    break
  fi
  sleep 2
done

# --- scale down ---
step "scale down"
$BLACKARK_BIN scale "$APP_ID" 1 2>/dev/null | python3 -c "
import sys,json
app=json.load(sys.stdin)
assert app.get('replicas')==1, f'expected 1 replica'
print('scaled to 1')
"
for _ in $(seq 1 60); do
  ready=$($BLACKARK_BIN describe -o json app "$APP_ID" 2>/dev/null | python3 -c "
import sys,json
app=json.load(sys.stdin)
print(len([i for i in app.get('instances',[]) if i.get('status')=='running']))
" 2>/dev/null || echo 0)
  if [ "$ready" -le 1 ]; then
    echo "down to 1 instance"
    break
  fi
  sleep 2
done

# --- restart ---
if [ -z "${BLACKARK_SKIP_RESTART_CHECK:-}" ]; then
  step "restart app"
  $BLACKARK_BIN restart "$APP_ID" 2>/dev/null | python3 -c "
import sys,json
app=json.load(sys.stdin)
print(f'restart accepted')
"
  sleep 3
  $BLACKARK_BIN describe -o json app "$APP_ID" 2>/dev/null | python3 -c "
import sys,json
app=json.load(sys.stdin)
running=[i for i in app.get('instances',[]) if i.get('status')=='running']
assert len(running)==1, f'expected 1 running after restart, got {len(running)}'
print('restart verified')
"
fi

# --- delete app ---
step "delete app"
$BLACKARK_BIN delete app "$APP_ID" 2>/dev/null | python3 -c "
import sys
data=sys.stdin.read()
assert 'deleted' in data, f'expected delete confirmation'
print('delete accepted')
"

for _ in $(seq 1 60); do
  remaining=$($BLACKARK_BIN get -o json apps 2>/dev/null | python3 -c "
import sys,json
apps=json.load(sys.stdin)
print(len([a for a in apps if a['name']=='$BLACKARK_APP_NAME']))
" 2>/dev/null || echo 1)
  if [ "$remaining" -eq 0 ]; then
    echo "app fully removed"
    break
  fi
  sleep 2
done

# --- error handling: invalid manifest ---
step "error handling: invalid manifest"
BAD=$(mktemp /tmp/blackark-bad-manifest-XXXXXX.yaml)
cat > "$BAD" <<YAML
apiVersion: blackark/v1
kind: Pod
metadata:
  name: bad
spec:
  image: nginx
YAML
if $BLACKARK_BIN apply -f "$BAD" 2>/dev/null; then
  echo "ERROR: expected failure for bad kind" >&2
  exit 1
fi
echo "correctly rejected invalid manifest"
rm -f "$BAD"

# --- error handling: not found ---
step "error handling: describe nonexistent"
if $BLACKARK_BIN describe app "nonexistent-id" 2>/dev/null; then
  echo "ERROR: expected failure for nonexistent app" >&2
  exit 1
fi
echo "correctly rejected nonexistent app"

step "ALL CHECKS PASSED"
