#!/usr/bin/env bash
set -euo pipefail

VIRT_USER="${VIRT_USER:-virtadmin}"
REPO_URL="${REPO_URL:-https://github.com/germeh/virt.git}"
REPO_BRANCH="${REPO_BRANCH:-feature/native-warm-migration-core}"
SOURCE_DIR="${SOURCE_DIR:-/opt/virt}"
WARM_MIGRATION_LISTEN_ADDR="${WARM_MIGRATION_LISTEN_ADDR:-127.0.0.1:8080}"
WARM_MIGRATION_LOG_LEVEL="${WARM_MIGRATION_LOG_LEVEL:-info}"

export WARM_MIGRATION_LISTEN_ADDR
export WARM_MIGRATION_LOG_LEVEL

if ! command -v go >/dev/null 2>&1; then
  apt-get update
  apt-get install -y golang
fi

systemctl enable libvirtd.service
systemctl start libvirtd.service

if id "${VIRT_USER}" >/dev/null 2>&1; then
  usermod -aG libvirt,kvm "${VIRT_USER}"
fi

if [[ ! -d "${SOURCE_DIR}/.git" ]]; then
  rm -rf "${SOURCE_DIR}"
  git clone --branch "${REPO_BRANCH}" --depth 1 "${REPO_URL}" "${SOURCE_DIR}"
else
  git -C "${SOURCE_DIR}" fetch origin "${REPO_BRANCH}"
  git -C "${SOURCE_DIR}" checkout "${REPO_BRANCH}"
  git -C "${SOURCE_DIR}" pull --ff-only
fi

"${SOURCE_DIR}/scripts/install-node.sh"

systemctl enable warm-migrationd.service
systemctl restart warm-migrationd.service

for attempt in {1..30}; do
  if /usr/local/bin/virtctl health >/tmp/virtctl-health.json 2>/tmp/virtctl-health.err; then
    cat /tmp/virtctl-health.json
    exit 0
  fi
  sleep 1
done

cat /tmp/virtctl-health.err >&2 || true
echo "warm-migrationd did not become healthy" >&2
exit 1
