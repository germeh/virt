# Host Agent Read-Only Design

## Goal

Add the first host-agent layer to `warm-migrationd` so a Virt node can report local KVM/libvirt readiness and VM inventory.

## Scope

This stage is intentionally read-only. It does not create, start, stop, delete, or migrate VMs. It provides the discovery surface needed before adding lifecycle operations.

## Components

- `internal/host` owns host capability detection and `virsh` VM inventory parsing.
- `internal/api` exposes `GET /host/capabilities` and `GET /host/vms`.
- `cmd/warm-migrationd` wires the API to the local host service.
- `cmd/virtctl` exposes `host-capabilities` and `vm-list` commands.

## Compatibility

The implementation is pure Go and can compile on Windows, but useful runtime results require a Debian/KVM node with `/dev/kvm`, QEMU, libvirt, virt-install, OVMF, and active `libvirtd.service`.
