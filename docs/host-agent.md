# Host Agent

`warm-migrationd` now includes the first host-agent layer for Debian/KVM nodes.

## Endpoints

```text
GET /host/capabilities
GET /host/vms
```

`/host/capabilities` checks whether the node has the minimum local virtualization stack:

- `/dev/kvm`
- `qemu-system-x86_64`
- `virsh`
- `virt-install`
- OVMF firmware
- active `libvirtd.service`

`/host/vms` reads VM inventory from:

```bash
virsh --connect qemu:///system list --all
```

## CLI

```bash
virtctl host-capabilities
virtctl vm-list
```

## Current Scope

This stage is read-only. It detects host readiness and lists VMs. Create, start, stop, attach storage, and live migration operations belong to the next host-agent stage.
