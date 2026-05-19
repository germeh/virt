package host

import (
	"context"
	"testing"
)

func TestDetectCapabilitiesReportsReadyKVMHost(t *testing.T) {
	inspector := fakeInspector{
		files: map[string]bool{
			"/dev/kvm":                     true,
			"/usr/share/OVMF/OVMF_CODE.fd": true,
		},
		commands: map[string]bool{
			"qemu-system-x86_64": true,
			"virsh":              true,
			"virt-install":       true,
		},
		services: map[string]bool{
			"libvirtd.service": true,
		},
	}

	caps := DetectCapabilities(context.Background(), inspector)

	if !caps.Ready {
		t.Fatalf("Ready = false, missing = %v", caps.Missing)
	}
	if !caps.KVMDevice || !caps.QEMU || !caps.Libvirt || !caps.VirtInstall || !caps.OVMF || !caps.LibvirtActive {
		t.Fatalf("capabilities = %+v", caps)
	}
	if len(caps.Missing) != 0 {
		t.Fatalf("Missing = %v", caps.Missing)
	}
}

func TestDetectCapabilitiesListsMissingRequirements(t *testing.T) {
	inspector := fakeInspector{
		files:    map[string]bool{},
		commands: map[string]bool{"virsh": true},
		services: map[string]bool{
			"libvirtd.service": false,
		},
	}

	caps := DetectCapabilities(context.Background(), inspector)

	if caps.Ready {
		t.Fatalf("Ready = true")
	}
	assertContains(t, caps.Missing, "/dev/kvm")
	assertContains(t, caps.Missing, "qemu-system-x86_64")
	assertContains(t, caps.Missing, "virt-install")
	assertContains(t, caps.Missing, "OVMF firmware")
	assertContains(t, caps.Missing, "libvirtd.service active")
}

type fakeInspector struct {
	files    map[string]bool
	commands map[string]bool
	services map[string]bool
}

func (f fakeInspector) FileExists(path string) bool {
	return f.files[path]
}

func (f fakeInspector) LookPath(name string) bool {
	return f.commands[name]
}

func (f fakeInspector) ServiceActive(_ context.Context, name string) bool {
	return f.services[name]
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("%q not in %v", want, values)
}
