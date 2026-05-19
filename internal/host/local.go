package host

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

type LocalInspector struct{}

func (LocalInspector) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (LocalInspector) LookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (LocalInspector) ServiceActive(ctx context.Context, name string) bool {
	output, err := exec.CommandContext(ctx, "systemctl", "is-active", name).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "active"
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(output), err
}
