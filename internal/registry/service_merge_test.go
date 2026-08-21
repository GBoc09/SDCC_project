package registry

import (
	"testing"
	"time"
)

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
