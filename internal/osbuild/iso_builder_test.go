package osbuild

import (
	"strings"
	"testing"
)

func TestISOBuilderScriptInjectsApplianceFilesAndBootEntries(t *testing.T) {
	script := readFile(t, "scripts", "build-virt-node-iso.sh")

	required := []string{
		"set -euo pipefail",
		"base-iso.json",
		"sha256sum",
		"expected_hash",
		"xorriso",
		"-boot_image any replay",
		"-map \"${PRESEED_FILE}\" /preseed.cfg",
		"-map \"${POST_INSTALL_FILE}\" /virt/post-install.sh",
		"label virt-auto",
		"Install Virt node (automated, erases disk)",
		"preseed/file=/cdrom/preseed.cfg",
		"boot/grub/grub.cfg",
		"isolinux/txt.cfg",
	}
	for _, want := range required {
		if !strings.Contains(script, want) {
			t.Fatalf("builder script missing %q", want)
		}
	}
}

func TestISOBuilderScriptDocumentsLinuxRequirements(t *testing.T) {
	doc := readFile(t, "docs", "build-virt-node-iso.md")

	required := []string{
		"scripts/build-virt-node-iso.sh",
		"debian-13.5.0-amd64-netinst.iso",
		"xorriso",
		"python3",
		"sha256sum",
		"dist/virt-node.iso",
		"Automated install erases the target disk",
	}
	for _, want := range required {
		if !strings.Contains(doc, want) {
			t.Fatalf("ISO builder doc missing %q", want)
		}
	}
}
