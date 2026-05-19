package preflight

import (
	"math"
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

	assertFailureCode(t, result, FailureCodeNetworkMappingMissing)
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

	assertFailureCode(t, result, FailureCodeCPUProfileUnsupported)
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

	assertFailureCode(t, result, FailureCodeStorageCapacityInsufficient)
}

func TestCheckRejectsAggregateStorageCapacity(t *testing.T) {
	vm := sampleVM()
	vm.Disks = []domain.Disk{
		{ID: "disk-1", Format: domain.DiskFormatQCOW2, VirtualSizeBytes: 60 * domain.GiB, Bus: "virtio", StorageClass: "fast-local"},
		{ID: "disk-2", Format: domain.DiskFormatQCOW2, VirtualSizeBytes: 60 * domain.GiB, Bus: "virtio", StorageClass: "archive-local"},
	}
	cluster := sampleCluster()
	storage := cluster.StorageClasses["fast-local-b"]
	storage.AvailableBytes = 100 * domain.GiB
	cluster.StorageClasses["fast-local-b"] = storage

	result := Check(Request{
		VM:          vm,
		Target:      cluster,
		NetworkMap:  map[string]string{"prod-net": "prod-net-b"},
		StorageMap:  map[string]string{"fast-local": "fast-local-b", "archive-local": "fast-local-b"},
		CutoverMode: CutoverWarm,
	})

	assertFailureCode(t, result, FailureCodeStorageCapacityInsufficient)
	assertFailureCodeCount(t, result, FailureCodeStorageCapacityInsufficient, 1)
}

func TestCheckRejectsStorageCapacityOverflow(t *testing.T) {
	vm := sampleVM()
	vm.Disks = []domain.Disk{
		{ID: "disk-1", Format: domain.DiskFormatQCOW2, VirtualSizeBytes: math.MaxUint64 - 10, Bus: "virtio", StorageClass: "fast-local"},
		{ID: "disk-2", Format: domain.DiskFormatQCOW2, VirtualSizeBytes: 20, Bus: "virtio", StorageClass: "archive-local"},
	}

	result := Check(Request{
		VM:          vm,
		Target:      sampleCluster(),
		NetworkMap:  map[string]string{"prod-net": "prod-net-b"},
		StorageMap:  map[string]string{"fast-local": "fast-local-b", "archive-local": "fast-local-b"},
		CutoverMode: CutoverWarm,
	})

	if result.OK {
		t.Fatalf("expected preflight failure, got success")
	}
	assertFailureCode(t, result, FailureCodeStorageCapacityInsufficient)
	assertFailureCodeCount(t, result, FailureCodeStorageCapacityInsufficient, 1)
}

func TestCheckRejectsUnsupportedCutoverMode(t *testing.T) {
	for _, mode := range []CutoverMode{"", "cold"} {
		t.Run(string(mode), func(t *testing.T) {
			result := Check(Request{
				VM:          sampleVM(),
				Target:      sampleCluster(),
				NetworkMap:  map[string]string{"prod-net": "prod-net-b"},
				StorageMap:  map[string]string{"fast-local": "fast-local-b"},
				CutoverMode: mode,
			})

			assertFailureCode(t, result, FailureCodeCutoverModeUnsupported)
		})
	}
}

func TestCheckRejectsWarmMigrationWithoutDirtyTracking(t *testing.T) {
	cluster := sampleCluster()
	storage := cluster.StorageClasses["fast-local-b"]
	storage.SupportsDirtyTrack = false
	cluster.StorageClasses["fast-local-b"] = storage

	result := Check(Request{
		VM:          sampleVM(),
		Target:      cluster,
		NetworkMap:  map[string]string{"prod-net": "prod-net-b"},
		StorageMap:  map[string]string{"fast-local": "fast-local-b"},
		CutoverMode: CutoverWarm,
	})

	assertFailureCode(t, result, FailureCodeStorageDirtyTrackingUnsupported)
}

func TestCheckRejectsMissingStorageMapping(t *testing.T) {
	result := Check(Request{
		VM:          sampleVM(),
		Target:      sampleCluster(),
		NetworkMap:  map[string]string{"prod-net": "prod-net-b"},
		StorageMap:  map[string]string{},
		CutoverMode: CutoverWarm,
	})

	assertFailureCode(t, result, FailureCodeStorageMappingMissing)
}

func TestCheckRejectsMissingStorageTarget(t *testing.T) {
	result := Check(Request{
		VM:          sampleVM(),
		Target:      sampleCluster(),
		NetworkMap:  map[string]string{"prod-net": "prod-net-b"},
		StorageMap:  map[string]string{"fast-local": "missing-storage"},
		CutoverMode: CutoverWarm,
	})

	assertFailureCode(t, result, FailureCodeStorageTargetMissing)
}

func TestCheckRejectsMissingNetworkTarget(t *testing.T) {
	result := Check(Request{
		VM:          sampleVM(),
		Target:      sampleCluster(),
		NetworkMap:  map[string]string{"prod-net": "missing-net"},
		StorageMap:  map[string]string{"fast-local": "fast-local-b"},
		CutoverMode: CutoverWarm,
	})

	assertFailureCode(t, result, FailureCodeNetworkTargetMissing)
}

func TestCheckRejectsInsufficientMemory(t *testing.T) {
	cluster := sampleCluster()
	cluster.AvailableMemoryBytes = domain.GiB

	result := Check(Request{
		VM:          sampleVM(),
		Target:      cluster,
		NetworkMap:  map[string]string{"prod-net": "prod-net-b"},
		StorageMap:  map[string]string{"fast-local": "fast-local-b"},
		CutoverMode: CutoverWarm,
	})

	assertFailureCode(t, result, FailureCodeMemoryCapacityInsufficient)
}

func TestCheckRejectsInvalidVM(t *testing.T) {
	vm := sampleVM()
	vm.NICs[0].MAC = ""

	result := Check(Request{
		VM:          vm,
		Target:      sampleCluster(),
		NetworkMap:  map[string]string{"prod-net": "prod-net-b"},
		StorageMap:  map[string]string{"fast-local": "fast-local-b"},
		CutoverMode: CutoverWarm,
	})

	assertFailureCode(t, result, FailureCodeVMInvalid)
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

func assertFailureCodeCount(t *testing.T, result Result, code string, expected int) {
	t.Helper()
	var actual int
	for _, failure := range result.Failures {
		if failure.Code == code {
			actual++
		}
	}
	if actual != expected {
		t.Fatalf("expected failure code %q %d times, got %d in %#v", code, expected, actual, result.Failures)
	}
}
