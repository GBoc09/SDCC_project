package registry

import (
	"errors"
	"sync"
	"testing"
)

func TestInstanceLifecycle(t *testing.T) {
	store := New("registry-1")

	created, wasCreated, err := store.UpsertInstance("payments", "payment-1", InstanceInput{
		Address: "10.0.0.10",
		Port:    8080,
	})
	if err != nil || !wasCreated {
		t.Fatalf("create instance: created=%v err=%v", wasCreated, err)
	}

	updated, wasCreated, err := store.UpsertInstance("payments", "payment-1", InstanceInput{
		Address: "10.0.0.11",
		Port:    9090,
	})
	if err != nil || wasCreated {
		t.Fatalf("update instance: created=%v err=%v", wasCreated, err)
	}
	if updated.Version <= created.Version {
		t.Fatalf("updated version %d is not newer than %d", updated.Version, created.Version)
	}

	instances := store.Discover("payments")
	if len(instances) != 1 || instances[0].Address != "10.0.0.11" {
		t.Fatalf("unexpected discovery result: %#v", instances)
	}

	if err := store.DeleteInstance("payments", "payment-1"); err != nil {
		t.Fatalf("delete instance: %v", err)
	}
	if err := store.DeleteInstance("payments", "payment-1"); err != nil {
		t.Fatalf("repeated deletion should be idempotent: %v", err)
	}
	if instances := store.Discover("payments"); len(instances) != 0 {
		t.Fatalf("deleted instance returned by discovery: %#v", instances)
	}
}

func TestDeleteAndRecreateService(t *testing.T) {
	store := New("registry-1")
	for _, id := range []string{"payment-1", "payment-2"} {
		if _, _, err := store.UpsertInstance("payments", id, InstanceInput{
			Address: "127.0.0.1",
			Port:    8080,
		}); err != nil {
			t.Fatalf("register %s: %v", id, err)
		}
	}

	if err := store.DeleteService("payments"); err != nil {
		t.Fatalf("delete service: %v", err)
	}
	if instances := store.Discover("payments"); len(instances) != 0 {
		t.Fatalf("deleted service returned instances: %#v", instances)
	}

	if _, created, err := store.UpsertInstance("payments", "payment-3", InstanceInput{
		Address: "127.0.0.1",
		Port:    8081,
	}); err != nil || !created {
		t.Fatalf("recreate service: created=%v err=%v", created, err)
	}
	instances := store.Discover("payments")
	if len(instances) != 1 || instances[0].ID != "payment-3" {
		t.Fatalf("old instances reappeared after recreation: %#v", instances)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name     string
		service  string
		instance string
		input    InstanceInput
		expected error
	}{
		{"empty service", "", "one", InstanceInput{Address: "localhost", Port: 80}, ErrInvalidServiceName},
		{"empty ID", "api", "", InstanceInput{Address: "localhost", Port: 80}, ErrInvalidInstanceID},
		{"empty address", "api", "one", InstanceInput{Port: 80}, ErrInvalidAddress},
		{"zero port", "api", "one", InstanceInput{Address: "localhost"}, ErrInvalidPort},
		{"large port", "api", "one", InstanceInput{Address: "localhost", Port: 65536}, ErrInvalidPort},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := New("registry-1").UpsertInstance(test.service, test.instance, test.input)
			if !errors.Is(err, test.expected) {
				t.Fatalf("got %v, want %v", err, test.expected)
			}
		})
	}
}

func TestConcurrentUpserts(t *testing.T) {
	store := New("registry-1")
	const count = 100

	var waitGroup sync.WaitGroup
	for index := 0; index < count; index++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			_, _, err := store.UpsertInstance("workers", instanceID(index), InstanceInput{
				Address: "127.0.0.1",
				Port:    8000 + index,
			})
			if err != nil {
				t.Errorf("upsert instance %d: %v", index, err)
			}
		}(index)
	}
	waitGroup.Wait()

	if instances := store.Discover("workers"); len(instances) != count {
		t.Fatalf("got %d instances, want %d", len(instances), count)
	}
}

func instanceID(index int) string {
	const digits = "0123456789"
	if index < 10 {
		return "worker-" + string(digits[index])
	}
	return "worker-" + string(digits[index/10]) + string(digits[index%10])
}

func TestIsNewerRecord(t *testing.T) {
	tests := []struct {
		name          string
		remoteVersion uint64
		remoteOrigin  string
		localVersion  uint64
		localOrigin   string
		want          bool
	}{
		{
			name:          "remote version is greater",
			remoteVersion: 3,
			remoteOrigin:  "registry-1",
			localVersion:  2,
			localOrigin:   "registry-2",
			want:          true,
		},
		{
			name:          "remote version is smaller",
			remoteVersion: 1,
			remoteOrigin:  "registry-2",
			localVersion:  2,
			localOrigin:   "registry-1",
			want:          false,
		},
		{
			name:          "same version and remote origin is greater",
			remoteVersion: 2,
			remoteOrigin:  "registry-2",
			localVersion:  2,
			localOrigin:   "registry-1",
			want:          true,
		},
		{
			name:          "same version and remote origin is smaller",
			remoteVersion: 2,
			remoteOrigin:  "registry-1",
			localVersion:  2,
			localOrigin:   "registry-2",
			want:          false,
		},
		{
			name:          "same version and same origin",
			remoteVersion: 2,
			remoteOrigin:  "registry-1",
			localVersion:  2,
			localOrigin:   "registry-1",
			want:          false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isNewerRecord(
				test.remoteVersion,
				test.remoteOrigin,
				test.localVersion,
				test.localOrigin,
			)

			if got != test.want {
				t.Fatalf("isNewerRecord() = %v, want %v", got, test.want)
			}
		})
	}
}
