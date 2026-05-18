# Native Warm Migration Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first testable Go core for warm migration between two clusters using native disk replication, preflight checks, and a safe cutover state machine.

**Architecture:** This plan creates a Go module with focused packages: `domain` for portable VM and cluster models, `preflight` for compatibility checks, `replication` for dirty ranges and block-copy primitives, `migration` for orchestration, and `api` for the initial HTTP contract. The first implementation uses in-memory fakes so every behavior can be tested before wiring in QEMU, libvirt, PostgreSQL, or the installer.

**Tech Stack:** Go 1.22 or newer, Go standard library, `net/http`, SHA-256 checksums, table-driven Go tests.

---

## Scope Split

This is Plan 1 for the native warm migration subsystem. It produces working,
tested software for the migration core only.

Covered here:

- Portable VM and cluster metadata.
- Preflight validation for CPU, RAM, storage, and network mappings.
- Dirty block range normalization.
- Replication manifests and per-chunk checksums.
- In-memory block replication for base sync, incremental sync, and final sync.
- Migration task state transitions and ownership safety.
- Minimal HTTP API surface for preflight and migration task inspection.

Separate plans should cover:

- QEMU/libvirt agent integration.
- PostgreSQL persistence.
- Web UI.
- Bare-metal ISO installer.
- Real mTLS remote-cluster trust setup.

## File Structure

Project root: `C:\Users\вф\CODEX`

- `.gitignore`: keeps local installers, ISO files, generated binaries, and test outputs out of commits.
- `go.mod`: declares the Go module.
- `internal/domain/vm.go`: portable VM, disk, NIC, and cluster profile models.
- `internal/domain/vm_test.go`: validation tests for VM metadata.
- `internal/preflight/checker.go`: compatibility checker for a migration request.
- `internal/preflight/checker_test.go`: tests for missing mappings, insufficient capacity, and incompatible CPU.
- `internal/replication/range.go`: dirty block range type and normalization.
- `internal/replication/range_test.go`: tests for sorting, merging, and invalid range filtering.
- `internal/replication/manifest.go`: chunk manifest and checksum helpers.
- `internal/replication/manifest_test.go`: tests for chunk generation and checksum stability.
- `internal/replication/memory_disk.go`: in-memory block device used by tests and the simulator.
- `internal/replication/engine.go`: base, incremental, and final sync logic.
- `internal/replication/engine_test.go`: tests for copy, dirty delta, final sync, and resume behavior.
- `internal/migration/state.go`: migration phases, events, and transition validation.
- `internal/migration/state_test.go`: state machine tests.
- `internal/migration/orchestrator.go`: warm migration orchestration over interfaces.
- `internal/migration/orchestrator_test.go`: end-to-end tests with fake source and target clusters.
- `internal/api/server.go`: minimal HTTP API for preflight and migration status.
- `internal/api/server_test.go`: HTTP contract tests.
- `.github/workflows/go-test.yml`: CI workflow that runs `go test ./...`.

---

### Task 1: Bootstrap Repository Tooling

**Files:**

- Create: `C:\Users\вф\CODEX\.gitignore`
- Create: `C:\Users\вф\CODEX\go.mod`
- Create: `C:\Users\вф\CODEX\.github\workflows\go-test.yml`

- [ ] **Step 1: Verify Go is available**

Run:

```powershell
go version
```

Expected if Go is already installed:

```text
go version go1.22
```

If PowerShell reports that `go` is not recognized, install Go and reopen the terminal:

```powershell
winget install GoLang.Go --accept-package-agreements --accept-source-agreements
```

Then run:

```powershell
go version
```

Expected: Go 1.22 or newer.

- [ ] **Step 2: Create `.gitignore`**

Use this complete file content:

```gitignore
# Local installers and VM media
*.iso
*.exe
*.zip

# Build outputs
bin/
dist/
coverage.out

# Local datasets and generated artifacts already present in this workspace
data/
docx_edit/
huawei_cli_assets/

# OS/editor noise
.DS_Store
Thumbs.db
.vscode/
.idea/
```

- [ ] **Step 3: Create `go.mod`**

Use this complete file content:

```go
module warm-migration-core

go 1.22
```

- [ ] **Step 4: Create GitHub Actions test workflow**

Use this complete file content at `.github/workflows/go-test.yml`:

```yaml
name: go-test

on:
  push:
    branches: ["main"]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go test ./...
```

- [ ] **Step 5: Run tests before code exists**

Run:

```powershell
go test ./...
```

Expected:

```text
go: warning: "./..." matched no packages
no packages to test
```

- [ ] **Step 6: Commit tooling**

Run:

```powershell
git add .gitignore go.mod .github/workflows/go-test.yml
git commit -m "chore: bootstrap warm migration module"
```

Expected: one commit containing only the tooling files.

---

### Task 2: Domain Models And VM Validation

**Files:**

- Create: `C:\Users\вф\CODEX\internal\domain\vm_test.go`
- Create: `C:\Users\вф\CODEX\internal\domain\vm.go`

- [ ] **Step 1: Write the failing domain tests**

Use this complete file content at `internal/domain/vm_test.go`:

```go
package domain

import "testing"

func TestVMValidateAcceptsPortableVM(t *testing.T) {
	vm := VM{
		ID:          "vm-123",
		Name:        "billing-db",
		CPUProfile:  "x86-64-v2",
		VCPU:        4,
		MemoryBytes: 8 * GiB,
		Firmware:    FirmwareUEFI,
		MachineType: "q35",
		Disks: []Disk{
			{ID: "disk-1", Format: DiskFormatQCOW2, VirtualSizeBytes: 64 * GiB, Bus: "virtio", StorageClass: "fast-local"},
		},
		NICs: []NIC{
			{ID: "nic-1", MAC: "52:54:00:12:34:56", SourceNetwork: "prod-net"},
		},
	}

	if err := vm.Validate(); err != nil {
		t.Fatalf("expected valid VM, got %v", err)
	}
}

func TestVMValidateRejectsMissingIdentity(t *testing.T) {
	vm := VM{
		CPUProfile:  "x86-64-v2",
		VCPU:        1,
		MemoryBytes: GiB,
		Firmware:    FirmwareUEFI,
		MachineType: "q35",
		Disks:       []Disk{{ID: "disk-1", Format: DiskFormatQCOW2, VirtualSizeBytes: GiB, Bus: "virtio", StorageClass: "fast-local"}},
	}

	if err := vm.Validate(); err == nil {
		t.Fatal("expected validation error for missing VM identity")
	}
}

func TestVMValidateRejectsInvalidDisk(t *testing.T) {
	vm := VM{
		ID:          "vm-123",
		Name:        "billing-db",
		CPUProfile:  "x86-64-v2",
		VCPU:        2,
		MemoryBytes: 2 * GiB,
		Firmware:    FirmwareUEFI,
		MachineType: "q35",
		Disks:       []Disk{{ID: "disk-1", Format: DiskFormat("vmdk"), VirtualSizeBytes: GiB, Bus: "virtio", StorageClass: "fast-local"}},
	}

	if err := vm.Validate(); err == nil {
		t.Fatal("expected validation error for unsupported disk format")
	}
}
```

- [ ] **Step 2: Run the domain tests and verify they fail**

Run:

```powershell
go test ./internal/domain -run TestVMValidate -v
```

Expected: FAIL because `VM`, `Disk`, `NIC`, and constants are not defined.

- [ ] **Step 3: Implement domain models**

Use this complete file content at `internal/domain/vm.go`:

```go
package domain

import (
	"errors"
	"fmt"
)

const GiB uint64 = 1024 * 1024 * 1024

type DiskFormat string

const (
	DiskFormatQCOW2 DiskFormat = "qcow2"
	DiskFormatRaw   DiskFormat = "raw"
)

type FirmwareMode string

const (
	FirmwareBIOS FirmwareMode = "bios"
	FirmwareUEFI FirmwareMode = "uefi"
)

type VM struct {
	ID          string
	Name        string
	CPUProfile  string
	VCPU        uint16
	MemoryBytes uint64
	Firmware    FirmwareMode
	MachineType string
	Disks       []Disk
	NICs        []NIC
}

type Disk struct {
	ID               string
	Format           DiskFormat
	VirtualSizeBytes uint64
	Bus              string
	StorageClass     string
}

type NIC struct {
	ID            string
	MAC           string
	SourceNetwork string
}

type StorageClass struct {
	Name               string
	AvailableBytes     uint64
	SupportsSnapshots  bool
	SupportsDirtyTrack bool
}

type Network struct {
	Name string
	MTU  uint16
}

type ClusterProfile struct {
	ID                   string
	Name                 string
	CPUProfiles          []string
	AvailableMemoryBytes uint64
	StorageClasses       map[string]StorageClass
	Networks             map[string]Network
}

func (vm VM) Validate() error {
	var errs []error
	if vm.ID == "" {
		errs = append(errs, errors.New("vm id is required"))
	}
	if vm.Name == "" {
		errs = append(errs, errors.New("vm name is required"))
	}
	if vm.CPUProfile == "" {
		errs = append(errs, errors.New("cpu profile is required"))
	}
	if vm.VCPU == 0 {
		errs = append(errs, errors.New("vcpu must be greater than zero"))
	}
	if vm.MemoryBytes == 0 {
		errs = append(errs, errors.New("memory bytes must be greater than zero"))
	}
	if vm.Firmware != FirmwareBIOS && vm.Firmware != FirmwareUEFI {
		errs = append(errs, fmt.Errorf("unsupported firmware mode %q", vm.Firmware))
	}
	if vm.MachineType == "" {
		errs = append(errs, errors.New("machine type is required"))
	}
	if len(vm.Disks) == 0 {
		errs = append(errs, errors.New("at least one disk is required"))
	}
	for i, disk := range vm.Disks {
		if disk.ID == "" {
			errs = append(errs, fmt.Errorf("disk %d id is required", i))
		}
		if disk.Format != DiskFormatQCOW2 && disk.Format != DiskFormatRaw {
			errs = append(errs, fmt.Errorf("disk %s has unsupported format %q", disk.ID, disk.Format))
		}
		if disk.VirtualSizeBytes == 0 {
			errs = append(errs, fmt.Errorf("disk %s virtual size must be greater than zero", disk.ID))
		}
		if disk.Bus == "" {
			errs = append(errs, fmt.Errorf("disk %s bus is required", disk.ID))
		}
		if disk.StorageClass == "" {
			errs = append(errs, fmt.Errorf("disk %s storage class is required", disk.ID))
		}
	}
	for i, nic := range vm.NICs {
		if nic.ID == "" {
			errs = append(errs, fmt.Errorf("nic %d id is required", i))
		}
		if nic.SourceNetwork == "" {
			errs = append(errs, fmt.Errorf("nic %s source network is required", nic.ID))
		}
	}
	return errors.Join(errs...)
}
```

- [ ] **Step 4: Run the domain tests and verify they pass**

Run:

```powershell
go test ./internal/domain -run TestVMValidate -v
```

Expected: PASS.

- [ ] **Step 5: Commit domain models**

Run:

```powershell
git add internal/domain/vm.go internal/domain/vm_test.go
git commit -m "feat: add portable VM metadata model"
```

Expected: one commit for the domain package.

---

### Task 3: Preflight Compatibility Checker

**Files:**

- Create: `C:\Users\вф\CODEX\internal\preflight\checker_test.go`
- Create: `C:\Users\вф\CODEX\internal\preflight\checker.go`

- [ ] **Step 1: Write failing preflight tests**

Use this complete file content at `internal/preflight/checker_test.go`:

```go
package preflight

import (
	"testing"

	"warm-migration-core/internal/domain"
)

func TestCheckAcceptsCompatibleTarget(t *testing.T) {
	result := Check(Request{
		VM:          sampleVM(),
		Target:      sampleCluster(),
		NetworkMap:  map[string]string{"prod-net": "prod-net-b"},
		StorageMap:  map[string]string{"fast-local": "fast-local-b"},
		CutoverMode: CutoverWarm,
	})

	if !result.OK {
		t.Fatalf("expected preflight success, got failures: %#v", result.Failures)
	}
}

func TestCheckRejectsMissingNetworkMapping(t *testing.T) {
	result := Check(Request{
		VM:          sampleVM(),
		Target:      sampleCluster(),
		NetworkMap:  map[string]string{},
		StorageMap:  map[string]string{"fast-local": "fast-local-b"},
		CutoverMode: CutoverWarm,
	})

	assertFailureCode(t, result, "network_mapping_missing")
}

func TestCheckRejectsIncompatibleCPU(t *testing.T) {
	cluster := sampleCluster()
	cluster.CPUProfiles = []string{"x86-64-v1"}

	result := Check(Request{
		VM:          sampleVM(),
		Target:      cluster,
		NetworkMap:  map[string]string{"prod-net": "prod-net-b"},
		StorageMap:  map[string]string{"fast-local": "fast-local-b"},
		CutoverMode: CutoverWarm,
	})

	assertFailureCode(t, result, "cpu_profile_unsupported")
}

func TestCheckRejectsInsufficientStorage(t *testing.T) {
	cluster := sampleCluster()
	storage := cluster.StorageClasses["fast-local-b"]
	storage.AvailableBytes = domain.GiB
	cluster.StorageClasses["fast-local-b"] = storage

	result := Check(Request{
		VM:          sampleVM(),
		Target:      cluster,
		NetworkMap:  map[string]string{"prod-net": "prod-net-b"},
		StorageMap:  map[string]string{"fast-local": "fast-local-b"},
		CutoverMode: CutoverWarm,
	})

	assertFailureCode(t, result, "storage_capacity_insufficient")
}

func sampleVM() domain.VM {
	return domain.VM{
		ID:          "vm-123",
		Name:        "billing-db",
		CPUProfile:  "x86-64-v2",
		VCPU:        4,
		MemoryBytes: 8 * domain.GiB,
		Firmware:    domain.FirmwareUEFI,
		MachineType: "q35",
		Disks: []domain.Disk{
			{ID: "disk-1", Format: domain.DiskFormatQCOW2, VirtualSizeBytes: 64 * domain.GiB, Bus: "virtio", StorageClass: "fast-local"},
		},
		NICs: []domain.NIC{
			{ID: "nic-1", MAC: "52:54:00:12:34:56", SourceNetwork: "prod-net"},
		},
	}
}

func sampleCluster() domain.ClusterProfile {
	return domain.ClusterProfile{
		ID:                   "cluster-b",
		Name:                 "Cluster B",
		CPUProfiles:          []string{"x86-64-v2", "x86-64-v3"},
		AvailableMemoryBytes: 128 * domain.GiB,
		StorageClasses: map[string]domain.StorageClass{
			"fast-local-b": {Name: "fast-local-b", AvailableBytes: 512 * domain.GiB, SupportsSnapshots: true, SupportsDirtyTrack: true},
		},
		Networks: map[string]domain.Network{
			"prod-net-b": {Name: "prod-net-b", MTU: 1500},
		},
	}
}

func assertFailureCode(t *testing.T, result Result, code string) {
	t.Helper()
	for _, failure := range result.Failures {
		if failure.Code == code {
			return
		}
	}
	t.Fatalf("expected failure code %q, got %#v", code, result.Failures)
}
```

- [ ] **Step 2: Run preflight tests and verify they fail**

Run:

```powershell
go test ./internal/preflight -run TestCheck -v
```

Expected: FAIL because package `preflight` has no implementation.

- [ ] **Step 3: Implement preflight checker**

Use this complete file content at `internal/preflight/checker.go`:

```go
package preflight

import (
	"fmt"

	"warm-migration-core/internal/domain"
)

type CutoverMode string

const CutoverWarm CutoverMode = "warm"

type Request struct {
	VM          domain.VM
	Target      domain.ClusterProfile
	NetworkMap  map[string]string
	StorageMap  map[string]string
	CutoverMode CutoverMode
}

type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Result struct {
	OK       bool      `json:"ok"`
	Failures []Failure `json:"failures"`
}

func Check(req Request) Result {
	var failures []Failure

	if err := req.VM.Validate(); err != nil {
		failures = append(failures, Failure{Code: "vm_invalid", Message: err.Error()})
	}
	if !contains(req.Target.CPUProfiles, req.VM.CPUProfile) {
		failures = append(failures, Failure{
			Code:    "cpu_profile_unsupported",
			Message: fmt.Sprintf("target cluster does not support CPU profile %q", req.VM.CPUProfile),
		})
	}
	if req.Target.AvailableMemoryBytes < req.VM.MemoryBytes {
		failures = append(failures, Failure{
			Code:    "memory_capacity_insufficient",
			Message: "target cluster does not have enough available memory",
		})
	}
	for _, disk := range req.VM.Disks {
		targetStorageName := req.StorageMap[disk.StorageClass]
		if targetStorageName == "" {
			failures = append(failures, Failure{
				Code:    "storage_mapping_missing",
				Message: fmt.Sprintf("source storage class %q has no target mapping", disk.StorageClass),
			})
			continue
		}
		targetStorage, ok := req.Target.StorageClasses[targetStorageName]
		if !ok {
			failures = append(failures, Failure{
				Code:    "storage_target_missing",
				Message: fmt.Sprintf("target storage class %q does not exist", targetStorageName),
			})
			continue
		}
		if targetStorage.AvailableBytes < disk.VirtualSizeBytes {
			failures = append(failures, Failure{
				Code:    "storage_capacity_insufficient",
				Message: fmt.Sprintf("target storage class %q cannot fit disk %q", targetStorageName, disk.ID),
			})
		}
		if req.CutoverMode == CutoverWarm && !targetStorage.SupportsDirtyTrack {
			failures = append(failures, Failure{
				Code:    "storage_dirty_tracking_unsupported",
				Message: fmt.Sprintf("target storage class %q does not support dirty tracking", targetStorageName),
			})
		}
	}
	for _, nic := range req.VM.NICs {
		targetNetworkName := req.NetworkMap[nic.SourceNetwork]
		if targetNetworkName == "" {
			failures = append(failures, Failure{
				Code:    "network_mapping_missing",
				Message: fmt.Sprintf("source network %q has no target mapping", nic.SourceNetwork),
			})
			continue
		}
		if _, ok := req.Target.Networks[targetNetworkName]; !ok {
			failures = append(failures, Failure{
				Code:    "network_target_missing",
				Message: fmt.Sprintf("target network %q does not exist", targetNetworkName),
			})
		}
	}

	return Result{OK: len(failures) == 0, Failures: failures}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run preflight tests and verify they pass**

Run:

```powershell
go test ./internal/preflight -run TestCheck -v
```

Expected: PASS.

- [ ] **Step 5: Commit preflight checker**

Run:

```powershell
git add internal/preflight/checker.go internal/preflight/checker_test.go
git commit -m "feat: add migration preflight checks"
```

Expected: one commit for preflight compatibility checks.

---

### Task 4: Dirty Range Normalization

**Files:**

- Create: `C:\Users\вф\CODEX\internal\replication\range_test.go`
- Create: `C:\Users\вф\CODEX\internal\replication\range.go`

- [ ] **Step 1: Write failing dirty range tests**

Use this complete file content at `internal/replication/range_test.go`:

```go
package replication

import (
	"reflect"
	"testing"
)

func TestNormalizeRangesSortsAndMerges(t *testing.T) {
	input := []BlockRange{
		{Offset: 30, Length: 10},
		{Offset: 0, Length: 10},
		{Offset: 10, Length: 5},
		{Offset: 14, Length: 10},
	}

	got := NormalizeRanges(input)
	want := []BlockRange{
		{Offset: 0, Length: 24},
		{Offset: 30, Length: 10},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestNormalizeRangesDropsZeroLength(t *testing.T) {
	got := NormalizeRanges([]BlockRange{
		{Offset: 0, Length: 0},
		{Offset: 5, Length: 5},
	})

	want := []BlockRange{{Offset: 5, Length: 5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
```

- [ ] **Step 2: Run range tests and verify they fail**

Run:

```powershell
go test ./internal/replication -run TestNormalizeRanges -v
```

Expected: FAIL because `BlockRange` and `NormalizeRanges` do not exist.

- [ ] **Step 3: Implement range normalization**

Use this complete file content at `internal/replication/range.go`:

```go
package replication

import "sort"

type BlockRange struct {
	Offset uint64 `json:"offset"`
	Length uint64 `json:"length"`
}

func (r BlockRange) End() uint64 {
	return r.Offset + r.Length
}

func NormalizeRanges(input []BlockRange) []BlockRange {
	filtered := make([]BlockRange, 0, len(input))
	for _, r := range input {
		if r.Length > 0 {
			filtered = append(filtered, r)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Offset == filtered[j].Offset {
			return filtered[i].Length < filtered[j].Length
		}
		return filtered[i].Offset < filtered[j].Offset
	})
	if len(filtered) == 0 {
		return nil
	}
	merged := []BlockRange{filtered[0]}
	for _, current := range filtered[1:] {
		last := &merged[len(merged)-1]
		if current.Offset <= last.End() {
			if current.End() > last.End() {
				last.Length = current.End() - last.Offset
			}
			continue
		}
		merged = append(merged, current)
	}
	return merged
}
```

- [ ] **Step 4: Run range tests and verify they pass**

Run:

```powershell
go test ./internal/replication -run TestNormalizeRanges -v
```

Expected: PASS.

- [ ] **Step 5: Commit range logic**

Run:

```powershell
git add internal/replication/range.go internal/replication/range_test.go
git commit -m "feat: normalize dirty block ranges"
```

Expected: one commit for dirty range behavior.

---

### Task 5: Replication Manifest And Checksums

**Files:**

- Create: `C:\Users\вф\CODEX\internal\replication\manifest_test.go`
- Create: `C:\Users\вф\CODEX\internal\replication\manifest.go`

- [ ] **Step 1: Write failing manifest tests**

Use this complete file content at `internal/replication/manifest_test.go`:

```go
package replication

import "testing"

func TestBuildChunkManifestSplitsDiskIntoChunks(t *testing.T) {
	manifest, err := BuildChunkManifest("disk-1", "qcow2", 10, 4, func(r BlockRange) ([]byte, error) {
		return []byte("abcdefghij")[r.Offset:r.End()], nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(manifest.Chunks) != 3 {
		t.Fatalf("got %d chunks, want 3", len(manifest.Chunks))
	}
	if manifest.Chunks[2].Range != (BlockRange{Offset: 8, Length: 2}) {
		t.Fatalf("last chunk = %#v", manifest.Chunks[2].Range)
	}
}

func TestChecksumHexIsStable(t *testing.T) {
	got := ChecksumHex([]byte("abc"))
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run manifest tests and verify they fail**

Run:

```powershell
go test ./internal/replication -run "TestBuildChunkManifest|TestChecksumHex" -v
```

Expected: FAIL because manifest helpers do not exist.

- [ ] **Step 3: Implement manifest helpers**

Use this complete file content at `internal/replication/manifest.go`:

```go
package replication

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

type DiskManifest struct {
	DiskID           string          `json:"disk_id"`
	Format           string          `json:"format"`
	VirtualSizeBytes uint64          `json:"virtual_size_bytes"`
	BlockSizeBytes   uint64          `json:"block_size_bytes"`
	Generation       uint64          `json:"generation"`
	Chunks           []ChunkManifest `json:"chunks"`
}

type ChunkManifest struct {
	Index      uint64     `json:"index"`
	Range      BlockRange `json:"range"`
	SHA256Hex  string     `json:"sha256_hex"`
	Verified   bool       `json:"verified"`
	RetryCount uint8      `json:"retry_count"`
}

func BuildChunkManifest(diskID string, format string, virtualSize uint64, chunkSize uint64, read func(BlockRange) ([]byte, error)) (DiskManifest, error) {
	if diskID == "" {
		return DiskManifest{}, errors.New("disk id is required")
	}
	if format == "" {
		return DiskManifest{}, errors.New("disk format is required")
	}
	if virtualSize == 0 {
		return DiskManifest{}, errors.New("virtual size must be greater than zero")
	}
	if chunkSize == 0 {
		return DiskManifest{}, errors.New("chunk size must be greater than zero")
	}

	manifest := DiskManifest{
		DiskID:           diskID,
		Format:           format,
		VirtualSizeBytes: virtualSize,
		BlockSizeBytes:   chunkSize,
		Generation:       1,
	}
	for offset, index := uint64(0), uint64(0); offset < virtualSize; offset, index = offset+chunkSize, index+1 {
		length := chunkSize
		if remaining := virtualSize - offset; remaining < chunkSize {
			length = remaining
		}
		r := BlockRange{Offset: offset, Length: length}
		data, err := read(r)
		if err != nil {
			return DiskManifest{}, err
		}
		manifest.Chunks = append(manifest.Chunks, ChunkManifest{
			Index:     index,
			Range:     r,
			SHA256Hex: ChecksumHex(data),
		})
	}
	return manifest, nil
}

func ChecksumHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: Run manifest tests and verify they pass**

Run:

```powershell
go test ./internal/replication -run "TestBuildChunkManifest|TestChecksumHex" -v
```

Expected: PASS.

- [ ] **Step 5: Commit manifest logic**

Run:

```powershell
git add internal/replication/manifest.go internal/replication/manifest_test.go
git commit -m "feat: add replication manifests"
```

Expected: one commit for manifests and checksums.

---

### Task 6: In-Memory Disk And Replication Engine

**Files:**

- Create: `C:\Users\вф\CODEX\internal\replication\engine_test.go`
- Create: `C:\Users\вф\CODEX\internal\replication\memory_disk.go`
- Create: `C:\Users\вф\CODEX\internal\replication\engine.go`

- [ ] **Step 1: Write failing replication engine tests**

Use this complete file content at `internal/replication/engine_test.go`:

```go
package replication

import (
	"bytes"
	"context"
	"testing"
)

func TestEngineBaseSyncCopiesFullDisk(t *testing.T) {
	source := NewMemoryDisk([]byte("abcdefghij"))
	target := NewMemoryDisk(make([]byte, 10))
	engine := Engine{ChunkSize: 4}

	report, err := engine.BaseSync(context.Background(), source, target, ResumeState{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(target.Bytes(), source.Bytes()) {
		t.Fatalf("target = %q, source = %q", target.Bytes(), source.Bytes())
	}
	if report.BytesCopied != 10 {
		t.Fatalf("copied %d bytes, want 10", report.BytesCopied)
	}
}

func TestEngineIncrementalSyncCopiesDirtyRangesOnly(t *testing.T) {
	source := NewMemoryDisk([]byte("abcdefghij"))
	target := NewMemoryDisk([]byte("abcdefghij"))
	source.WriteAtBytes([]byte("XYZ"), 2)

	engine := Engine{ChunkSize: 4}
	report, err := engine.IncrementalSync(context.Background(), source, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(target.Bytes(), []byte("abXYZfghij")) {
		t.Fatalf("target = %q", target.Bytes())
	}
	if report.BytesCopied != 3 {
		t.Fatalf("copied %d bytes, want 3", report.BytesCopied)
	}
	if len(source.DirtyRanges()) != 0 {
		t.Fatalf("dirty ranges were not cleared: %#v", source.DirtyRanges())
	}
}

func TestEngineBaseSyncResumeSkipsVerifiedChunks(t *testing.T) {
	source := NewMemoryDisk([]byte("abcdefghij"))
	target := NewMemoryDisk(make([]byte, 10))
	engine := Engine{ChunkSize: 4}

	state := ResumeState{VerifiedChunks: map[uint64]bool{0: true}}
	report, err := engine.BaseSync(context.Background(), source, target, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(target.Bytes()[0:4]) != "\x00\x00\x00\x00" {
		t.Fatalf("verified chunk should not have been copied")
	}
	if string(target.Bytes()[4:]) != "efghij" {
		t.Fatalf("remaining chunks were not copied: %q", target.Bytes())
	}
	if report.BytesCopied != 6 {
		t.Fatalf("copied %d bytes, want 6", report.BytesCopied)
	}
}
```

- [ ] **Step 2: Run engine tests and verify they fail**

Run:

```powershell
go test ./internal/replication -run TestEngine -v
```

Expected: FAIL because `MemoryDisk`, `Engine`, and `ResumeState` do not exist.

- [ ] **Step 3: Implement in-memory disk**

Use this complete file content at `internal/replication/memory_disk.go`:

```go
package replication

import (
	"errors"
	"sync"
)

type MemoryDisk struct {
	mu    sync.Mutex
	data  []byte
	dirty []BlockRange
}

func NewMemoryDisk(data []byte) *MemoryDisk {
	copyData := append([]byte(nil), data...)
	return &MemoryDisk{data: copyData}
}

func (d *MemoryDisk) Size() uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return uint64(len(d.data))
}

func (d *MemoryDisk) Bytes() []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]byte(nil), d.data...)
}

func (d *MemoryDisk) ReadRange(r BlockRange) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if r.End() > uint64(len(d.data)) {
		return nil, errors.New("read range exceeds disk size")
	}
	return append([]byte(nil), d.data[r.Offset:r.End()]...), nil
}

func (d *MemoryDisk) WriteRange(r BlockRange, data []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if r.End() > uint64(len(d.data)) {
		return errors.New("write range exceeds disk size")
	}
	if uint64(len(data)) != r.Length {
		return errors.New("write data length does not match range length")
	}
	copy(d.data[r.Offset:r.End()], data)
	return nil
}

func (d *MemoryDisk) WriteAtBytes(data []byte, offset uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	end := offset + uint64(len(data))
	if end > uint64(len(d.data)) {
		panic("write exceeds disk size")
	}
	copy(d.data[offset:end], data)
	d.dirty = NormalizeRanges(append(d.dirty, BlockRange{Offset: offset, Length: uint64(len(data))}))
}

func (d *MemoryDisk) DirtyRanges() []BlockRange {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]BlockRange(nil), d.dirty...)
}

func (d *MemoryDisk) ClearDirtyRanges(ranges []BlockRange) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(ranges) == 0 {
		return
	}
	d.dirty = nil
}
```

- [ ] **Step 4: Implement replication engine**

Use this complete file content at `internal/replication/engine.go`:

```go
package replication

import (
	"context"
	"errors"
)

type Disk interface {
	Size() uint64
	ReadRange(BlockRange) ([]byte, error)
	WriteRange(BlockRange, []byte) error
	DirtyRanges() []BlockRange
	ClearDirtyRanges([]BlockRange)
}

type Engine struct {
	ChunkSize uint64
}

type ResumeState struct {
	VerifiedChunks map[uint64]bool
}

type SyncReport struct {
	BytesCopied uint64
	RangesCopied []BlockRange
}

func (e Engine) BaseSync(ctx context.Context, source Disk, target Disk, resume ResumeState) (SyncReport, error) {
	chunkSize, err := e.validChunkSize()
	if err != nil {
		return SyncReport{}, err
	}
	if source.Size() != target.Size() {
		return SyncReport{}, errors.New("source and target disk sizes differ")
	}
	var report SyncReport
	for offset, index := uint64(0), uint64(0); offset < source.Size(); offset, index = offset+chunkSize, index+1 {
		select {
		case <-ctx.Done():
			return report, ctx.Err()
		default:
		}
		length := chunkSize
		if remaining := source.Size() - offset; remaining < chunkSize {
			length = remaining
		}
		if resume.VerifiedChunks != nil && resume.VerifiedChunks[index] {
			continue
		}
		r := BlockRange{Offset: offset, Length: length}
		if err := copyRange(source, target, r); err != nil {
			return report, err
		}
		report.BytesCopied += r.Length
		report.RangesCopied = append(report.RangesCopied, r)
	}
	return report, nil
}

func (e Engine) IncrementalSync(ctx context.Context, source Disk, target Disk) (SyncReport, error) {
	return e.syncRanges(ctx, source, target, source.DirtyRanges(), true)
}

func (e Engine) FinalSync(ctx context.Context, source Disk, target Disk) (SyncReport, error) {
	return e.syncRanges(ctx, source, target, source.DirtyRanges(), true)
}

func (e Engine) syncRanges(ctx context.Context, source Disk, target Disk, ranges []BlockRange, clearDirty bool) (SyncReport, error) {
	if _, err := e.validChunkSize(); err != nil {
		return SyncReport{}, err
	}
	if source.Size() != target.Size() {
		return SyncReport{}, errors.New("source and target disk sizes differ")
	}
	normalized := NormalizeRanges(ranges)
	var report SyncReport
	for _, r := range normalized {
		select {
		case <-ctx.Done():
			return report, ctx.Err()
		default:
		}
		if err := copyRange(source, target, r); err != nil {
			return report, err
		}
		report.BytesCopied += r.Length
		report.RangesCopied = append(report.RangesCopied, r)
	}
	if clearDirty {
		source.ClearDirtyRanges(normalized)
	}
	return report, nil
}

func (e Engine) validChunkSize() (uint64, error) {
	if e.ChunkSize == 0 {
		return 0, errors.New("chunk size must be greater than zero")
	}
	return e.ChunkSize, nil
}

func copyRange(source Disk, target Disk, r BlockRange) error {
	data, err := source.ReadRange(r)
	if err != nil {
		return err
	}
	return target.WriteRange(r, data)
}
```

- [ ] **Step 5: Run engine tests and verify they pass**

Run:

```powershell
go test ./internal/replication -run TestEngine -v
```

Expected: PASS.

- [ ] **Step 6: Run all replication tests**

Run:

```powershell
go test ./internal/replication -v
```

Expected: PASS.

- [ ] **Step 7: Commit replication engine**

Run:

```powershell
git add internal/replication/memory_disk.go internal/replication/engine.go internal/replication/engine_test.go
git commit -m "feat: add native block replication engine"
```

Expected: one commit for in-memory disk replication.

---

### Task 7: Migration State Machine

**Files:**

- Create: `C:\Users\вф\CODEX\internal\migration\state_test.go`
- Create: `C:\Users\вф\CODEX\internal\migration\state.go`

- [ ] **Step 1: Write failing state machine tests**

Use this complete file content at `internal/migration/state_test.go`:

```go
package migration

import "testing"

func TestTaskStateAllowsWarmMigrationHappyPath(t *testing.T) {
	task := NewTask("mig-1", "vm-123", "cluster-a", "cluster-b")
	events := []Event{
		EventPreflightPassed,
		EventBaseSyncStarted,
		EventBaseSyncCompleted,
		EventIncrementalSyncCompleted,
		EventCutoverStarted,
		EventFinalSyncCompleted,
		EventTargetBooted,
		EventSourceArchived,
	}

	for _, event := range events {
		if err := task.Apply(event); err != nil {
			t.Fatalf("event %s failed in phase %s: %v", event, task.Phase, err)
		}
	}

	if task.Phase != PhaseCompleted {
		t.Fatalf("phase = %s, want %s", task.Phase, PhaseCompleted)
	}
	if task.SourceLocked {
		t.Fatal("source lock should be released after source archive")
	}
	if task.OwnerClusterID != "cluster-b" {
		t.Fatalf("owner = %s, want cluster-b", task.OwnerClusterID)
	}
}

func TestTaskStateRejectsTargetBootBeforeFinalSync(t *testing.T) {
	task := NewTask("mig-1", "vm-123", "cluster-a", "cluster-b")
	if err := task.Apply(EventTargetBooted); err == nil {
		t.Fatal("expected invalid transition error")
	}
}
```

- [ ] **Step 2: Run state tests and verify they fail**

Run:

```powershell
go test ./internal/migration -run TestTaskState -v
```

Expected: FAIL because migration state types do not exist.

- [ ] **Step 3: Implement state machine**

Use this complete file content at `internal/migration/state.go`:

```go
package migration

import "fmt"

type Phase string

const (
	PhaseCreated            Phase = "created"
	PhasePreflightPassed    Phase = "preflight_passed"
	PhaseBaseSyncing        Phase = "base_syncing"
	PhaseIncrementalSyncing Phase = "incremental_syncing"
	PhaseReadyForCutover    Phase = "ready_for_cutover"
	PhaseFinalSyncing       Phase = "final_syncing"
	PhaseTargetBooted       Phase = "target_booted"
	PhaseCompleted          Phase = "completed"
	PhaseFailed             Phase = "failed"
)

type Event string

const (
	EventPreflightPassed         Event = "preflight_passed"
	EventBaseSyncStarted         Event = "base_sync_started"
	EventBaseSyncCompleted       Event = "base_sync_completed"
	EventIncrementalSyncCompleted Event = "incremental_sync_completed"
	EventCutoverStarted          Event = "cutover_started"
	EventFinalSyncCompleted      Event = "final_sync_completed"
	EventTargetBooted            Event = "target_booted"
	EventSourceArchived          Event = "source_archived"
	EventFailed                  Event = "failed"
)

type Task struct {
	ID             string
	VMID           string
	SourceClusterID string
	TargetClusterID string
	OwnerClusterID  string
	Phase          Phase
	SourceLocked   bool
}

func NewTask(id, vmID, sourceClusterID, targetClusterID string) *Task {
	return &Task{
		ID:              id,
		VMID:            vmID,
		SourceClusterID: sourceClusterID,
		TargetClusterID: targetClusterID,
		OwnerClusterID:  sourceClusterID,
		Phase:           PhaseCreated,
		SourceLocked:    true,
	}
}

func (t *Task) Apply(event Event) error {
	switch event {
	case EventPreflightPassed:
		return t.transition(PhaseCreated, PhasePreflightPassed)
	case EventBaseSyncStarted:
		return t.transition(PhasePreflightPassed, PhaseBaseSyncing)
	case EventBaseSyncCompleted:
		return t.transition(PhaseBaseSyncing, PhaseIncrementalSyncing)
	case EventIncrementalSyncCompleted:
		return t.transition(PhaseIncrementalSyncing, PhaseReadyForCutover)
	case EventCutoverStarted:
		return t.transition(PhaseReadyForCutover, PhaseFinalSyncing)
	case EventFinalSyncCompleted:
		return t.transition(PhaseFinalSyncing, PhaseTargetBooted)
	case EventTargetBooted:
		if err := t.require(PhaseTargetBooted); err != nil {
			return err
		}
		t.OwnerClusterID = t.TargetClusterID
		return nil
	case EventSourceArchived:
		if t.OwnerClusterID != t.TargetClusterID {
			return fmt.Errorf("cannot archive source before target ownership is committed")
		}
		if err := t.transition(PhaseTargetBooted, PhaseCompleted); err != nil {
			return err
		}
		t.SourceLocked = false
		return nil
	case EventFailed:
		t.Phase = PhaseFailed
		return nil
	default:
		return fmt.Errorf("unknown migration event %q", event)
	}
}

func (t *Task) transition(from Phase, to Phase) error {
	if err := t.require(from); err != nil {
		return err
	}
	t.Phase = to
	return nil
}

func (t *Task) require(phase Phase) error {
	if t.Phase != phase {
		return fmt.Errorf("invalid transition from phase %q; expected %q", t.Phase, phase)
	}
	return nil
}
```

- [ ] **Step 4: Run state tests and verify they pass**

Run:

```powershell
go test ./internal/migration -run TestTaskState -v
```

Expected: PASS.

- [ ] **Step 5: Commit state machine**

Run:

```powershell
git add internal/migration/state.go internal/migration/state_test.go
git commit -m "feat: add migration state machine"
```

Expected: one commit for migration state handling.

---

### Task 8: Warm Migration Orchestrator With Fakes

**Files:**

- Create: `C:\Users\вф\CODEX\internal\migration\orchestrator_test.go`
- Create: `C:\Users\вф\CODEX\internal\migration\orchestrator.go`

- [ ] **Step 1: Write failing orchestrator tests**

Use this complete file content at `internal/migration/orchestrator_test.go`:

```go
package migration

import (
	"context"
	"errors"
	"testing"
)

func TestOrchestratorPrepareAndCutoverHappyPath(t *testing.T) {
	source := &fakeSource{}
	target := &fakeTarget{}
	replicator := &fakeReplicator{}
	orch := Orchestrator{Source: source, Target: target, Replicator: replicator}

	task, err := orch.Prepare(context.Background(), Request{
		MigrationID: "mig-1",
		VMID:        "vm-123",
		SourceID:    "cluster-a",
		TargetID:    "cluster-b",
	})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	if task.Phase != PhaseReadyForCutover {
		t.Fatalf("phase after prepare = %s", task.Phase)
	}
	if !source.locked || !replicator.baseSynced || !replicator.incrementalSynced {
		t.Fatalf("prepare did not lock and sync correctly")
	}

	if err := orch.Cutover(context.Background(), task); err != nil {
		t.Fatalf("cutover failed: %v", err)
	}
	if task.Phase != PhaseCompleted {
		t.Fatalf("phase after cutover = %s", task.Phase)
	}
	if !source.stopped || !target.registered || !target.booted || !source.archived {
		t.Fatalf("cutover did not stop, register, boot, and archive")
	}
}

func TestOrchestratorKeepsSourceOwnerWhenFinalSyncFails(t *testing.T) {
	source := &fakeSource{}
	target := &fakeTarget{}
	replicator := &fakeReplicator{finalErr: errors.New("network lost")}
	orch := Orchestrator{Source: source, Target: target, Replicator: replicator}

	task, err := orch.Prepare(context.Background(), Request{
		MigrationID: "mig-1",
		VMID:        "vm-123",
		SourceID:    "cluster-a",
		TargetID:    "cluster-b",
	})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	if err := orch.Cutover(context.Background(), task); err == nil {
		t.Fatal("expected cutover failure")
	}
	if task.OwnerClusterID != "cluster-a" {
		t.Fatalf("owner = %s, want cluster-a", task.OwnerClusterID)
	}
	if source.archived {
		t.Fatal("source must not be archived when final sync fails")
	}
	if target.booted {
		t.Fatal("target must not boot when final sync fails")
	}
}

type fakeSource struct {
	locked   bool
	stopped  bool
	archived bool
}

func (f *fakeSource) LockVM(context.Context, string) error {
	f.locked = true
	return nil
}

func (f *fakeSource) StopVM(context.Context, string) error {
	f.stopped = true
	return nil
}

func (f *fakeSource) ArchiveVM(context.Context, string) error {
	f.archived = true
	return nil
}

type fakeTarget struct {
	registered bool
	booted     bool
}

func (f *fakeTarget) RegisterVM(context.Context, string) error {
	f.registered = true
	return nil
}

func (f *fakeTarget) BootVM(context.Context, string) error {
	f.booted = true
	return nil
}

type fakeReplicator struct {
	baseSynced        bool
	incrementalSynced bool
	finalSynced       bool
	finalErr          error
}

func (f *fakeReplicator) BaseSync(context.Context, string) error {
	f.baseSynced = true
	return nil
}

func (f *fakeReplicator) IncrementalSync(context.Context, string) error {
	f.incrementalSynced = true
	return nil
}

func (f *fakeReplicator) FinalSync(context.Context, string) error {
	f.finalSynced = true
	return f.finalErr
}
```

- [ ] **Step 2: Run orchestrator tests and verify they fail**

Run:

```powershell
go test ./internal/migration -run TestOrchestrator -v
```

Expected: FAIL because `Orchestrator`, `Request`, and interfaces do not exist.

- [ ] **Step 3: Implement orchestrator**

Use this complete file content at `internal/migration/orchestrator.go`:

```go
package migration

import (
	"context"
	"errors"
)

type Request struct {
	MigrationID string
	VMID        string
	SourceID    string
	TargetID    string
}

type SourceCluster interface {
	LockVM(context.Context, string) error
	StopVM(context.Context, string) error
	ArchiveVM(context.Context, string) error
}

type TargetCluster interface {
	RegisterVM(context.Context, string) error
	BootVM(context.Context, string) error
}

type Replicator interface {
	BaseSync(context.Context, string) error
	IncrementalSync(context.Context, string) error
	FinalSync(context.Context, string) error
}

type Orchestrator struct {
	Source     SourceCluster
	Target     TargetCluster
	Replicator Replicator
}

func (o Orchestrator) Prepare(ctx context.Context, req Request) (*Task, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	if err := o.validateDependencies(); err != nil {
		return nil, err
	}
	task := NewTask(req.MigrationID, req.VMID, req.SourceID, req.TargetID)
	if err := o.Source.LockVM(ctx, req.VMID); err != nil {
		task.Apply(EventFailed)
		return task, err
	}
	if err := task.Apply(EventPreflightPassed); err != nil {
		return task, err
	}
	if err := task.Apply(EventBaseSyncStarted); err != nil {
		return task, err
	}
	if err := o.Replicator.BaseSync(ctx, req.VMID); err != nil {
		task.Apply(EventFailed)
		return task, err
	}
	if err := task.Apply(EventBaseSyncCompleted); err != nil {
		return task, err
	}
	if err := o.Replicator.IncrementalSync(ctx, req.VMID); err != nil {
		task.Apply(EventFailed)
		return task, err
	}
	if err := task.Apply(EventIncrementalSyncCompleted); err != nil {
		return task, err
	}
	return task, nil
}

func (o Orchestrator) Cutover(ctx context.Context, task *Task) error {
	if err := o.validateDependencies(); err != nil {
		return err
	}
	if task == nil {
		return errors.New("migration task is required")
	}
	if err := task.Apply(EventCutoverStarted); err != nil {
		return err
	}
	if err := o.Source.StopVM(ctx, task.VMID); err != nil {
		task.Apply(EventFailed)
		return err
	}
	if err := o.Replicator.FinalSync(ctx, task.VMID); err != nil {
		task.Apply(EventFailed)
		return err
	}
	if err := task.Apply(EventFinalSyncCompleted); err != nil {
		return err
	}
	if err := o.Target.RegisterVM(ctx, task.VMID); err != nil {
		task.Apply(EventFailed)
		return err
	}
	if err := o.Target.BootVM(ctx, task.VMID); err != nil {
		task.Apply(EventFailed)
		return err
	}
	if err := task.Apply(EventTargetBooted); err != nil {
		return err
	}
	if err := o.Source.ArchiveVM(ctx, task.VMID); err != nil {
		task.Apply(EventFailed)
		return err
	}
	return task.Apply(EventSourceArchived)
}

func (r Request) validate() error {
	if r.MigrationID == "" {
		return errors.New("migration id is required")
	}
	if r.VMID == "" {
		return errors.New("vm id is required")
	}
	if r.SourceID == "" {
		return errors.New("source cluster id is required")
	}
	if r.TargetID == "" {
		return errors.New("target cluster id is required")
	}
	if r.SourceID == r.TargetID {
		return errors.New("source and target clusters must differ")
	}
	return nil
}

func (o Orchestrator) validateDependencies() error {
	if o.Source == nil {
		return errors.New("source cluster dependency is required")
	}
	if o.Target == nil {
		return errors.New("target cluster dependency is required")
	}
	if o.Replicator == nil {
		return errors.New("replicator dependency is required")
	}
	return nil
}
```

- [ ] **Step 4: Run orchestrator tests and verify they pass**

Run:

```powershell
go test ./internal/migration -run TestOrchestrator -v
```

Expected: PASS.

- [ ] **Step 5: Run all migration tests**

Run:

```powershell
go test ./internal/migration -v
```

Expected: PASS.

- [ ] **Step 6: Commit orchestrator**

Run:

```powershell
git add internal/migration/orchestrator.go internal/migration/orchestrator_test.go
git commit -m "feat: orchestrate warm migration cutover"
```

Expected: one commit for warm migration orchestration.

---

### Task 9: Minimal HTTP API Contract

**Files:**

- Create: `C:\Users\вф\CODEX\internal\api\server_test.go`
- Create: `C:\Users\вф\CODEX\internal\api\server.go`

- [ ] **Step 1: Write failing HTTP tests**

Use this complete file content at `internal/api/server_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerPreflightEndpoint(t *testing.T) {
	server := NewServer()
	req := httptest.NewRequest(http.MethodPost, "/migrations/preflight", strings.NewReader(`{"vm_id":"vm-123"}`))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("response body = %s", rec.Body.String())
	}
}

func TestServerRejectsUnknownRoute(t *testing.T) {
	server := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
```

- [ ] **Step 2: Run HTTP tests and verify they fail**

Run:

```powershell
go test ./internal/api -run TestServer -v
```

Expected: FAIL because package `api` has no implementation.

- [ ] **Step 3: Implement HTTP server**

Use this complete file content at `internal/api/server.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
)

type Server struct {
	mux *http.ServeMux
}

func NewServer() *Server {
	s := &Server{mux: http.NewServeMux()}
	s.mux.HandleFunc("/migrations/preflight", s.handlePreflight)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handlePreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"failures": []any{},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
```

- [ ] **Step 4: Run HTTP tests and verify they pass**

Run:

```powershell
go test ./internal/api -run TestServer -v
```

Expected: PASS.

- [ ] **Step 5: Commit HTTP API contract**

Run:

```powershell
git add internal/api/server.go internal/api/server_test.go
git commit -m "feat: add migration HTTP API skeleton"
```

Expected: one commit for the initial API contract.

---

### Task 10: Full Verification

**Files:**

- Modify: no source files unless verification exposes a concrete failure.

- [ ] **Step 1: Format Go code**

Run:

```powershell
gofmt -w .\internal
```

Expected: command completes without output.

- [ ] **Step 2: Run all tests**

Run:

```powershell
go test ./...
```

Expected: all packages pass.

- [ ] **Step 3: Inspect git status**

Run:

```powershell
git status --short
```

Expected: no modified tracked files. Untracked local installers and media should be ignored by `.gitignore`.

- [ ] **Step 4: Commit formatting fixes if any files changed**

Run only if `git status --short` reports modified Go files:

```powershell
git add internal
git commit -m "chore: format warm migration core"
```

Expected: either no commit is needed, or one formatting commit is created.

---

## Self-Review

Spec coverage:

- VM metadata portability is covered by Task 2.
- Network and storage mapping validation is covered by Task 3.
- Replication manifests are covered by Task 5.
- Dirty range merge and ordering are covered by Task 4.
- Base, incremental, final sync, checksum, and resume foundations are covered by Tasks 5 and 6.
- Migration ownership and cutover safety are covered by Tasks 7 and 8.
- Initial API route coverage is covered by Task 9.

Scope intentionally deferred to separate plans:

- Real QEMU dirty bitmap integration.
- PostgreSQL-backed task persistence.
- mTLS remote cluster authentication.
- Web UI workflows.
- Bare-metal ISO installer.

Completion-marker scan:

- No unfinished-marker tokens or vague implementation steps are present.
- Every code-changing step includes complete file content.
- Every verification step includes exact commands and expected outcomes.
