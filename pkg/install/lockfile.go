package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Lockfile is an exclusive file lock. The zero value is unusable; create via AcquireLock.
type Lockfile struct {
	path string
	file *os.File
}

// AcquireLock creates an exclusive lock at path, retrying every 100ms up to
// timeout. If timeout is zero a reasonable default is used.
func AcquireLock(path string, timeout time.Duration) (*Lockfile, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			fmt.Fprintf(f, "%d\n", os.Getpid())
			return &Lockfile{path: path, file: f}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("acquire lock %s: timed out after %s", path, timeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Release releases the lock and removes the lock file.
func (l *Lockfile) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	l.file.Close()
	os.Remove(l.path)
	l.file = nil
	return nil
}

// LockPath is the conventional path for a lock file alongside a data file.
func LockPath(dataPath string) string {
	return dataPath + ".lock"
}

// readPid reads a PID from a lock file.
func readPid(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, err
	}
	return pid, nil
}
