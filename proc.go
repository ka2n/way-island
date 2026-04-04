package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type procStat struct {
	PPid           int
	TTYNr          int64
	StartTimeTicks uint64
}

func readProcStat(pid int) (procStat, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return procStat{}, err
	}

	text := strings.TrimSpace(string(data))
	closeIdx := strings.LastIndex(text, ")")
	if closeIdx == -1 || closeIdx+2 >= len(text) {
		return procStat{}, fmt.Errorf("unexpected stat format for pid %d", pid)
	}

	fields := strings.Fields(text[closeIdx+2:])
	if len(fields) < 20 {
		return procStat{}, fmt.Errorf("unexpected stat field count for pid %d", pid)
	}

	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return procStat{}, fmt.Errorf("parse ppid for pid %d: %w", pid, err)
	}
	ttyNr, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil {
		return procStat{}, fmt.Errorf("parse tty_nr for pid %d: %w", pid, err)
	}
	startTimeTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return procStat{}, fmt.Errorf("parse starttime for pid %d: %w", pid, err)
	}

	return procStat{
		PPid:           ppid,
		TTYNr:          ttyNr,
		StartTimeTicks: startTimeTicks,
	}, nil
}

func readPIDNamespaceInodeForPID(pid int) (uint64, error) {
	info, err := os.Stat(fmt.Sprintf("/proc/%d/ns/pid", pid))
	if err != nil {
		return 0, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("unexpected stat type for pid namespace %d", pid)
	}
	return stat.Ino, nil
}

func readCurrentPIDNamespaceInode() (uint64, error) {
	return readPIDNamespaceInodeForPID(os.Getpid())
}

func listProcPIDs() ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	pids := make([]int, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func readNSPIDsForPID(pid int) ([]int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "NSpid:") {
			continue
		}
		raw := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "NSpid:")))
		pids := make([]int, 0, len(raw))
		for _, value := range raw {
			pidValue, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("parse NSpid value %q for pid %d: %w", value, pid, err)
			}
			pids = append(pids, pidValue)
		}
		return pids, nil
	}

	return nil, fmt.Errorf("NSpid not found for pid %d", pid)
}

func readTTYNameForPID(pid int) string {
	for _, fd := range []int{0, 1, 2} {
		target, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%d", pid, fd))
		if err != nil {
			continue
		}
		if strings.HasPrefix(target, "/dev/") {
			return target
		}
	}
	return ""
}

func ttyBaseName(tty string) string {
	if strings.TrimSpace(tty) == "" {
		return ""
	}
	return filepath.Base(tty)
}
