package migration

import "fmt"

type Phase string

const (
	PhaseCreated            Phase = "created"
	PhasePreflightPassed    Phase = "preflight_passed"
	PhaseBaseSyncing        Phase = "base_syncing"
	PhaseIncrementalSyncing Phase = "incremental_syncing"
	PhaseReadyForCutover    Phase = "ready_for_cutover"
	PhaseFinalSyncing       Phase = "final_syncing"
	PhaseTargetBooted       Phase = "target_booted"
	PhaseCompleted          Phase = "completed"
	PhaseFailed             Phase = "failed"
)

type Event string

const (
	EventPreflightPassed          Event = "preflight_passed"
	EventBaseSyncStarted          Event = "base_sync_started"
	EventBaseSyncCompleted        Event = "base_sync_completed"
	EventIncrementalSyncCompleted Event = "incremental_sync_completed"
	EventCutoverStarted           Event = "cutover_started"
	EventFinalSyncCompleted       Event = "final_sync_completed"
	EventTargetBooted             Event = "target_booted"
	EventSourceArchived           Event = "source_archived"
	EventFailed                   Event = "failed"
)

type Task struct {
	ID              string
	VMID            string
	SourceClusterID string
	TargetClusterID string
	OwnerClusterID  string
	Phase           Phase
	SourceLocked    bool
}

func NewTask(id, vmID, sourceClusterID, targetClusterID string) *Task {
	return &Task{
		ID:              id,
		VMID:            vmID,
		SourceClusterID: sourceClusterID,
		TargetClusterID: targetClusterID,
		OwnerClusterID:  sourceClusterID,
		Phase:           PhaseCreated,
		SourceLocked:    true,
	}
}

func (t *Task) Apply(event Event) error {
	switch event {
	case EventPreflightPassed:
		return t.transition(PhaseCreated, PhasePreflightPassed)
	case EventBaseSyncStarted:
		return t.transition(PhasePreflightPassed, PhaseBaseSyncing)
	case EventBaseSyncCompleted:
		return t.transition(PhaseBaseSyncing, PhaseIncrementalSyncing)
	case EventIncrementalSyncCompleted:
		return t.transition(PhaseIncrementalSyncing, PhaseReadyForCutover)
	case EventCutoverStarted:
		return t.transition(PhaseReadyForCutover, PhaseFinalSyncing)
	case EventFinalSyncCompleted:
		return t.transition(PhaseFinalSyncing, PhaseTargetBooted)
	case EventTargetBooted:
		if err := t.require(PhaseTargetBooted); err != nil {
			return err
		}
		t.OwnerClusterID = t.TargetClusterID
		return nil
	case EventSourceArchived:
		if t.OwnerClusterID != t.TargetClusterID {
			return fmt.Errorf("cannot archive source before target ownership is committed")
		}
		if err := t.transition(PhaseTargetBooted, PhaseCompleted); err != nil {
			return err
		}
		t.SourceLocked = false
		return nil
	case EventFailed:
		t.Phase = PhaseFailed
		return nil
	default:
		return fmt.Errorf("unknown migration event %q", event)
	}
}

func (t *Task) transition(from Phase, to Phase) error {
	if err := t.require(from); err != nil {
		return err
	}
	t.Phase = to
	return nil
}

func (t *Task) require(phase Phase) error {
	if t.Phase != phase {
		return fmt.Errorf("invalid transition from phase %q; expected %q", t.Phase, phase)
	}
	return nil
}
