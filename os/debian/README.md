# Debian Appliance Layer

This directory defines the first Debian-based appliance layer for Virt. It does not modify the upstream ISO in place. It records the base ISO and provides installer files that can be injected into a custom image later.

## Base ISO

- File: `debian-13.5.0-amd64-netinst.iso`
- Local path: `C:\Users\вф\CODEX\OS\debian-13.5.0-amd64-netinst.iso`
- Manifest path from this worktree: `../../OS/debian-13.5.0-amd64-netinst.iso`
- SHA256: `95838884F5EA6C82421DFE6BAAA5A639DBBE6756C1E380F9FE7A7CB0C1949D2A`
- Variant: Debian amd64 netinst

Because this is a netinst image, the target host needs network access during installation.

## Files

- `base-iso.json` pins the exact local base ISO and checksum.
- `preseed.cfg` automates the Debian install, creates the first admin user, installs KVM/QEMU/libvirt packages, and runs the appliance post-install hook.
- `post-install.sh` bootstraps `/opt/virt`, installs `warm-migrationd` and `virtctl`, enables libvirt, enables the node service, and runs `virtctl health`.

## First Node Flow

The intended first-node flow is:

1. Boot the Debian netinst ISO with `preseed.cfg`.
2. Make `post-install.sh` available in the ISO at `/virt/post-install.sh`.
3. Debian installs the base system and virtualization packages.
4. The late command runs `/opt/virt-appliance/post-install.sh` in the installed target.
5. The node starts `warm-migrationd`.
6. Verify with:

```bash
virtctl health
```

## Current Limitation

Use `scripts/build-virt-node-iso.sh` to build `dist/virt-node.iso`. The builder copies these files into the Debian ISO and adds BIOS/UEFI boot entries for unattended installation.
