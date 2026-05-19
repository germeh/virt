package host

import (
	"context"
	"reflect"
	"testing"
)

func TestParseVirshList(t *testing.T) {
	output := ` Id   Name          State
----------------------------------
 1    web-01        running
 -    db-01         shut off
 7    worker-01     paused
`

	vms, err := ParseVirshList(output)
	if err != nil {
		t.Fatalf("ParseVirshList() error = %v", err)
	}

	want := []VM{
		{ID: "1", Name: "web-01", State: "running"},
		{ID: "", Name: "db-01", State: "shut off"},
		{ID: "7", Name: "worker-01", State: "paused"},
	}
	if !reflect.DeepEqual(vms, want) {
		t.Fatalf("VMs = %+v, want %+v", vms, want)
	}
}

func TestVirshDriverListsVMs(t *testing.T) {
	runner := &fakeRunner{
		output: ` Id   Name          State
----------------------------------
 2    test-vm       running
`,
	}
	driver := VirshDriver{Runner: runner}

	vms, err := driver.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs() error = %v", err)
	}

	if len(vms) != 1 || vms[0].Name != "test-vm" || vms[0].State != "running" {
		t.Fatalf("VMs = %+v", vms)
	}
	if !reflect.DeepEqual(runner.calls[0], []string{"virsh", "--connect", "qemu:///system", "list", "--all"}) {
		t.Fatalf("calls = %+v", runner.calls)
	}
}

type fakeRunner struct {
	output string
	calls  [][]string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	call := append([]string{name}, args...)
	f.calls = append(f.calls, call)
	return f.output, nil
}
