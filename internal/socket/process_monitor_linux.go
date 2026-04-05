//go:build linux

package socket

import (
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sys/unix"
)

var newSessionProcessMonitor = func(session Session, onExit func()) (sessionProcessMonitor, error) {
	hostPID, err := resolveSessionMonitorPID(session)
	if err != nil {
		return nil, err
	}

	fd, err := unix.PidfdOpen(hostPID, 0)
	if err != nil {
		return nil, fmt.Errorf("pidfd_open pid=%d: %w", hostPID, err)
	}

	monitor := &pidfdProcessMonitor{
		fd:     fd,
		done:   make(chan struct{}),
		onExit: onExit,
	}
	go monitor.wait()
	return monitor, nil
}

type pidfdProcessMonitor struct {
	fd     int
	done   chan struct{}
	onExit func()

	once sync.Once
}

func (m *pidfdProcessMonitor) Close() error {
	var err error
	m.once.Do(func() {
		close(m.done)
		err = unix.Close(m.fd)
		if errors.Is(err, unix.EBADF) {
			err = nil
		}
	})
	return err
}

func (m *pidfdProcessMonitor) wait() {
	_, err := unix.Poll([]unix.PollFd{{
		Fd:     int32(m.fd),
		Events: unix.POLLIN,
	}}, -1)

	select {
	case <-m.done:
		return
	default:
	}

	if err != nil {
		debugf("pidfd wait failed fd=%d err=%v", m.fd, err)
		return
	}
	if m.onExit != nil {
		m.onExit()
	}
}

func resolveSessionMonitorPID(session Session) (int, error) {
	if session.AgentPID <= 0 {
		return 0, errors.New("agent pid is missing")
	}

	if hostPID, ok := newLivenessHostPIDResolver().Resolve(session); ok {
		return hostPID, nil
	}

	if session.AgentInJail || session.AgentPIDNamespaceInode > 0 && session.AgentPID < 100 {
		return 0, errors.New("host pid could not be resolved")
	}

	return session.AgentPID, nil
}
