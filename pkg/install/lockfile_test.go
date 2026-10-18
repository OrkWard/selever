package install

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireRelease(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	l, err := AcquireLock(lockPath, time.Second)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	// File exists while locked.
	if _, err := os.Stat(lockPath); err != nil {
		t.Errorf("lock file should exist: %v", err)
	}

	// Release.
	if err := l.Release(); err != nil {
		t.Errorf("Release: %v", err)
	}

	// The lock file stays, but the lock can be taken again.
	second, err := AcquireLock(lockPath, time.Second)
	if err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	}
	second.Release()
}

func TestAcquireLockContention(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	// Hold lock.
	first, err := AcquireLock(lockPath, time.Second)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer first.Release()

	// Second acquisition should time out.
	_, err = AcquireLock(lockPath, 200*time.Millisecond)
	if err == nil {
		t.Error("expected timeout acquiring contested lock")
	}
}

func TestAcquireLockStaleFile(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	// A lock file left behind by a killed run must not block anyone.
	if err := os.WriteFile(lockPath, []byte("999999\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	l, err := AcquireLock(lockPath, time.Second)
	if err != nil {
		t.Fatalf("AcquireLock over stale file: %v", err)
	}
	l.Release()
}

func TestLockPath(t *testing.T) {
	if got := LockPath("/foo/bar.json"); got != "/foo/bar.json.lock" {
		t.Errorf("LockPath = %q", got)
	}
}
