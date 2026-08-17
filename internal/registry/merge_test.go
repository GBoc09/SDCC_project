package registry

import (
	"testing"
	"time"
)

func TestMergeInstanceAcceptsNewerRecord(t *testing.T) {
	store := New("registry-2")

	local, _, err := store.UpsertInstance(
		"payments",
		"payment-1",
		InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create local instance: %v", err)
	}

	remote := InstanceRecord{
		ID:          "payment-1",
		ServiceName: "payments",
		Address:     "10.0.0.2",
		Port:        9090,
		Status:      StatusActive,
		Version:     local.Version + 1,
		OriginNode:  "registry-1",
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

	applied := store.MergeInstance(remote)

	if !applied {
		t.Fatal("newer remote record was not applied")
	}

	instances := store.Discover("payments")
	if len(instances) != 1 {
		t.Fatalf("got %d instances, want 1", len(instances))
	}

	got := instances[0]

	if got.Address != remote.Address {
		t.Errorf("address = %q, want %q", got.Address, remote.Address)
	}

	if got.Port != remote.Port {
		t.Errorf("port = %d, want %d", got.Port, remote.Port)
	}

	if got.Version != remote.Version {
		t.Errorf("version = %d, want %d", got.Version, remote.Version)
	}

	if got.OriginNode != remote.OriginNode {
		t.Errorf(
			"origin node = %q, want %q",
			got.OriginNode,
			remote.OriginNode,
		)
	}

	if !got.UpdatedAt.Equal(remote.UpdatedAt) {
		t.Errorf(
			"updatedAt = %v, want %v",
			got.UpdatedAt,
			remote.UpdatedAt,
		)
	}
}
func TestMergeInstanceRejectsOlderRecord(t *testing.T) {
	store := New("registry-1")

	local, _, err := store.UpsertInstance(
		"payments",
		"payment-1",
		InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create local instance: %v", err)
	}

	remote := InstanceRecord{
		ID:          "payment-1",
		ServiceName: "payments",
		Address:     "10.0.0.2",
		Port:        9090,
		Status:      StatusActive,
		Version:     local.Version - 1,
		OriginNode:  "registry-2",
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

	applied := store.MergeInstance(remote)

	if applied {
		t.Fatal("older remote record was applied")
	}

	instances := store.Discover("payments")
	if len(instances) != 1 {
		t.Fatalf("got %d instances, want 1", len(instances))
	}

	got := instances[0]

	if got.Address != local.Address {
		t.Errorf("address = %q, want %q", got.Address, local.Address)
	}

	if got.Port != local.Port {
		t.Errorf("port = %d, want %d", got.Port, local.Port)
	}

	if got.Version != local.Version {
		t.Errorf("version = %d, want %d", got.Version, local.Version)
	}

	if got.OriginNode != local.OriginNode {
		t.Errorf(
			"origin node = %q, want %q",
			got.OriginNode,
			local.OriginNode,
		)
	}
}
func TestMergeInstanceAcceptsGreaterOriginOnEqualVersion(t *testing.T) {
	store := New("registry-1")

	local, _, err := store.UpsertInstance(
		"payments",
		"payment-1",
		InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create local instance: %v", err)
	}

	remote := InstanceRecord{
		ID:          "payment-1",
		ServiceName: "payments",
		Address:     "10.0.0.2",
		Port:        9090,
		Status:      StatusActive,
		Version:     local.Version,
		OriginNode:  "registry-2",
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

	applied := store.MergeInstance(remote)

	if !applied {
		t.Fatal("record with greater origin node was not applied")
	}

	instances := store.Discover("payments")
	if len(instances) != 1 {
		t.Fatalf("got %d instances, want 1", len(instances))
	}

	got := instances[0]

	if got.Address != remote.Address {
		t.Errorf("address = %q, want %q", got.Address, remote.Address)
	}

	if got.Port != remote.Port {
		t.Errorf("port = %d, want %d", got.Port, remote.Port)
	}

	if got.Version != remote.Version {
		t.Errorf("version = %d, want %d", got.Version, remote.Version)
	}

	if got.OriginNode != remote.OriginNode {
		t.Errorf(
			"origin node = %q, want %q",
			got.OriginNode,
			remote.OriginNode,
		)
	}
}
func TestMergeInstanceRejectsSmallerOriginOnEqualVersion(t *testing.T) {
	store := New("registry-2")

	local, _, err := store.UpsertInstance(
		"payments",
		"payment-1",
		InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create local instance: %v", err)
	}

	remote := InstanceRecord{
		ID:          "payment-1",
		ServiceName: "payments",
		Address:     "10.0.0.2",
		Port:        9090,
		Status:      StatusActive,
		Version:     local.Version,
		OriginNode:  "registry-1",
		UpdatedAt:   local.UpdatedAt.Add(time.Hour),
	}

	applied := store.MergeInstance(remote)

	if applied {
		t.Fatal("record with smaller origin node was applied")
	}

	instances := store.Discover("payments")
	if len(instances) != 1 {
		t.Fatalf("got %d instances, want 1", len(instances))
	}

	got := instances[0]

	if got.Address != local.Address {
		t.Errorf("address = %q, want %q", got.Address, local.Address)
	}

	if got.Port != local.Port {
		t.Errorf("port = %d, want %d", got.Port, local.Port)
	}

	if got.Version != local.Version {
		t.Errorf("version = %d, want %d", got.Version, local.Version)
	}

	if got.OriginNode != local.OriginNode {
		t.Errorf(
			"origin node = %q, want %q",
			got.OriginNode,
			local.OriginNode,
		)
	}
}
func TestMergeInstanceIsIdempotent(t *testing.T) {
	store := New("registry-1")

	local, _, err := store.UpsertInstance(
		"payments",
		"payment-1",
		InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create local instance: %v", err)
	}

	remote := InstanceRecord{
		ID:          "payment-1",
		ServiceName: "payments",
		Address:     "10.0.0.2",
		Port:        9090,
		Status:      StatusActive,
		Version:     local.Version + 1,
		OriginNode:  "registry-2",
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

	firstApplied := store.MergeInstance(remote)
	if !firstApplied {
		t.Fatal("remote record was not applied the first time")
	}

	secondApplied := store.MergeInstance(remote)
	if secondApplied {
		t.Fatal("identical remote record was applied more than once")
	}

	instances := store.Discover("payments")
	if len(instances) != 1 {
		t.Fatalf("got %d instances, want 1", len(instances))
	}

	got := instances[0]

	if got != remote {
		t.Errorf("stored instance = %#v, want %#v", got, remote)
	}
}
func TestMergeInstanceAcceptsUnknownInstance(t *testing.T) {
	store := New("registry-1")

	_, _, err := store.UpsertInstance(
		"payments",
		"payment-1",
		InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create local instance: %v", err)
	}

	remote := InstanceRecord{
		ID:          "payment-2",
		ServiceName: "payments",
		Address:     "10.0.0.2",
		Port:        9090,
		Status:      StatusActive,
		Version:     10,
		OriginNode:  "registry-2",
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

	applied := store.MergeInstance(remote)

	if !applied {
		t.Fatal("unknown remote instance was not applied")
	}

	instances := store.Discover("payments")
	if len(instances) != 2 {
		t.Fatalf("got %d instances, want 2", len(instances))
	}

	var found bool

	for _, instance := range instances {
		if instance.ID != remote.ID {
			continue
		}

		found = true

		if instance != remote {
			t.Errorf(
				"stored instance = %#v, want %#v",
				instance,
				remote,
			)
		}
	}

	if !found {
		t.Fatal("remote instance was not returned by discovery")
	}
}
func TestMergeInstanceAdvancesLocalVersion(t *testing.T) {
	store := New("registry-1")

	_, _, err := store.UpsertInstance(
		"payments",
		"payment-1",
		InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create local instance: %v", err)
	}

	remote := InstanceRecord{
		ID:          "payment-1",
		ServiceName: "payments",
		Address:     "10.0.0.2",
		Port:        9090,
		Status:      StatusActive,
		Version:     10,
		OriginNode:  "registry-2",
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

	if applied := store.MergeInstance(remote); !applied {
		t.Fatal("remote record was not applied")
	}

	updated, created, err := store.UpsertInstance(
		"payments",
		"payment-1",
		InstanceInput{
			Address: "10.0.0.3",
			Port:    7070,
		},
	)
	if err != nil {
		t.Fatalf("update local instance: %v", err)
	}

	if created {
		t.Fatal("existing instance was reported as newly created")
	}

	if updated.Version <= remote.Version {
		t.Fatalf(
			"local version = %d, want greater than remote version %d",
			updated.Version,
			remote.Version,
		)
	}

	if updated.OriginNode != "registry-1" {
		t.Errorf(
			"origin node = %q, want %q",
			updated.OriginNode,
			"registry-1",
		)
	}

	if updated.Address != "10.0.0.3" {
		t.Errorf(
			"address = %q, want %q",
			updated.Address,
			"10.0.0.3",
		)
	}

	if updated.Port != 7070 {
		t.Errorf("port = %d, want %d", updated.Port, 7070)
	}
}
func TestMergeInstanceAppliesTombstone(t *testing.T) {
	store := New("registry-1")

	local, _, err := store.UpsertInstance(
		"payments",
		"payment-1",
		InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create local instance: %v", err)
	}

	tombstone := InstanceRecord{
		ID:          local.ID,
		ServiceName: local.ServiceName,
		Address:     local.Address,
		Port:        local.Port,
		Status:      StatusDeleted,
		Version:     local.Version + 1,
		OriginNode:  "registry-2",
		UpdatedAt:   local.UpdatedAt.Add(time.Minute),
	}

	applied := store.MergeInstance(tombstone)

	if !applied {
		t.Fatal("newer tombstone was not applied")
	}

	instances := store.Discover("payments")
	if len(instances) != 0 {
		t.Fatalf(
			"discovery returned deleted instances: %#v",
			instances,
		)
	}

	stored, exists := store.instances["payments"]["payment-1"]
	if !exists {
		t.Fatal("tombstone was not stored")
	}

	if stored != tombstone {
		t.Errorf(
			"stored record = %#v, want tombstone %#v",
			stored,
			tombstone,
		)
	}

	staleApplied := store.MergeInstance(local)
	if staleApplied {
		t.Fatal("stale active record replaced the tombstone")
	}

	instances = store.Discover("payments")
	if len(instances) != 0 {
		t.Fatalf(
			"deleted instance reappeared after stale merge: %#v",
			instances,
		)
	}
}
func TestSnapshotIncludesTombstones(t *testing.T) {
	store := New("registry-1")

	_, _, err := store.UpsertInstance(
		"payments",
		"payment-1",
		InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	if err := store.DeleteInstance("payments", "payment-1"); err != nil {
		t.Fatalf("delete instance: %v", err)
	}

	snapshot := store.Snapshot()

	if len(snapshot.Services) != 1 {
		t.Fatalf(
			"snapshot contains %d services, want 1",
			len(snapshot.Services),
		)
	}

	if len(snapshot.Instances) != 1 {
		t.Fatalf(
			"snapshot contains %d instances, want 1",
			len(snapshot.Instances),
		)
	}

	service := snapshot.Services[0]
	if service.Name != "payments" {
		t.Errorf(
			"service name = %q, want %q",
			service.Name,
			"payments",
		)
	}

	instance := snapshot.Instances[0]

	if instance.ID != "payment-1" {
		t.Errorf(
			"instance ID = %q, want %q",
			instance.ID,
			"payment-1",
		)
	}

	if instance.Status != StatusDeleted {
		t.Errorf(
			"instance status = %q, want %q",
			instance.Status,
			StatusDeleted,
		)
	}
}
func TestMergeStateAppliesRemoteSnapshot(t *testing.T) {
	source := New("registry-1")
	target := New("registry-2")

	_, _, err := source.UpsertInstance(
		"payments",
		"payment-1",
		InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create source instance: %v", err)
	}

	remoteState := source.Snapshot()

	applied := target.MergeState(remoteState)

	if applied != 2 {
		t.Fatalf(
			"applied records = %d, want 2",
			applied,
		)
	}

	instances := target.Discover("payments")
	if len(instances) != 1 {
		t.Fatalf(
			"target discovered %d instances, want 1",
			len(instances),
		)
	}

	got := instances[0]

	if got.ID != "payment-1" {
		t.Errorf(
			"instance ID = %q, want %q",
			got.ID,
			"payment-1",
		)
	}

	if got.Address != "10.0.0.1" {
		t.Errorf(
			"address = %q, want %q",
			got.Address,
			"10.0.0.1",
		)
	}

	if got.Port != 8080 {
		t.Errorf("port = %d, want %d", got.Port, 8080)
	}
}
func TestMergeStatePreservesTombstonesAgainstStaleState(t *testing.T) {
	source := New("registry-1")
	target := New("registry-2")

	_, _, err := source.UpsertInstance(
		"payments",
		"payment-1",
		InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create source instance: %v", err)
	}

	activeState := source.Snapshot()

	if applied := target.MergeState(activeState); applied != 2 {
		t.Fatalf(
			"initial merge applied %d records, want 2",
			applied,
		)
	}

	if err := source.DeleteInstance(
		"payments",
		"payment-1",
	); err != nil {
		t.Fatalf("delete source instance: %v", err)
	}

	deletedState := source.Snapshot()

	if applied := target.MergeState(deletedState); applied != 1 {
		t.Fatalf(
			"tombstone merge applied %d records, want 1",
			applied,
		)
	}

	if instances := target.Discover("payments"); len(instances) != 0 {
		t.Fatalf(
			"target returned deleted instances: %#v",
			instances,
		)
	}

	if applied := target.MergeState(activeState); applied != 0 {
		t.Fatalf(
			"stale merge applied %d records, want 0",
			applied,
		)
	}

	if instances := target.Discover("payments"); len(instances) != 0 {
		t.Fatalf(
			"instance reappeared after stale synchronization: %#v",
			instances,
		)
	}
}
