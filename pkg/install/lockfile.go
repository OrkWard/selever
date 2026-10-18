package install

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Lockfile is an exclusive advisory lock held on an open file.
//
// The lock lives in the kernel, not in the existence of the file, so a run
// that is killed cannot leave a lock behind: the OS drops it when the process
// dies. Lock files are therefore never deleted, which also avoids the classic
// unlink race where a waiter ends up holding a lock on a deleted inode while
// another process locks a freshly created file of the same name.
type Lockfile struct {
	path string
	file *os.File
}

// AcquireLock takes an exclusive lock at path, retrying every 100ms up to
// timeout. If timeout is zero a reasonable default is used.
func AcquireLock(path string, timeout time.Duration) (*Lockfile, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	for {
		locked, err := tryLockFile(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("acquire lock %s: %w", path, err)
		}
		if locked {
			// The pid is only a hint for humans; the lock is the open file.
			if err := f.Truncate(0); err == nil {
				f.Seek(0, 0)
				fmt.Fprintf(f, "%d\n", os.Getpid())
				f.Sync()
			}
			return &Lockfile{path: path, file: f}, nil
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("acquire lock %s: timed out after %s", path, timeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Release releases the lock. The lock file itself is left in place.
func (l *Lockfile) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := unlockFile(l.file)
	if cerr := l.file.Close(); err == nil {
		err = cerr
	}
	l.file = nil
	return err
}

// LockPath is the conventional path for a lock file alongside a data file.
func LockPath(dataPath string) string {
	return dataPath + ".lock"
}
