# Virt Node ISO Builder Design

## Goal

Build `dist/virt-node.iso` from the user's pinned Debian 13.5 amd64 netinst ISO without modifying the original ISO.

## Approach

The builder runs on Linux with `xorriso`. It verifies the base ISO SHA256 from `os/debian/base-iso.json`, extracts the existing BIOS and UEFI boot menu files, prepends an explicit automated Virt node install entry, and writes a new ISO with `xorriso -boot_image any replay` so the Debian boot equipment is preserved.

## Injected Files

- `/preseed.cfg` from `os/debian/preseed.cfg`
- `/virt/post-install.sh` from `os/debian/post-install.sh`
- `/isolinux/txt.cfg` with the BIOS automated install entry
- `/boot/grub/grub.cfg` with the UEFI automated install entry

## Safety

The automated entry is named `Install Virt node (automated, erases disk)`. The builder does not make unattended installation the implicit default; the operator must select it from the boot menu.

## Limitations

The local Windows environment can validate the script, checksum, and Debian ISO structure, but actual ISO remastering requires Linux tools, specifically `xorriso`.
