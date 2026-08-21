package registry

import (
	"testing"
	"time"
)

func TestMergeInstanceAcceptsNewerRecord(t *testing.T) {
	store := New("registry-2")

	local, _, err := store.InsertUpdateInstance(
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

	local, _, err := store.InsertUpdateInstance(
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

	local, _, err := store.InsertUpdateInstance(
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

	local, _, err := store.InsertUpdateInstance(
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
