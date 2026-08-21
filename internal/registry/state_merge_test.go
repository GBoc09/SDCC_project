package registry

import "testing"

func TestSnapshotIncludesTombstones(t *testing.T) {
	store := New("registry-1")

	_, _, err := store.InsertUpdateInstance(
		"payments",
		"payment-1",
		InstanceInput{Address: "10.0.0.1", Port: 8080},
	)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if err := store.DeleteInstance("payments", "payment-1"); err != nil {
		t.Fatalf("delete instance: %v", err)
	}

	snapshot := store.Snapshot()
	if len(snapshot.Services) != 1 {
		t.Fatalf("snapshot contains %d services, want 1", len(snapshot.Services))
	}
	if len(snapshot.Instances) != 1 {
		t.Fatalf("snapshot contains %d instances, want 1", len(snapshot.Instances))
	}

	service := snapshot.Services[0]
	if service.Name != "payments" {
		t.Errorf("service name = %q, want %q", service.Name, "payments")
	}

	instance := snapshot.Instances[0]
	if instance.ID != "payment-1" {
		t.Errorf("instance ID = %q, want %q", instance.ID, "payment-1")
	}
	if instance.Status != StatusDeleted {
		t.Errorf("instance status = %q, want %q", instance.Status, StatusDeleted)
	}
}

func TestMergeStateAppliesRemoteSnapshot(t *testing.T) {
	source := New("registry-1")
	target := New("registry-2")

	_, _, err := source.InsertUpdateInstance(
		"payments",
		"payment-1",
		InstanceInput{Address: "10.0.0.1", Port: 8080},
	)
	if err != nil {
		t.Fatalf("create source instance: %v", err)
	}

	applied := target.MergeState(source.Snapshot())
	if applied != 2 {
		t.Fatalf("applied records = %d, want 2", applied)
	}

	instances := target.Discover("payments")
	if len(instances) != 1 {
		t.Fatalf("target discovered %d instances, want 1", len(instances))
	}

	got := instances[0]
	if got.ID != "payment-1" {
		t.Errorf("instance ID = %q, want %q", got.ID, "payment-1")
	}
	if got.Address != "10.0.0.1" {
		t.Errorf("address = %q, want %q", got.Address, "10.0.0.1")
	}
	if got.Port != 8080 {
		t.Errorf("port = %d, want %d", got.Port, 8080)
	}
}

func TestMergeStatePreservesTombstonesAgainstStaleState(t *testing.T) {
	source := New("registry-1")
	target := New("registry-2")

	_, _, err := source.InsertUpdateInstance(
		"payments",
		"payment-1",
		InstanceInput{Address: "10.0.0.1", Port: 8080},
	)
	if err != nil {
		t.Fatalf("create source instance: %v", err)
	}

	activeState := source.Snapshot()
	if applied := target.MergeState(activeState); applied != 2 {
		t.Fatalf("initial merge applied %d records, want 2", applied)
	}

	if err := source.DeleteInstance("payments", "payment-1"); err != nil {
		t.Fatalf("delete source instance: %v", err)
	}

	deletedState := source.Snapshot()
	if applied := target.MergeState(deletedState); applied != 1 {
		t.Fatalf("tombstone merge applied %d records, want 1", applied)
	}
	if instances := target.Discover("payments"); len(instances) != 0 {
		t.Fatalf("target returned deleted instances: %#v", instances)
	}

	if applied := target.MergeState(activeState); applied != 0 {
		t.Fatalf("stale merge applied %d records, want 0", applied)
	}
	if instances := target.Discover("payments"); len(instances) != 0 {
		t.Fatalf("instance reappeared after stale synchronization: %#v", instances)
	}
}
