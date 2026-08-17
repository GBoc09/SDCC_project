package registry

func isNewerRecord(
	remoteVersion uint64,
	remoteOrigin string,
	localVersion uint64,
	localOrigin string,
) bool {
	if remoteVersion != localVersion {
		return remoteVersion > localVersion
	}

	return remoteOrigin > localOrigin
}
func (r *Registry) MergeInstance(remote InstanceRecord) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	serviceInstances := r.instances[remote.ServiceName]
	if serviceInstances == nil {
		serviceInstances = make(map[string]InstanceRecord)
		r.instances[remote.ServiceName] = serviceInstances
	}

	local, exists := serviceInstances[remote.ID]

	if exists && !isNewerRecord(
		remote.Version,
		remote.OriginNode,
		local.Version,
		local.OriginNode,
	) {
		return false
	}

	serviceInstances[remote.ID] = remote

	if remote.Version > r.version {
		r.version = remote.Version
	}

	return true
}
func (r *Registry) MergeService(remote ServiceRecord) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	local, exists := r.services[remote.Name]

	if exists && !isNewerRecord(
		remote.Version,
		remote.OriginNode,
		local.Version,
		local.OriginNode,
	) {
		return false
	}

	r.services[remote.Name] = remote

	if remote.Version > r.version {
		r.version = remote.Version
	}

	return true
}
