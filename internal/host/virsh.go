package host

import (
	"context"
	"fmt"
	"strings"
)

type VM struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type VirshDriver struct {
	Runner CommandRunner
}

func (d VirshDriver) ListVMs(ctx context.Context) ([]VM, error) {
	if d.Runner == nil {
		return nil, fmt.Errorf("virsh runner is required")
	}
	output, err := d.Runner.Run(ctx, "virsh", "--connect", "qemu:///system", "list", "--all")
	if err != nil {
		return nil, err
	}
	return ParseVirshList(output)
}

func ParseVirshList(output string) ([]VM, error) {
	var vms []VM
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Id ") || strings.HasPrefix(line, "---") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, fmt.Errorf("invalid virsh list line: %q", line)
		}
		id := fields[0]
		if id == "-" {
			id = ""
		}
		vms = append(vms, VM{
			ID:    id,
			Name:  fields[1],
			State: strings.Join(fields[2:], " "),
		})
	}
	return vms, nil
}
