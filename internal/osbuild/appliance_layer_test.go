package osbuild

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const debian135SHA256 = "95838884F5EA6C82421DFE6BAAA5A639DBBE6756C1E380F9FE7A7CB0C1949D2A"

func TestDebianBaseManifestPinsLocalNetinstISO(t *testing.T) {
	manifest := readFile(t, "os", "debian", "base-iso.json")

	var value struct {
		Name     string `json:"name"`
		Path     string `json:"path"`
		SHA256   string `json:"sha256"`
		Variant  string `json:"variant"`
		Arch     string `json:"arch"`
		Use      string `json:"use"`
		Verified bool   `json:"verified_locally"`
	}
	if err := json.Unmarshal([]byte(manifest), &value); err != nil {
		t.Fatalf("manifest JSON is invalid: %v", err)
	}

	if value.Name != "debian-13.5.0-amd64-netinst.iso" {
		t.Fatalf("manifest name = %q", value.Name)
	}
	if value.Path != "../../OS/debian-13.5.0-amd64-netinst.iso" {
		t.Fatalf("manifest path = %q", value.Path)
	}
	if value.SHA256 != debian135SHA256 {
		t.Fatalf("manifest sha256 = %q", value.SHA256)
	}
	if value.Variant != "netinst" || value.Arch != "amd64" || !value.Verified {
		t.Fatalf("manifest metadata = %+v", value)
	}
	if !strings.Contains(value.Use, "base ISO") {
		t.Fatalf("manifest use = %q", value.Use)
	}
}

func TestPreseedConfigInstallsVirtualizationAppliance(t *testing.T) {
	preseed := readFile(t, "os", "debian", "preseed.cfg")

	required := []string{
		"d-i debian-installer/locale string en_US.UTF-8",
		"d-i netcfg/get_hostname string virt-node",
		"d-i passwd/root-login boolean false",
		"d-i partman-auto/method string regular",
		"d-i pkgsel/include string openssh-server sudo curl ca-certificates git qemu-system-x86 libvirt-daemon-system libvirt-clients virtinst bridge-utils ovmf cloud-image-utils",
		"PasswordAuthentication no",
		"in-target chage -d 0 virtadmin",
		"in-target /opt/virt-appliance/post-install.sh",
	}
	for _, want := range required {
		if !strings.Contains(preseed, want) {
			t.Fatalf("preseed missing %q", want)
		}
	}
}

func TestPostInstallScriptBootstrapsNodeService(t *testing.T) {
	script := readFile(t, "os", "debian", "post-install.sh")

	required := []string{
		"set -euo pipefail",
		"systemctl enable libvirtd.service",
		"systemctl enable warm-migrationd.service",
		"install-node.sh",
		"virtctl health",
		"usermod -aG libvirt,kvm",
	}
	for _, want := range required {
		if !strings.Contains(script, want) {
			t.Fatalf("post-install missing %q", want)
		}
	}
}

func TestApplianceREADMEDocumentsFirstNodeFlow(t *testing.T) {
	doc := readFile(t, "os", "debian", "README.md")

	required := []string{
		"debian-13.5.0-amd64-netinst.iso",
		debian135SHA256,
		"preseed.cfg",
		"post-install.sh",
		"warm-migrationd",
		"virtctl health",
	}
	for _, want := range required {
		if !strings.Contains(doc, want) {
			t.Fatalf("README missing %q", want)
		}
	}
}

func readFile(t *testing.T, parts ...string) string {
	t.Helper()
	fullPath := filepath.Join(append([]string{repoRoot(t)}, parts...)...)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("read %s: %v", fullPath, err)
	}
	return string(content)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
