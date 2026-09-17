package registry

import (
	"testing"
	"time"
)

func TestMergeInstanceConflictResolution(t *testing.T) {
	tests := []struct {
		name         string
		localOrigin  string
		remoteOrigin string
		versionDelta int
		wantApplied  bool
	}{
		{"newer version", "registry-2", "registry-1", 1, true},
		{"older version", "registry-1", "registry-2", -1, false},
		{"equal version greater origin", "registry-1", "registry-2", 0, true},
		{"equal version smaller origin", "registry-2", "registry-1", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := New(tt.localOrigin)
			local := createMergeInstance(t, store)
			remote := local
			remote.Address = "10.0.0.2"
			remote.Port = 9090
			remote.Version = uint64(int(local.Version) + tt.versionDelta)
			remote.OriginNode = tt.remoteOrigin
			remote.UpdatedAt = local.UpdatedAt.Add(time.Hour)

			if applied := store.MergeInstance(remote); applied != tt.wantApplied {
				t.Fatalf("applied = %v, want %v", applied, tt.wantApplied)
			}
			want := local
			if tt.wantApplied {
				want = remote
			}
			instances := store.Discover("payments")
			if len(instances) != 1 {
				t.Fatalf("got %d instances, want 1", len(instances))
			}
			if instances[0] != want {
				t.Errorf("stored instance = %#v, want %#v", instances[0], want)
			}
		})
	}
}

func createMergeInstance(t *testing.T, store *Registry) InstanceRecord {
	t.Helper()
	instance, _, err := store.InsertUpdateInstance(
		"payments", "payment-1", InstanceInput{Address: "10.0.0.1", Port: 8080},
	)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	return instance
}

func TestMergeInstanceIsIdempotent(t *testing.T) {
	store := New("registry-1")
	local := createMergeInstance(t, store)

	remote := InstanceRecord{
		ID: "payment-1", ServiceName: "payments", Address: "10.0.0.2", Port: 9090,
		Status: StatusActive, Version: local.Version + 1, OriginNode: "registry-2",
		UpdatedAt: time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC),
	}
	if !store.MergeInstance(remote) {
		t.Fatal("remote record was not applied the first time")
	}
	if store.MergeInstance(remote) {
		t.Fatal("identical remote record was applied more than once")
	}

	instances := store.Discover("payments")
	if len(instances) != 1 {
		t.Fatalf("got %d instances, want 1", len(instances))
	}
	if instances[0] != remote {
		t.Errorf("stored instance = %#v, want %#v", instances[0], remote)
	}
}

func TestMergeInstanceAcceptsUnknownInstance(t *testing.T) {
	store := New("registry-1")
	createMergeInstance(t, store)

	remote := InstanceRecord{
		ID: "payment-2", ServiceName: "payments", Address: "10.0.0.2", Port: 9090,
		Status: StatusActive, Version: 10, OriginNode: "registry-2",
		UpdatedAt: time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC),
	}
	if !store.MergeInstance(remote) {
		t.Fatal("unknown remote instance was not applied")
	}

	instances := store.Discover("payments")
	if len(instances) != 2 {
		t.Fatalf("got %d instances, want 2", len(instances))
	}

	for _, instance := range instances {
		if instance.ID == remote.ID {
			if instance != remote {
				t.Errorf("stored instance = %#v, want %#v", instance, remote)
			}
			return
		}
	}
	t.Fatal("remote instance was not returned by discovery")
}

func TestMergeInstanceAdvancesLocalVersion(t *testing.T) {
	store := New("registry-1")
	createMergeInstance(t, store)

	remote := InstanceRecord{
		ID: "payment-1", ServiceName: "payments", Address: "10.0.0.2", Port: 9090,
		Status: StatusActive, Version: 10, OriginNode: "registry-2",
		UpdatedAt: time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC),
	}
	if !store.MergeInstance(remote) {
		t.Fatal("remote record was not applied")
	}

	updated, created, err := store.InsertUpdateInstance(
		"payments", "payment-1", InstanceInput{Address: "10.0.0.3", Port: 7070},
	)
	if err != nil {
		t.Fatalf("update local instance: %v", err)
	}
	if created {
		t.Fatal("existing instance was reported as newly created")
	}
	if updated.Version <= remote.Version {
		t.Fatalf("local version = %d, want greater than remote version %d", updated.Version, remote.Version)
	}
	if updated.OriginNode != "registry-1" {
		t.Errorf("origin node = %q, want %q", updated.OriginNode, "registry-1")
	}
	if updated.Address != "10.0.0.3" {
		t.Errorf("address = %q, want %q", updated.Address, "10.0.0.3")
	}
	if updated.Port != 7070 {
		t.Errorf("port = %d, want %d", updated.Port, 7070)
	}
}

func TestMergeInstanceAppliesTombstone(t *testing.T) {
	store := New("registry-1")
	local := createMergeInstance(t, store)

	tombstone := InstanceRecord{
		ID: local.ID, ServiceName: local.ServiceName, Address: local.Address, Port: local.Port,
		Status: StatusDeleted, Version: local.Version + 1, OriginNode: "registry-2",
		UpdatedAt: local.UpdatedAt.Add(time.Minute),
	}
	if !store.MergeInstance(tombstone) {
		t.Fatal("newer tombstone was not applied")
	}
	if instances := store.Discover("payments"); len(instances) != 0 {
		t.Fatalf("discovery returned deleted instances: %#v", instances)
	}

	stored, exists := store.instances["payments"]["payment-1"]
	if !exists {
		t.Fatal("tombstone was not stored")
	}
	if stored != tombstone {
		t.Errorf("stored record = %#v, want tombstone %#v", stored, tombstone)
	}
	if store.MergeInstance(local) {
		t.Fatal("stale active record replaced the tombstone")
	}
	if instances := store.Discover("payments"); len(instances) != 0 {
		t.Fatalf("deleted instance reappeared after stale merge: %#v", instances)
	}
}

func TestMergeServiceAcceptsNewerRecord(t *testing.T) {
	store := New("registry-1")

	_, _, err := store.InsertUpdateInstance(
		"payments",
		"payment-1",
		InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create local service: %v", err)
	}

	local, exists := store.services["payments"]
	if !exists {
		t.Fatal("local service was not created")
	}

	remote := ServiceRecord{
		Name:       "payments",
		Status:     StatusActive,
		Version:    local.Version + 1,
		OriginNode: "registry-2",
		UpdatedAt: time.Date(
			2026,
			time.August,
			11,
			12,
			0,
			0,
			0,
			time.UTC,
		),
	}

	applied := store.MergeService(remote)

	if !applied {
		t.Fatal("newer remote service was not applied")
	}

	got, exists := store.services["payments"]
	if !exists {
		t.Fatal("merged service was not stored")
	}

	if got != remote {
		t.Errorf(
			"stored service = %#v, want %#v",
			got,
			remote,
		)
	}
}
func TestMergeServiceAppliesTombstone(t *testing.T) {
	store := New("registry-1")

	for _, id := range []string{"payment-1", "payment-2"} {
		_, _, err := store.InsertUpdateInstance(
			"payments",
			id,
			InstanceInput{
				Address: "10.0.0.1",
				Port:    8080,
			},
		)
		if err != nil {
			t.Fatalf("create instance %q: %v", id, err)
		}
	}

	local, exists := store.services["payments"]
	if !exists {
		t.Fatal("local service was not created")
	}

	tombstone := ServiceRecord{
		Name:       "payments",
		Status:     StatusDeleted,
		Version:    local.Version + 10,
		OriginNode: "registry-2",
		UpdatedAt:  local.UpdatedAt.Add(time.Minute),
	}

	applied := store.MergeService(tombstone)

	if !applied {
		t.Fatal("newer service tombstone was not applied")
	}

	instances := store.Discover("payments")
	if len(instances) != 0 {
		t.Fatalf(
			"discovery returned instances of deleted service: %#v",
			instances,
		)
	}

	stored, exists := store.services["payments"]
	if !exists {
		t.Fatal("service tombstone was not stored")
	}

	if stored != tombstone {
		t.Errorf(
			"stored service = %#v, want tombstone %#v",
			stored,
			tombstone,
		)
	}

	staleApplied := store.MergeService(local)
	if staleApplied {
		t.Fatal("stale active service replaced the tombstone")
	}

	instances = store.Discover("payments")
	if len(instances) != 0 {
		t.Fatalf(
			"deleted service reappeared after stale merge: %#v",
			instances,
		)
	}
}

func TestSnapshotIncludesTombstones(t *testing.T) {
	store := New("registry-1")

	createMergeInstance(t, store)
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

	createMergeInstance(t, source)

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

	createMergeInstance(t, source)

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
