package socket

type HostPIDResolver struct {
	ReadCurrentPIDNSInode func() (uint64, error)
	ReadPIDNamespaceInode func(int) (uint64, error)
	ReadNamespacedPIDs    func(int) ([]int, error)
	ReadStartTimeTicks    func(int) (uint64, error)
	ListPIDs              func() ([]int, error)
}

func (r HostPIDResolver) Resolve(session Session) (int, bool) {
	if session.AgentPID <= 0 {
		return 0, false
	}

	shouldTryNamespaceFirst := session.AgentInJail || session.AgentPID < 100

	if r.ReadCurrentPIDNSInode != nil && session.AgentPIDNamespaceInode > 0 {
		if currentNS, err := r.ReadCurrentPIDNSInode(); err == nil && currentNS > 0 && session.AgentPIDNamespaceInode != currentNS {
			shouldTryNamespaceFirst = true
		}
	}

	if shouldTryNamespaceFirst {
		if hostPID, ok := r.findByNamespace(session); ok {
			return hostPID, true
		}
	}

	if r.matches(session.AgentPID, session) {
		return session.AgentPID, true
	}

	return r.findByNamespace(session)
}

func (r HostPIDResolver) matches(pid int, session Session) bool {
	if pid <= 0 {
		return false
	}

	if r.ReadStartTimeTicks != nil && session.AgentStartTimeTicks > 0 {
		startTimeTicks, err := r.ReadStartTimeTicks(pid)
		if err != nil || startTimeTicks != session.AgentStartTimeTicks {
			return false
		}
	}

	if r.ReadPIDNamespaceInode != nil && session.AgentPIDNamespaceInode > 0 {
		nsInode, err := r.ReadPIDNamespaceInode(pid)
		if err != nil || nsInode != session.AgentPIDNamespaceInode {
			return false
		}
	}

	return true
}

func (r HostPIDResolver) findByNamespace(session Session) (int, bool) {
	if session.AgentPID <= 0 || session.AgentPIDNamespaceInode == 0 {
		return 0, false
	}
	if r.ListPIDs == nil || r.ReadPIDNamespaceInode == nil || r.ReadNamespacedPIDs == nil {
		return 0, false
	}

	pids, err := r.ListPIDs()
	if err != nil {
		return 0, false
	}

	candidates := make([]int, 0, 1)
	for _, pid := range pids {
		nsInode, err := r.ReadPIDNamespaceInode(pid)
		if err != nil || nsInode != session.AgentPIDNamespaceInode {
			continue
		}

		nsPIDs, err := r.ReadNamespacedPIDs(pid)
		if err != nil || !containsPID(nsPIDs, session.AgentPID) {
			continue
		}

		if r.ReadStartTimeTicks != nil && session.AgentStartTimeTicks > 0 {
			startTimeTicks, err := r.ReadStartTimeTicks(pid)
			if err != nil || startTimeTicks != session.AgentStartTimeTicks {
				continue
			}
		}

		candidates = append(candidates, pid)
	}

	if len(candidates) == 1 {
		return candidates[0], true
	}
	if len(candidates) > 1 && session.AgentStartTimeTicks == 0 {
		return candidates[0], true
	}

	return 0, false
}

func containsPID(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
