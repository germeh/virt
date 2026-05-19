package migration

import (
	"context"
	"errors"
	"testing"
)

func TestOrchestratorPrepareAndCutoverHappyPath(t *testing.T) {
	source := &fakeSource{}
	target := &fakeTarget{}
	replicator := &fakeReplicator{}
	orch := Orchestrator{Source: source, Target: target, Replicator: replicator}

	task, err := orch.Prepare(context.Background(), Request{
		MigrationID: "mig-1",
		VMID:        "vm-123",
		SourceID:    "cluster-a",
		TargetID:    "cluster-b",
	})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	if task.Phase != PhaseReadyForCutover {
		t.Fatalf("phase after prepare = %s", task.Phase)
	}
	if !source.locked || !replicator.baseSynced || !replicator.incrementalSynced {
		t.Fatalf("prepare did not lock and sync correctly")
	}

	if err := orch.Cutover(context.Background(), task); err != nil {
		t.Fatalf("cutover failed: %v", err)
	}
	if task.Phase != PhaseCompleted {
		t.Fatalf("phase after cutover = %s", task.Phase)
	}
	if !source.stopped || !target.registered || !target.booted || !source.archived {
		t.Fatalf("cutover did not stop, register, boot, and archive")
	}
}

func TestOrchestratorKeepsSourceOwnerWhenFinalSyncFails(t *testing.T) {
	source := &fakeSource{}
	target := &fakeTarget{}
	replicator := &fakeReplicator{finalErr: errors.New("network lost")}
	orch := Orchestrator{Source: source, Target: target, Replicator: replicator}

	task, err := orch.Prepare(context.Background(), Request{
		MigrationID: "mig-1",
		VMID:        "vm-123",
		SourceID:    "cluster-a",
		TargetID:    "cluster-b",
	})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	if err := orch.Cutover(context.Background(), task); err == nil {
		t.Fatal("expected cutover failure")
	}
	if task.OwnerClusterID != "cluster-a" {
		t.Fatalf("owner = %s, want cluster-a", task.OwnerClusterID)
	}
	if source.archived {
		t.Fatal("source must not be archived when final sync fails")
	}
	if target.booted {
		t.Fatal("target must not boot when final sync fails")
	}
}

func TestOrchestratorDoesNotMarkSourceLockedWhenLockFails(t *testing.T) {
	lockErr := errors.New("lock failed")
	source := &fakeSource{lockErr: lockErr}
	orch := Orchestrator{Source: source, Target: &fakeTarget{}, Replicator: &fakeReplicator{}}

	task, err := orch.Prepare(context.Background(), Request{
		MigrationID: "mig-1",
		VMID:        "vm-123",
		SourceID:    "cluster-a",
		TargetID:    "cluster-b",
	})
	if !errors.Is(err, lockErr) {
		t.Fatalf("error = %v, want %v", err, lockErr)
	}
	if task == nil {
		t.Fatal("expected failed task")
	}
	if task.SourceLocked {
		t.Fatal("source lock flag should be false when LockVM fails")
	}
	if task.Phase != PhaseFailed {
		t.Fatalf("phase = %s, want %s", task.Phase, PhaseFailed)
	}
}

func TestOrchestratorPrepareRejectsInvalidRequest(t *testing.T) {
	orch := Orchestrator{Source: &fakeSource{}, Target: &fakeTarget{}, Replicator: &fakeReplicator{}}

	if task, err := orch.Prepare(context.Background(), Request{
		MigrationID: "mig-1",
		VMID:        "vm-123",
		SourceID:    "cluster-a",
		TargetID:    "cluster-a",
	}); err == nil || task != nil {
		t.Fatalf("Prepare = (%#v, %v), want nil task and error", task, err)
	}
}

func TestOrchestratorCutoverRejectsNilTask(t *testing.T) {
	orch := Orchestrator{Source: &fakeSource{}, Target: &fakeTarget{}, Replicator: &fakeReplicator{}}

	if err := orch.Cutover(context.Background(), nil); err == nil {
		t.Fatal("expected nil task error")
	}
}

func TestOrchestratorRejectsMissingDependencies(t *testing.T) {
	if _, err := (Orchestrator{}).Prepare(context.Background(), Request{
		MigrationID: "mig-1",
		VMID:        "vm-123",
		SourceID:    "cluster-a",
		TargetID:    "cluster-b",
	}); err == nil {
		t.Fatal("expected missing dependency error")
	}
}

type fakeSource struct {
	locked   bool
	stopped  bool
	archived bool
	lockErr  error
}

func (f *fakeSource) LockVM(context.Context, string) error {
	if f.lockErr != nil {
		return f.lockErr
	}
	f.locked = true
	return nil
}

func (f *fakeSource) StopVM(context.Context, string) error {
	f.stopped = true
	return nil
}

func (f *fakeSource) ArchiveVM(context.Context, string) error {
	f.archived = true
	return nil
}

type fakeTarget struct {
	registered bool
	booted     bool
}

func (f *fakeTarget) RegisterVM(context.Context, string) error {
	f.registered = true
	return nil
}

func (f *fakeTarget) BootVM(context.Context, string) error {
	f.booted = true
	return nil
}

type fakeReplicator struct {
	baseSynced        bool
	incrementalSynced bool
	finalSynced       bool
	finalErr          error
}

func (f *fakeReplicator) BaseSync(context.Context, string) error {
	f.baseSynced = true
	return nil
}

func (f *fakeReplicator) IncrementalSync(context.Context, string) error {
	f.incrementalSynced = true
	return nil
}

func (f *fakeReplicator) FinalSync(context.Context, string) error {
	f.finalSynced = true
	return f.finalErr
}
