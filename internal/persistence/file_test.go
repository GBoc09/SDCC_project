package persistence

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/GBoc09/SDCC_project/internal/registry"
)

func TestFileStoreSaveAndLoad(t *testing.T) {
	path := filepath.Join(
		t.TempDir(),
		"registry-state.json",
	)

	store := NewFileStore(path)

	updatedAt := time.Date(
		2026,
		time.August,
		17,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	want := registry.RegistryState{
		Services: []registry.ServiceRecord{
			{
				Name:       "payments",
				Status:     registry.StatusActive,
				Version:    1,
				OriginNode: "registry-1",
				UpdatedAt:  updatedAt,
			},
		},
		Instances: []registry.InstanceRecord{
			{
				ID:          "payment-1",
				ServiceName: "payments",
				Address:     "10.0.0.1",
				Port:        8080,
				Status:      registry.StatusDeleted,
				Version:     3,
				OriginNode:  "registry-2",
				UpdatedAt:   updatedAt,
			},
		},
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("save state: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf(
			"loaded state = %#v, want %#v",
			got,
			want,
		)
	}
}
func TestFileStoreLoadMissingFileReturnsEmptyState(
	t *testing.T,
) {
	path := filepath.Join(
		t.TempDir(),
		"missing-state.json",
	)

	store := NewFileStore(path)

	state, err := store.Load()
	if err != nil {
		t.Fatalf(
			"load missing state file: %v",
			err,
		)
	}

	if len(state.Services) != 0 {
		t.Errorf(
			"loaded %d services, want 0",
			len(state.Services),
		)
	}

	if len(state.Instances) != 0 {
		t.Errorf(
			"loaded %d instances, want 0",
			len(state.Instances),
		)
	}
}
func TestPersisterSavesRegistryState(t *testing.T) {
	path := filepath.Join(
		t.TempDir(),
		"registry-state.json",
	)

	fileStore := NewFileStore(path)
	registryStore := registry.New("registry-1")

	_, _, err := registryStore.UpsertInstance(
		"payments",
		"payment-1",
		registry.InstanceInput{
			Address: "10.0.0.1",
			Port:    8080,
		},
	)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	persister := NewPersister(
		fileStore,
		registryStore,
		10*time.Millisecond,
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		persister.Run(ctx)
	}()

	deadline := time.After(time.Second)
	check := time.NewTicker(5 * time.Millisecond)
	defer check.Stop()

	saved := false

	for !saved {
		select {
		case <-deadline:
			t.Fatal("registry state was not persisted")

		case <-check.C:
			state, err := fileStore.Load()
			if err != nil {
				t.Fatalf("load persisted state: %v", err)
			}

			saved = len(state.Instances) == 1
		}
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("persister did not stop after cancellation")
	}
}
