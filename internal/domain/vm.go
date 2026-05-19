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
		if nic.MAC == "" {
			errs = append(errs, fmt.Errorf("nic %s mac is required", nic.ID))
		}
		if nic.SourceNetwork == "" {
			errs = append(errs, fmt.Errorf("nic %s source network is required", nic.ID))
		}
	}
	return errors.Join(errs...)
}
