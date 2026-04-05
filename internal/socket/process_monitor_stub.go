//go:build !linux

package socket

import "errors"

var newSessionProcessMonitor = func(session Session, onExit func()) (sessionProcessMonitor, error) {
	return nil, errors.New("pidfd monitor is only supported on linux")
}
