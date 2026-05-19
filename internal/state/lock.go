package state

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Lock represents a held state lock backed by a file on disk.
type Lock struct {
	path string
}

// AcquireLock creates a lock file alongside the state file.
// Returns an error if the lock file already exists.
func AcquireLock(statePath string) (*Lock, error) {
	lockPath := statePath + ".lock"

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			pid := readLockPID(lockPath)
			if pid != "" {
				return nil, fmt.Errorf("state is locked by process %s (lock file: %s) — if no other disform process is running, delete the lock file manually", pid, lockPath)
			}
			return nil, fmt.Errorf("state is locked by another process (lock file: %s) — if no other disform process is running, delete the lock file manually", lockPath)
		}
		return nil, fmt.Errorf("acquiring state lock: %w", err)
	}
	defer f.Close()

	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	return &Lock{path: lockPath}, nil
}

// Release removes the lock file.
func (l *Lock) Release() {
	_ = os.Remove(l.path)
}

func readLockPID(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strconv.Itoa(func() int {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		return pid
	}())
}
