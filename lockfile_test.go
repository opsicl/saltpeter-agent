package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestAcquireLockfile_Success(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	jr := &JobRunner{
		config: &Config{LockfilePath: lockPath},
		logger: &Logger{level: "off"},
	}

	acquired, err := jr.acquireLockfile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Fatal("expected to acquire lockfile")
	}

	// Lockfile should exist with our PID
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Fatal("lockfile was not created")
	}

	jr.releaseLockfile()
}

func TestAcquireLockfile_BlocksSecondInstance(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	jr1 := &JobRunner{
		config: &Config{LockfilePath: lockPath},
		logger: &Logger{level: "off"},
	}
	jr2 := &JobRunner{
		config: &Config{LockfilePath: lockPath},
		logger: &Logger{level: "off"},
	}

	acquired, err := jr1.acquireLockfile()
	if err != nil {
		t.Fatalf("jr1 unexpected error: %v", err)
	}
	if !acquired {
		t.Fatal("jr1 should have acquired lockfile")
	}

	// Second instance should be blocked
	acquired, err = jr2.acquireLockfile()
	if err != nil {
		t.Fatalf("jr2 unexpected error: %v", err)
	}
	if acquired {
		t.Fatal("jr2 should NOT have acquired lockfile while jr1 holds it")
	}

	jr1.releaseLockfile()
}

func TestAcquireLockfile_ReacquireAfterRelease(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	jr1 := &JobRunner{
		config: &Config{LockfilePath: lockPath},
		logger: &Logger{level: "off"},
	}
	jr2 := &JobRunner{
		config: &Config{LockfilePath: lockPath},
		logger: &Logger{level: "off"},
	}

	// First acquires and releases
	acquired, _ := jr1.acquireLockfile()
	if !acquired {
		t.Fatal("jr1 should have acquired")
	}
	jr1.releaseLockfile()

	// Second should now succeed
	acquired, err := jr2.acquireLockfile()
	if err != nil {
		t.Fatalf("jr2 unexpected error: %v", err)
	}
	if !acquired {
		t.Fatal("jr2 should have acquired after jr1 released")
	}

	jr2.releaseLockfile()
}

func TestAcquireLockfile_ReleasedOnFileClose(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	// Acquire lock directly via flock to simulate a process holding it
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("flock: %v", err)
	}

	// Closing the file should release the flock
	f.Close()

	jr := &JobRunner{
		config: &Config{LockfilePath: lockPath},
		logger: &Logger{level: "off"},
	}
	acquired, err := jr.acquireLockfile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Fatal("should have acquired after previous holder closed file")
	}

	jr.releaseLockfile()
}

func TestAcquireLockfile_EmptyPathSkips(t *testing.T) {
	jr := &JobRunner{
		config: &Config{LockfilePath: ""},
		logger: &Logger{level: "off"},
	}

	acquired, err := jr.acquireLockfile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Fatal("empty lockfile path should always succeed")
	}
}
