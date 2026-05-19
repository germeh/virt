package host

import "context"

type Capabilities struct {
	Ready         bool     `json:"ready"`
	KVMDevice     bool     `json:"kvm_device"`
	QEMU          bool     `json:"qemu"`
	Libvirt       bool     `json:"libvirt"`
	VirtInstall   bool     `json:"virt_install"`
	OVMF          bool     `json:"ovmf"`
	LibvirtActive bool     `json:"libvirt_active"`
	Missing       []string `json:"missing"`
}

type Inspector interface {
	FileExists(path string) bool
	LookPath(name string) bool
	ServiceActive(ctx context.Context, name string) bool
}

func DetectCapabilities(ctx context.Context, inspector Inspector) Capabilities {
	caps := Capabilities{
		KVMDevice:     inspector.FileExists("/dev/kvm"),
		QEMU:          inspector.LookPath("qemu-system-x86_64"),
		Libvirt:       inspector.LookPath("virsh"),
		VirtInstall:   inspector.LookPath("virt-install"),
		OVMF:          inspector.FileExists("/usr/share/OVMF/OVMF_CODE.fd") || inspector.FileExists("/usr/share/ovmf/OVMF.fd"),
		LibvirtActive: inspector.ServiceActive(ctx, "libvirtd.service"),
	}

	if !caps.KVMDevice {
		caps.Missing = append(caps.Missing, "/dev/kvm")
	}
	if !caps.QEMU {
		caps.Missing = append(caps.Missing, "qemu-system-x86_64")
	}
	if !caps.Libvirt {
		caps.Missing = append(caps.Missing, "virsh")
	}
	if !caps.VirtInstall {
		caps.Missing = append(caps.Missing, "virt-install")
	}
	if !caps.OVMF {
		caps.Missing = append(caps.Missing, "OVMF firmware")
	}
	if !caps.LibvirtActive {
		caps.Missing = append(caps.Missing, "libvirtd.service active")
	}
	caps.Ready = len(caps.Missing) == 0
	return caps
}
