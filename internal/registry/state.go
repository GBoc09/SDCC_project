package registry

import "sort"

type RegistryState struct {
	Services  []ServiceRecord  `json:"services"`
	Instances []InstanceRecord `json:"instances"`
}

func (r *Registry) Snapshot() RegistryState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state := RegistryState{
		Services:  make([]ServiceRecord, 0, len(r.services)),
		Instances: make([]InstanceRecord, 0),
	}

	for _, service := range r.services {
		state.Services = append(state.Services, service)
	}

	for _, serviceInstances := range r.instances {
		for _, instance := range serviceInstances {
			state.Instances = append(state.Instances, instance)
		}
	}

	sort.Slice(state.Services, func(i, j int) bool {
		return state.Services[i].Name < state.Services[j].Name
	})

	sort.Slice(state.Instances, func(i, j int) bool {
		if state.Instances[i].ServiceName != state.Instances[j].ServiceName {
			return state.Instances[i].ServiceName <
				state.Instances[j].ServiceName
		}

		return state.Instances[i].ID < state.Instances[j].ID
	})

	return state
}
func (r *Registry) MergeState(remote RegistryState) int {
	applied := 0

	for _, service := range remote.Services {
		if r.MergeService(service) {
			applied++
		}
	}

	for _, instance := range remote.Instances {
		if r.MergeInstance(instance) {
			applied++
		}
	}

	return applied
}
