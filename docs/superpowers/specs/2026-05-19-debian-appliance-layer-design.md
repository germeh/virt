# Debian Appliance Layer Design

## Goal

Create the first reproducible Debian-based installation layer for Virt without writing an operating system from scratch.

## Scope

This stage records the exact Debian netinst ISO supplied by the user, adds unattended installation configuration, and defines a post-install bootstrap path for the current node service.

This stage does not yet remaster the ISO. The following stage will build `virt-node.iso` by injecting these files and boot parameters into the Debian installer image.

## Components

- `os/debian/base-iso.json` pins the local Debian ISO path and SHA256.
- `os/debian/preseed.cfg` automates the Debian installation, installs base virtualization packages, and invokes the appliance post-install script.
- `os/debian/post-install.sh` installs or updates the Virt repository on the target system, runs `scripts/install-node.sh`, enables libvirt, enables `warm-migrationd`, and verifies health with `virtctl health`.
- `os/debian/README.md` documents the manual first-node flow and current limitations.

## Safety

The default service listen address remains `127.0.0.1:8080`. Exposing it on the network should be a deliberate operator choice until authentication and cluster authorization are implemented.
