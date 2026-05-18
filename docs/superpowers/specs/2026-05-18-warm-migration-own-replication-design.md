# Warm Migration With Native Replication Engine Design

## Goal

Build warm migration between two independent clusters of the future virtualization
system. Both clusters are managed by our platform, use the same VM metadata
format, and expose the same cluster API.

The first implementation uses a native replication engine instead of Ceph,
shared storage, or VMware-derived components. The expected downtime is short but
not zero: the VM keeps running during the bulk disk copy, then stops briefly for
final synchronization and boot on the target cluster.

## Scope

Included in the first design:

- Two clusters managed by our own platform.
- VM disk replication from source cluster to target cluster.
- Incremental block synchronization while the VM is still running.
- Final cutover with VM stop, last delta sync, registration, and boot on target.
- Compatibility checks before migration starts.
- Network and storage mapping between clusters.
- Rollback before target boot if validation or final sync fails.
- Source VM lock/archive after successful target boot to prevent double start.

Excluded from the first design:

- ESXi code, ESXi installer reuse, or VMware proprietary component reuse.
- Live memory migration between clusters.
- Migration from third-party platforms such as ESXi, vSphere, or Proxmox.
- Cross-cluster HA failover without operator intent.
- GPU, USB, PCI passthrough migration.
- Distributed storage as a required dependency.

## Architecture

Each cluster has a Cluster Manager and one or more Hypervisor Nodes. A Migration
Gateway runs as part of the cluster control plane and handles authenticated
communication with remote clusters.

Main components:

- Cluster Manager: owns VM inventory, placement decisions, tasks, locks, and
  cluster health.
- Hypervisor Agent: runs on each node and controls QEMU/KVM, libvirt, snapshots,
  dirty block tracking, and disk operations.
- Migration Gateway: exposes secure inter-cluster APIs for discovery,
  compatibility checks, transfer sessions, and cutover commands.
- Replication Engine: copies disk data and applies incremental block deltas to
  the target disk image.
- Migration Orchestrator: coordinates the full workflow as an idempotent task
  with resumable phases.
- Console/API/UI Layer: lets an operator select a VM, target cluster, target
  storage, and network mappings.

## Storage Model

The first version supports local or network-mounted storage on each cluster,
but it does not require shared storage between clusters.

Supported disk formats for the first version:

- qcow2 for snapshot-friendly operation.
- raw for simpler high-throughput transfer when snapshots are handled by the
  host storage layer.

The replication engine treats disks as ordered block streams. It records a
replication manifest containing disk identity, virtual size, format, block size,
checksums, current generation, and dirty-range state.

## VM Metadata

Each VM has a portable metadata document containing:

- VM UUID and human-readable name.
- CPU profile, vCPU count, memory size, firmware mode, machine type.
- Disk list, disk bus, boot order, disk format, disk size.
- Network interfaces, MAC policy, source network labels.
- Cloud-init or guest customization settings if enabled.
- Snapshot and checkpoint metadata relevant to migration.

Cluster-local fields are separated from portable fields. During migration,
source networks and storage classes are mapped to target equivalents.

## Warm Migration Flow

1. Operator selects a VM in Cluster A and chooses Cluster B as the target.
2. Source Cluster Manager creates a migration task and locks destructive VM
   operations.
3. Target cluster runs compatibility checks for CPU profile, memory, firmware,
   disk format, storage capacity, network mappings, and policy constraints.
4. Source creates an initial disk checkpoint while the VM continues running.
5. Replication Engine copies the base disk state to the target cluster.
6. Source tracks new writes using dirty bitmaps or incremental checkpoints.
7. Replication Engine sends incremental deltas until the remaining dirty data is
   below the configured cutover threshold.
8. Operator approves cutover or an automatic policy triggers it.
9. Source stops the VM cleanly, freezes the final disk state, and sends the last
   delta.
10. Target verifies disk manifests, registers the VM, maps networks/storage, and
    boots the VM.
11. Source marks the old VM as migrated and locked/archived.
12. The migration task records final status, timings, checksums, and target VM
    identity.

## Replication Engine

The native replication engine has three phases:

- Base sync: copy all allocated disk blocks or all raw blocks depending on disk
  type and policy.
- Incremental sync: repeatedly copy dirty ranges generated after the base
  checkpoint.
- Final sync: stop the VM, flush guest-visible disk state, copy the final dirty
  ranges, and seal the target disk.

The engine must support:

- Chunked transfer with retry.
- Per-chunk checksums.
- Resume from the last verified chunk.
- Bandwidth limits.
- Compression as an optional transfer setting.
- Encryption in transit through mTLS between Migration Gateways.
- Progress reporting by disk, phase, bytes, dirty bytes, and estimated cutover
  window.

The first implementation can use QEMU dirty bitmaps where available. If dirty
bitmaps are not available for a disk backend, the engine falls back to
incremental snapshots.

## Cutover Safety

Only one cluster may own a runnable VM instance at a time.

Safety rules:

- Source VM receives a migration lock before replication begins.
- Target VM cannot boot until final sync is verified.
- Source VM is stopped before final sync.
- Source VM remains locked after successful target boot.
- If target boot fails, source can be unlocked and booted again if final
  ownership transfer was not completed.
- Once target boot is confirmed and ownership is committed, automatic rollback
  is disabled and recovery becomes an explicit operator action.

## Failure Handling

Before final cutover:

- Failed transfers can resume.
- Failed compatibility checks stop the task without changing the VM.
- Failed target preparation removes partial target VM registration.
- Source VM keeps running unless the operator cancels it.

During final cutover:

- If final sync fails before target boot, source VM remains the owner and can be
  restarted.
- If target registration fails, target artifacts are cleaned up and source
  remains owner.
- If target boot succeeds but post-boot health check fails, the task enters a
  manual recovery state to avoid unsafe automatic double-start.

## APIs

Initial API surface:

- POST /migrations/preflight
- POST /migrations
- GET /migrations/{id}
- POST /migrations/{id}/pause
- POST /migrations/{id}/resume
- POST /migrations/{id}/cancel
- POST /migrations/{id}/cutover
- GET /clusters/remotes
- POST /clusters/remotes

The API reports structured phase status and machine-readable failure reasons.

## Testing Strategy

Unit tests:

- VM metadata portability and validation.
- Network/storage mapping validation.
- Replication manifest generation.
- Dirty range merge and ordering logic.
- Migration state machine transitions.

Integration tests:

- Migrate a small VM between two local test clusters.
- Resume interrupted base sync.
- Resume interrupted incremental sync.
- Fail preflight due to missing target network.
- Fail final sync and verify source ownership remains intact.
- Successful cutover and source lock/archive behavior.

System tests:

- Multiple VM sizes.
- Multiple disks per VM.
- Bandwidth-limited replication.
- Long-running dirty workload before cutover.
- Target boot health checks.

## Initial Technical Decisions

- Default disk format: qcow2-first. Raw disks can be added as a secondary path
  after qcow2 migration is stable.
- Control plane implementation language: Go. Agents, APIs, concurrency, and
  static binaries fit the system shape well.
- Cluster Manager state database: PostgreSQL. Reliability and transactional
  ownership changes are more important than single-node minimalism for this
  migration design.
- UI framework and installer design are intentionally separate specs so the
  migration engine can be designed and tested independently.
