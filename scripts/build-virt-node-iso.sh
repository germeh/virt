#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

MANIFEST_FILE="${MANIFEST_FILE:-${REPO_ROOT}/os/debian/base-iso.json}"
PRESEED_FILE="${PRESEED_FILE:-${REPO_ROOT}/os/debian/preseed.cfg}"
POST_INSTALL_FILE="${POST_INSTALL_FILE:-${REPO_ROOT}/os/debian/post-install.sh}"
OUTPUT_ISO="${OUTPUT_ISO:-${REPO_ROOT}/dist/virt-node.iso}"
VOLUME_ID="${VOLUME_ID:-Virt-Node}"
WORK_DIR="${WORK_DIR:-}"

usage() {
  cat <<'USAGE'
Usage: scripts/build-virt-node-iso.sh

Builds dist/virt-node.iso from the pinned Debian netinst ISO.

Environment overrides:
  BASE_ISO=/path/to/debian.iso
  OUTPUT_ISO=/path/to/virt-node.iso
  MANIFEST_FILE=os/debian/base-iso.json
  PRESEED_FILE=os/debian/preseed.cfg
  POST_INSTALL_FILE=os/debian/post-install.sh
  WORK_DIR=/tmp/virt-node-iso

Required Linux tools:
  xorriso python3 sha256sum

Warning:
  The automated installer entry uses preseed partitioning and erases the
  selected target disk.
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

require_command xorriso
require_command python3
require_command sha256sum

manifest_value() {
  local key="$1"
  python3 - "$MANIFEST_FILE" "$key" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as f:
    manifest = json.load(f)

print(manifest[sys.argv[2]])
PY
}

manifest_path="$(manifest_value path)"
expected_hash="$(manifest_value sha256)"

if [[ -z "${BASE_ISO:-}" ]]; then
  if [[ "${manifest_path}" = /* ]]; then
    BASE_ISO="${manifest_path}"
  else
    BASE_ISO="$(realpath -m "${REPO_ROOT}/${manifest_path}")"
  fi
fi

if [[ ! -f "${BASE_ISO}" ]]; then
  echo "base ISO not found: ${BASE_ISO}" >&2
  exit 1
fi

actual_hash="$(sha256sum "${BASE_ISO}" | awk '{print toupper($1)}')"
expected_hash="$(printf '%s' "${expected_hash}" | tr '[:lower:]' '[:upper:]')"
if [[ "${actual_hash}" != "${expected_hash}" ]]; then
  echo "base ISO SHA256 mismatch" >&2
  echo "  expected: ${expected_hash}" >&2
  echo "  actual:   ${actual_hash}" >&2
  exit 1
fi

for required_file in "${PRESEED_FILE}" "${POST_INSTALL_FILE}"; do
  if [[ ! -f "${required_file}" ]]; then
    echo "required appliance file not found: ${required_file}" >&2
    exit 1
  fi
done

cleanup_work_dir=0
if [[ -z "${WORK_DIR}" ]]; then
  WORK_DIR="$(mktemp -d)"
  cleanup_work_dir=1
else
  mkdir -p "${WORK_DIR}"
fi

cleanup() {
  if [[ "${cleanup_work_dir}" == "1" ]]; then
    rm -rf "${WORK_DIR}"
  fi
}
trap cleanup EXIT

TXT_CFG="${WORK_DIR}/txt.cfg"
GRUB_CFG="${WORK_DIR}/grub.cfg"

xorriso -osirrox on -indev "${BASE_ISO}" \
  -extract /isolinux/txt.cfg "${TXT_CFG}" \
  -extract /boot/grub/grub.cfg "${GRUB_CFG}" \
  >/dev/null 2>&1

if ! grep -q "label virt-auto" "${TXT_CFG}"; then
  cp "${TXT_CFG}" "${TXT_CFG}.orig"
  cat > "${TXT_CFG}" <<'EOF'
label virt-auto
	menu label ^Install Virt node (automated, erases disk)
	kernel /install.amd/vmlinuz
	append auto=true priority=critical preseed/file=/cdrom/preseed.cfg vga=788 initrd=/install.amd/initrd.gz --- quiet

EOF
  cat "${TXT_CFG}.orig" >> "${TXT_CFG}"
fi

if ! grep -q "Install Virt node (automated, erases disk)" "${GRUB_CFG}"; then
  cp "${GRUB_CFG}" "${GRUB_CFG}.orig"
  cat > "${GRUB_CFG}" <<'EOF'
menuentry "Install Virt node (automated, erases disk)" {
	set gfxpayload=keep
	linux /install.amd/vmlinuz auto=true priority=critical preseed/file=/cdrom/preseed.cfg --- quiet
	initrd /install.amd/initrd.gz
}

EOF
  cat "${GRUB_CFG}.orig" >> "${GRUB_CFG}"
fi

mkdir -p "$(dirname -- "${OUTPUT_ISO}")"
rm -f "${OUTPUT_ISO}"

xorriso -indev "${BASE_ISO}" -outdev "${OUTPUT_ISO}" \
  -boot_image any replay \
  -volid "${VOLUME_ID}" \
  -map "${PRESEED_FILE}" /preseed.cfg \
  -mkdir /virt \
  -map "${POST_INSTALL_FILE}" /virt/post-install.sh \
  -map "${TXT_CFG}" /isolinux/txt.cfg \
  -map "${GRUB_CFG}" /boot/grub/grub.cfg \
  -chmod 0444 /preseed.cfg /isolinux/txt.cfg /boot/grub/grub.cfg \
  -chmod 0555 /virt/post-install.sh \
  -end

output_hash="$(sha256sum "${OUTPUT_ISO}" | awk '{print toupper($1)}')"

cat <<EOF
Built ${OUTPUT_ISO}
Base ISO: ${BASE_ISO}
Output SHA256: ${output_hash}
Boot entries: Install Virt node (automated, erases disk)
EOF
