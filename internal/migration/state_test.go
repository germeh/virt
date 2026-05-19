package migration

import "testing"

func TestTaskStateAllowsWarmMigrationHappyPath(t *testing.T) {
	task := NewTask("mig-1", "vm-123", "cluster-a", "cluster-b")
	events := []Event{
		EventPreflightPassed,
		EventBaseSyncStarted,
		EventBaseSyncCompleted,
		EventIncrementalSyncCompleted,
		EventCutoverStarted,
		EventFinalSyncCompleted,
		EventTargetBooted,
		EventSourceArchived,
	}

	for _, event := range events {
		if err := task.Apply(event); err != nil {
			t.Fatalf("event %s failed in phase %s: %v", event, task.Phase, err)
		}
	}

	if task.Phase != PhaseCompleted {
		t.Fatalf("phase = %s, want %s", task.Phase, PhaseCompleted)
	}
	if task.SourceLocked {
		t.Fatal("source lock should be released after source archive")
	}
	if task.OwnerClusterID != "cluster-b" {
		t.Fatalf("owner = %s, want cluster-b", task.OwnerClusterID)
	}
}

func TestTaskStateRejectsTargetBootBeforeFinalSync(t *testing.T) {
	task := NewTask("mig-1", "vm-123", "cluster-a", "cluster-b")
	if err := task.Apply(EventTargetBooted); err == nil {
		t.Fatal("expected invalid transition error")
	}
}

func TestTaskStateRejectsSourceArchiveBeforeTargetOwnership(t *testing.T) {
	task := NewTask("mig-1", "vm-123", "cluster-a", "cluster-b")
	events := []Event{
		EventPreflightPassed,
		EventBaseSyncStarted,
		EventBaseSyncCompleted,
		EventIncrementalSyncCompleted,
		EventCutoverStarted,
		EventFinalSyncCompleted,
	}
	for _, event := range events {
		if err := task.Apply(event); err != nil {
			t.Fatalf("event %s failed: %v", event, err)
		}
	}

	if err := task.Apply(EventSourceArchived); err == nil {
		t.Fatal("expected source archive to fail before target ownership")
	}
	if task.OwnerClusterID != "cluster-a" {
		t.Fatalf("owner = %s, want cluster-a", task.OwnerClusterID)
	}
}

func TestTaskStateFailedEventMovesToFailed(t *testing.T) {
	task := NewTask("mig-1", "vm-123", "cluster-a", "cluster-b")
	if err := task.Apply(EventFailed); err != nil {
		t.Fatalf("failed event returned error: %v", err)
	}
	if task.Phase != PhaseFailed {
		t.Fatalf("phase = %s, want %s", task.Phase, PhaseFailed)
	}
}
