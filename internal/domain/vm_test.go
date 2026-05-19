package domain

import (
	"strings"
	"testing"
)

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

	requireErrorContains(t, vm.Validate(), "vm id is required", "vm name is required")
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

	requireErrorContains(t, vm.Validate(), `disk disk-1 has unsupported format "vmdk"`)
}

func TestVMValidateRejectsInvalidNIC(t *testing.T) {
	vm := VM{
		ID:          "vm-123",
		Name:        "billing-db",
		CPUProfile:  "x86-64-v2",
		VCPU:        2,
		MemoryBytes: 2 * GiB,
		Firmware:    FirmwareUEFI,
		MachineType: "q35",
		Disks:       []Disk{{ID: "disk-1", Format: DiskFormatQCOW2, VirtualSizeBytes: GiB, Bus: "virtio", StorageClass: "fast-local"}},
		NICs:        []NIC{{ID: "nic-1", SourceNetwork: "prod-net"}},
	}

	requireErrorContains(t, vm.Validate(), "nic nic-1 mac is required")
}

func requireErrorContains(t *testing.T, err error, parts ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validation error containing %q", parts)
	}
	for _, part := range parts {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("expected validation error %q to contain %q", err.Error(), part)
		}
	}
}
