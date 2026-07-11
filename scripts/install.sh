#!/bin/sh
set -eu

REPO="${REPO:-sudowritecode/BlackArk}"
VERSION="${VERSION:-latest}"
ROLE="${ROLE:-all}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
ENV_DIR="${ENV_DIR:-/etc/blackark}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
BUNDLE_PATH="${BUNDLE_PATH:-}"

need() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "blackark install: missing required command: $1" >&2
		exit 1
	fi
}

as_root() {
	if [ "$(id -u)" -ne 0 ]; then
		if command -v sudo >/dev/null 2>&1; then
			sudo "$@"
		else
			echo "blackark install: rerun as root or install sudo" >&2
			exit 1
		fi
	else
		"$@"
	fi
}

detect_platform() {
	os="$(uname -s | tr '[:upper:]' '[:lower:]')"
	case "$(uname -m)" in
		x86_64|amd64) arch="amd64" ;;
		aarch64|arm64) arch="arm64" ;;
		*) echo "blackark install: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
	esac
	case "$os" in
		linux|darwin) ;;
		*) echo "blackark install: unsupported OS: $os" >&2; exit 1 ;;
	esac
	PLATFORM="$os-$arch"
}

download_bundle() {
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT INT TERM
	if [ "$BUNDLE_PATH" != "" ]; then
		if [ ! -f "$BUNDLE_PATH" ]; then
			echo "blackark install: BUNDLE_PATH does not exist: $BUNDLE_PATH" >&2
			exit 1
		fi
		echo "blackark install: using local bundle $BUNDLE_PATH"
		cp "$BUNDLE_PATH" "$tmp/blackark-bundle.tar.gz"
	elif [ "$VERSION" = "latest" ]; then
		url="https://github.com/$REPO/releases/latest/download/blackark-bundle-$PLATFORM.tar.gz"
		echo "blackark install: downloading $url"
		curl -fsSL "$url" -o "$tmp/blackark-bundle.tar.gz"
	else
		url="https://github.com/$REPO/releases/download/$VERSION/blackark-bundle-$VERSION-$PLATFORM.tar.gz"
		echo "blackark install: downloading $url"
		curl -fsSL "$url" -o "$tmp/blackark-bundle.tar.gz"
	fi
	tar -xzf "$tmp/blackark-bundle.tar.gz" -C "$tmp"
	BUNDLE_DIR="$(find "$tmp" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
	if [ ! -x "$BUNDLE_DIR/blackark" ]; then
		echo "blackark install: release bundle is missing BlackArk binaries" >&2
		exit 1
	fi
}

install_binaries() {
	as_root install -d -m 0755 "$INSTALL_DIR"
	case "$ROLE" in
		cli) bins="blackark" ;;
		control) bins="blackark blackark-control" ;;
		agent) bins="blackark-agent" ;;
		all|single-node) bins="blackark blackark-control blackark-agent" ;;
		*) echo "blackark install: ROLE must be cli, control, agent, all, or single-node" >&2; exit 1 ;;
	esac
	for bin in $bins; do
		as_root install -m 0755 "$BUNDLE_DIR/$bin" "$INSTALL_DIR/$bin"
	done
}

require_control_env() {
	if [ "${BLACKARK_API_TOKEN:-}" = "" ]; then
		echo "blackark install: set BLACKARK_API_TOKEN for ROLE=control or ROLE=single-node" >&2
		exit 1
	fi
	if [ "${BLACKARK_DATABASE_URL:-}" = "" ]; then
		echo "blackark install: set BLACKARK_DATABASE_URL for ROLE=control or ROLE=single-node" >&2
		exit 1
	fi
}

write_control_service() {
	require_control_env
	as_root install -d -m 0750 "$ENV_DIR"
	if ! id blackark >/dev/null 2>&1; then
		as_root useradd --system --home-dir /var/lib/blackark --create-home --shell /usr/sbin/nologin blackark
	fi
	tmp_env="$tmp/control.env"
	{
		printf 'BLACKARK_API_TOKEN=%s\n' "$BLACKARK_API_TOKEN"
		printf 'BLACKARK_DATABASE_URL=%s\n' "$BLACKARK_DATABASE_URL"
		printf 'BLACKARK_LISTEN_ADDR=%s\n' "${BLACKARK_LISTEN_ADDR:-:8080}"
		printf 'BLACKARK_DASHBOARD_ENABLED=%s\n' "${BLACKARK_DASHBOARD_ENABLED:-true}"
	} > "$tmp_env"
	as_root install -m 0600 -o root -g root "$tmp_env" "$ENV_DIR/control.env"

	tmp_unit="$tmp/blackark-control.service"
	cat > "$tmp_unit" <<EOF
[Unit]
Description=BlackArk control plane
After=network-online.target
Wants=network-online.target

[Service]
User=blackark
Group=blackark
EnvironmentFile=$ENV_DIR/control.env
ExecStart=$INSTALL_DIR/blackark-control serve
Restart=on-failure
RestartSec=5s
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=true

[Install]
WantedBy=multi-user.target
EOF
	as_root install -m 0644 "$tmp_unit" "$SYSTEMD_DIR/blackark-control.service"
	as_root systemctl daemon-reload
	as_root systemctl enable blackark-control.service
	as_root systemctl restart blackark-control.service
}

require_agent_env() {
	if [ "$ROLE" = "single-node" ] && [ "${BLACKARK_CONTROL_URL:-}" = "" ]; then
		BLACKARK_CONTROL_URL="http://127.0.0.1:8080"
		export BLACKARK_CONTROL_URL
	fi
	if [ "${BLACKARK_CONTROL_URL:-}" = "" ]; then
		echo "blackark install: set BLACKARK_CONTROL_URL for ROLE=agent or ROLE=single-node" >&2
		exit 1
	fi
	if [ "${BLACKARK_NODE_ID:-}" = "" ] || [ "${BLACKARK_NODE_TOKEN:-}" = "" ]; then
		if [ "${BLACKARK_JOIN_TOKEN:-}" = "" ]; then
			if [ "${BLACKARK_API_TOKEN:-}" = "" ]; then
				echo "blackark install: set BLACKARK_JOIN_TOKEN, BLACKARK_API_TOKEN, or BLACKARK_NODE_ID plus BLACKARK_NODE_TOKEN, for ROLE=agent or ROLE=single-node" >&2
				exit 1
			fi
		fi
	fi
}

env_line() {
	key="$1"
	value="$2"
	escaped=$(printf "%s" "$value" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\$/\\$/g; s/`/\\`/g')
	printf '%s="%s"\n' "$key" "$escaped"
}

agent_bootstrap_note() {
	if [ "${BLACKARK_NODE_ID:-}" = "" ] || [ "${BLACKARK_NODE_TOKEN:-}" = "" ]; then
		if [ "${BLACKARK_JOIN_TOKEN:-}" != "" ]; then
			echo "blackark install: agent will join on first start and persist node credentials to $ENV_DIR/agent.env"
		elif [ "${BLACKARK_API_TOKEN:-}" != "" ]; then
			echo "blackark install: agent will bootstrap a join token on first start and persist node credentials to $ENV_DIR/agent.env"
		fi
	fi
}

write_agent_service() {
	require_agent_env
	as_root install -d -m 0750 "$ENV_DIR"
	tmp_env="$tmp/agent.env"
	{
		env_line BLACKARK_CONTROL_URL "$BLACKARK_CONTROL_URL"
		env_line BLACKARK_NODE_NAME "${BLACKARK_NODE_NAME:-$(hostname)}"
		env_line BLACKARK_DOCKER_SOCKET "${BLACKARK_DOCKER_SOCKET:-/var/run/docker.sock}"
		env_line BLACKARK_AGENT_ENV_FILE "$ENV_DIR/agent.env"
		[ "${BLACKARK_JOIN_TOKEN:-}" = "" ] || env_line BLACKARK_JOIN_TOKEN "$BLACKARK_JOIN_TOKEN"
		[ "${BLACKARK_API_TOKEN:-}" = "" ] || env_line BLACKARK_API_TOKEN "$BLACKARK_API_TOKEN"
		[ "${BLACKARK_NODE_ID:-}" = "" ] || env_line BLACKARK_NODE_ID "$BLACKARK_NODE_ID"
		[ "${BLACKARK_NODE_TOKEN:-}" = "" ] || env_line BLACKARK_NODE_TOKEN "$BLACKARK_NODE_TOKEN"
	} > "$tmp_env"
	as_root install -m 0600 -o root -g root "$tmp_env" "$ENV_DIR/agent.env"
	agent_bootstrap_note

	tmp_unit="$tmp/blackark-agent.service"
	cat > "$tmp_unit" <<EOF
[Unit]
Description=BlackArk worker agent
After=network-online.target docker.service
Wants=network-online.target docker.service

[Service]
EnvironmentFile=$ENV_DIR/agent.env
ExecStart=$INSTALL_DIR/blackark-agent
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
	as_root install -m 0644 "$tmp_unit" "$SYSTEMD_DIR/blackark-agent.service"
	as_root systemctl daemon-reload
	as_root systemctl enable blackark-agent.service
	as_root systemctl restart blackark-agent.service
}

need curl
need tar
detect_platform
download_bundle
install_binaries

case "$ROLE" in
	control) write_control_service ;;
	agent) write_agent_service ;;
	single-node) write_control_service; write_agent_service ;;
	cli|all) ;;
esac

echo "blackark install: installed ROLE=$ROLE to $INSTALL_DIR"
