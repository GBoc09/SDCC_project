package registry

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidServiceName = errors.New("service name is required")
	ErrInvalidInstanceID  = errors.New("instance ID is required")
	ErrInvalidAddress     = errors.New("address is required")
	ErrInvalidPort        = errors.New("port must be between 1 and 65535")
	ErrServiceNotFound    = errors.New("service not found")
	ErrInstanceNotFound   = errors.New("instance not found")
)

type Registry struct {
	mu        sync.RWMutex
	nodeID    string
	version   uint64
	services  map[string]ServiceRecord
	instances map[string]map[string]InstanceRecord
}

func New(nodeID string) *Registry {
	return &Registry{
		nodeID:    nodeID,
		services:  make(map[string]ServiceRecord),
		instances: make(map[string]map[string]InstanceRecord),
	}
}

func (r *Registry) InsertUpdateInstance(serviceName, instanceID string, input InstanceInput) (InstanceRecord, bool, error) {
	serviceName = strings.TrimSpace(serviceName)
	instanceID = strings.TrimSpace(instanceID)
	input.Address = strings.TrimSpace(input.Address)
	if err := validate(serviceName, instanceID, input); err != nil {
		return InstanceRecord{}, false, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	service, serviceExists := r.services[serviceName]
	if !serviceExists || service.Status == StatusDeleted {
		service = ServiceRecord{
			Name:       serviceName,
			Status:     StatusActive,
			Version:    r.nextVersion(),
			OriginNode: r.nodeID,
			UpdatedAt:  now,
		}
		r.services[serviceName] = service
	}

	if r.instances[serviceName] == nil {
		r.instances[serviceName] = make(map[string]InstanceRecord)
	}
	previous, exists := r.instances[serviceName][instanceID]
	created := !exists || previous.Status == StatusDeleted
	instance := InstanceRecord{
		ID:          instanceID,
		ServiceName: serviceName,
		Address:     input.Address,
		Port:        input.Port,
		Status:      StatusActive,
		Version:     r.nextVersion(),
		OriginNode:  r.nodeID,
		UpdatedAt:   now,
	}
	r.instances[serviceName][instanceID] = instance
	return instance, created, nil
}

func (r *Registry) Discover(serviceName string) []InstanceRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	service, exists := r.services[serviceName]
	if !exists || service.Status != StatusActive {
		return []InstanceRecord{}
	}

	result := make([]InstanceRecord, 0, len(r.instances[serviceName]))
	for _, instance := range r.instances[serviceName] {
		if instance.Status == StatusActive {
			result = append(result, instance)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func (r *Registry) DeleteInstance(serviceName, instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instances, exists := r.instances[serviceName]
	if !exists {
		return ErrInstanceNotFound
	}
	instance, exists := instances[instanceID]
	if !exists {
		return ErrInstanceNotFound
	}
	if instance.Status == StatusDeleted {
		return nil
	}

	instance.Status = StatusDeleted
	instance.Version = r.nextVersion()
	instance.OriginNode = r.nodeID
	instance.UpdatedAt = time.Now().UTC()
	instances[instanceID] = instance
	return nil
}

func (r *Registry) DeleteService(serviceName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	service, exists := r.services[serviceName]
	if !exists {
		return ErrServiceNotFound
	}
	if service.Status == StatusDeleted {
		return nil
	}

	now := time.Now().UTC()
	service.Status = StatusDeleted
	service.Version = r.nextVersion()
	service.OriginNode = r.nodeID
	service.UpdatedAt = now
	r.services[serviceName] = service

	for id, instance := range r.instances[serviceName] {
		if instance.Status == StatusDeleted {
			continue
		}
		instance.Status = StatusDeleted
		instance.Version = r.nextVersion()
		instance.OriginNode = r.nodeID
		instance.UpdatedAt = now
		r.instances[serviceName][id] = instance
	}
	return nil
}

func (r *Registry) nextVersion() uint64 {
	r.version++
	return r.version
}

func validate(serviceName, instanceID string, input InstanceInput) error {
	switch {
	case serviceName == "":
		return ErrInvalidServiceName
	case instanceID == "":
		return ErrInvalidInstanceID
	case input.Address == "":
		return ErrInvalidAddress
	case input.Port < 1 || input.Port > 65535:
		return ErrInvalidPort
	default:
		return nil
	}
}
