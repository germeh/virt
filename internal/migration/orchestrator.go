package migration

import (
	"context"
	"errors"
)

type Request struct {
	MigrationID string
	VMID        string
	SourceID    string
	TargetID    string
}

type SourceCluster interface {
	LockVM(context.Context, string) error
	StopVM(context.Context, string) error
	ArchiveVM(context.Context, string) error
}

type TargetCluster interface {
	RegisterVM(context.Context, string) error
	BootVM(context.Context, string) error
}

type Replicator interface {
	BaseSync(context.Context, string) error
	IncrementalSync(context.Context, string) error
	FinalSync(context.Context, string) error
}

type Orchestrator struct {
	Source     SourceCluster
	Target     TargetCluster
	Replicator Replicator
}

func (o Orchestrator) Prepare(ctx context.Context, req Request) (*Task, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	if err := o.validateDependencies(); err != nil {
		return nil, err
	}
	task := NewTask(req.MigrationID, req.VMID, req.SourceID, req.TargetID)
	if err := o.Source.LockVM(ctx, req.VMID); err != nil {
		task.Apply(EventFailed)
		return task, err
	}
	if err := task.Apply(EventPreflightPassed); err != nil {
		return task, err
	}
	if err := task.Apply(EventBaseSyncStarted); err != nil {
		return task, err
	}
	if err := o.Replicator.BaseSync(ctx, req.VMID); err != nil {
		task.Apply(EventFailed)
		return task, err
	}
	if err := task.Apply(EventBaseSyncCompleted); err != nil {
		return task, err
	}
	if err := o.Replicator.IncrementalSync(ctx, req.VMID); err != nil {
		task.Apply(EventFailed)
		return task, err
	}
	if err := task.Apply(EventIncrementalSyncCompleted); err != nil {
		return task, err
	}
	return task, nil
}

func (o Orchestrator) Cutover(ctx context.Context, task *Task) error {
	if err := o.validateDependencies(); err != nil {
		return err
	}
	if task == nil {
		return errors.New("migration task is required")
	}
	if err := task.Apply(EventCutoverStarted); err != nil {
		return err
	}
	if err := o.Source.StopVM(ctx, task.VMID); err != nil {
		task.Apply(EventFailed)
		return err
	}
	if err := o.Replicator.FinalSync(ctx, task.VMID); err != nil {
		task.Apply(EventFailed)
		return err
	}
	if err := task.Apply(EventFinalSyncCompleted); err != nil {
		return err
	}
	if err := o.Target.RegisterVM(ctx, task.VMID); err != nil {
		task.Apply(EventFailed)
		return err
	}
	if err := o.Target.BootVM(ctx, task.VMID); err != nil {
		task.Apply(EventFailed)
		return err
	}
	if err := task.Apply(EventTargetBooted); err != nil {
		return err
	}
	if err := o.Source.ArchiveVM(ctx, task.VMID); err != nil {
		task.Apply(EventFailed)
		return err
	}
	return task.Apply(EventSourceArchived)
}

func (r Request) validate() error {
	if r.MigrationID == "" {
		return errors.New("migration id is required")
	}
	if r.VMID == "" {
		return errors.New("vm id is required")
	}
	if r.SourceID == "" {
		return errors.New("source cluster id is required")
	}
	if r.TargetID == "" {
		return errors.New("target cluster id is required")
	}
	if r.SourceID == r.TargetID {
		return errors.New("source and target clusters must differ")
	}
	return nil
}

func (o Orchestrator) validateDependencies() error {
	if o.Source == nil {
		return errors.New("source cluster dependency is required")
	}
	if o.Target == nil {
		return errors.New("target cluster dependency is required")
	}
	if o.Replicator == nil {
		return errors.New("replicator dependency is required")
	}
	return nil
}
