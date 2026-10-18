//go:build windows

package install

import (
	"os"
	"syscall"
	"unsafe"
)

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002

	errorLockViolation syscall.Errno = 33
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

// tryLockFile takes a non-blocking exclusive lock on the whole file. It
// reports whether the lock was taken; a lock held elsewhere is not an error.
func tryLockFile(f *os.File) (bool, error) {
	var ol syscall.Overlapped
	r1, _, err := procLockFileEx.Call(
		f.Fd(),
		uintptr(lockfileExclusiveLock|lockfileFailImmediately),
		0,
		uintptr(^uint32(0)),
		uintptr(^uint32(0)),
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 != 0 {
		return true, nil
	}
	if errno, ok := err.(syscall.Errno); ok {
		if errno == errorLockViolation || errno == syscall.ERROR_IO_PENDING {
			return false, nil
		}
	}
	return false, err
}

func unlockFile(f *os.File) error {
	var ol syscall.Overlapped
	r1, _, err := procUnlockFileEx.Call(
		f.Fd(),
		0,
		uintptr(^uint32(0)),
		uintptr(^uint32(0)),
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 != 0 {
		return nil
	}
	return err
}
