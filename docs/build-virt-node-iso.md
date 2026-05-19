# Build `virt-node.iso`

This step builds a Debian-based Virt installer ISO from `debian-13.5.0-amd64-netinst.iso`.

The builder injects:

- `os/debian/preseed.cfg` as `/preseed.cfg`
- `os/debian/post-install.sh` as `/virt/post-install.sh`
- BIOS boot menu entry in `isolinux/txt.cfg`
- UEFI boot menu entry in `boot/grub/grub.cfg`

## Requirements

Run this on Linux, Debian, or WSL with a Linux distro installed. Required tools:

```bash
sudo apt-get update
sudo apt-get install -y xorriso python3 coreutils
```

The script needs:

- `xorriso`
- `python3`
- `sha256sum`

## Build

From the repository root:

```bash
scripts/build-virt-node-iso.sh
```

Default output:

```text
dist/virt-node.iso
```

The script reads `os/debian/base-iso.json`, resolves `debian-13.5.0-amd64-netinst.iso`, and verifies SHA256 before building.

To override paths:

```bash
BASE_ISO=/mnt/c/Users/вф/CODEX/OS/debian-13.5.0-amd64-netinst.iso \
OUTPUT_ISO=dist/virt-node.iso \
scripts/build-virt-node-iso.sh
```

## Install Warning

Automated install erases the target disk. Use a test VM first.

The ISO adds a boot entry named:

```text
Install Virt node (automated, erases disk)
```

Selecting that entry starts the Debian preseed flow, installs the virtualization packages, runs `post-install.sh`, starts `warm-migrationd`, and verifies the node with:

```bash
virtctl health
```
