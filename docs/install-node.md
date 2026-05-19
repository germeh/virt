# Install a Warm Migration Node

This installs the current warm migration node service on a Linux host. At this stage the node exposes the platform API skeleton and health endpoint. VM host control and ISO installation are separate later stages.

## Requirements

- Linux host with systemd for managed service mode.
- Go 1.22 or newer available on `PATH`.
- Root privileges for installing binaries under `/usr/local/bin`, config under `/etc/virt`, and the systemd unit.

## Install

```bash
git clone https://github.com/germeh/virt.git
cd virt
git checkout feature/native-warm-migration-core
sudo ./scripts/install-node.sh
```

The installer builds and installs:

- `/usr/local/bin/warm-migrationd`
- `/usr/local/bin/virtctl`
- `/etc/virt/virt-node.env`
- `/etc/systemd/system/warm-migrationd.service`

## Configure

Override defaults with environment variables:

```bash
sudo WARM_MIGRATION_LISTEN_ADDR=0.0.0.0:8080 ./scripts/install-node.sh
```

Default config:

```text
WARM_MIGRATION_LISTEN_ADDR=127.0.0.1:8080
WARM_MIGRATION_CONFIG_DIR=/etc/virt
WARM_MIGRATION_DATA_DIR=/var/lib/virt
WARM_MIGRATION_LOG_LEVEL=info
```

## Verify

```bash
systemctl status warm-migrationd.service
virtctl health
curl http://127.0.0.1:8080/healthz
```
