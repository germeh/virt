#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="${SERVICE_NAME:-warm-migrationd}"
PREFIX="${PREFIX:-/usr/local}"
BIN_DIR="${BIN_DIR:-${PREFIX}/bin}"
CONFIG_DIR="${WARM_MIGRATION_CONFIG_DIR:-/etc/virt}"
DATA_DIR="${WARM_MIGRATION_DATA_DIR:-/var/lib/virt}"
LISTEN_ADDR="${WARM_MIGRATION_LISTEN_ADDR:-127.0.0.1:8080}"
LOG_LEVEL="${WARM_MIGRATION_LOG_LEVEL:-info}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

usage() {
  cat <<'USAGE'
Usage: sudo ./scripts/install-node.sh

Environment overrides:
  PREFIX=/usr/local
  BIN_DIR=/usr/local/bin
  WARM_MIGRATION_CONFIG_DIR=/etc/virt
  WARM_MIGRATION_DATA_DIR=/var/lib/virt
  WARM_MIGRATION_LISTEN_ADDR=127.0.0.1:8080
  WARM_MIGRATION_LOG_LEVEL=info
  SYSTEMD_DIR=/etc/systemd/system
  SKIP_SYSTEMD=1
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ "$(id -u)" -ne 0 ]]; then
  echo "install-node.sh must run as root; use sudo" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Go 1.22 or newer is required to build warm-migrationd and virtctl" >&2
  exit 1
fi

mkdir -p "${BIN_DIR}" "${CONFIG_DIR}" "${DATA_DIR}"

cd "${REPO_ROOT}"
go build -o "${BIN_DIR}/warm-migrationd" ./cmd/warm-migrationd
go build -o "${BIN_DIR}/virtctl" ./cmd/virtctl

chmod 0755 "${BIN_DIR}/warm-migrationd" "${BIN_DIR}/virtctl"

cat > "${CONFIG_DIR}/virt-node.env" <<EOF
WARM_MIGRATION_LISTEN_ADDR=${LISTEN_ADDR}
WARM_MIGRATION_CONFIG_DIR=${CONFIG_DIR}
WARM_MIGRATION_DATA_DIR=${DATA_DIR}
WARM_MIGRATION_LOG_LEVEL=${LOG_LEVEL}
EOF
chmod 0644 "${CONFIG_DIR}/virt-node.env"

if [[ "${SKIP_SYSTEMD:-0}" == "1" ]]; then
  echo "Installed binaries and config; skipped systemd setup because SKIP_SYSTEMD=1"
  exit 0
fi

if ! command -v systemctl >/dev/null 2>&1; then
  echo "Installed binaries and config; systemctl not found, service setup skipped" >&2
  exit 0
fi

mkdir -p "${SYSTEMD_DIR}"
cat > "${SYSTEMD_DIR}/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Warm Migration Node Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=${CONFIG_DIR}/virt-node.env
ExecStart=${BIN_DIR}/warm-migrationd
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "${SERVICE_NAME}.service"
systemctl restart "${SERVICE_NAME}.service"

echo "Installed ${SERVICE_NAME}. Check status with: systemctl status ${SERVICE_NAME}.service"
