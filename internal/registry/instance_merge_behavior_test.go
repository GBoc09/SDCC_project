package registry

import (
	"testing"
	"time"
)

func TestMergeInstanceIsIdempotent(t *testing.T) {
	store := New("registry-1")
	local, _, err := store.InsertUpdateInstance(
		"payments", "payment-1", InstanceInput{Address: "10.0.0.1", Port: 8080},
	)
	if err != nil {
		t.Fatalf("create local instance: %v", err)
	}

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
	_, _, err := store.InsertUpdateInstance(
		"payments", "payment-1", InstanceInput{Address: "10.0.0.1", Port: 8080},
	)
	if err != nil {
		t.Fatalf("create local instance: %v", err)
	}

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
	_, _, err := store.InsertUpdateInstance(
		"payments", "payment-1", InstanceInput{Address: "10.0.0.1", Port: 8080},
	)
	if err != nil {
		t.Fatalf("create local instance: %v", err)
	}

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
	local, _, err := store.InsertUpdateInstance(
		"payments", "payment-1", InstanceInput{Address: "10.0.0.1", Port: 8080},
	)
	if err != nil {
		t.Fatalf("create local instance: %v", err)
	}

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
