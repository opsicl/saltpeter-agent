package main

import (
	"fmt"
	"os"
	"syscall"
)

// Lockfile handling to prevent job overlap using flock.
// The lockfile path is per-job (/tmp/sp_wrapper_<jobname>.lock),
// so different cron jobs don't interfere with each other.

func (jr *JobRunner) acquireLockfile() (bool, error) {
	if jr.config.LockfilePath == "" {
		return true, nil
	}

	f, err := os.OpenFile(jr.config.LockfilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return false, fmt.Errorf("open lockfile: %w", err)
	}

	// Non-blocking exclusive lock — fails immediately if another process holds it
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		// EWOULDBLOCK means another instance holds the lock
		if err == syscall.EWOULDBLOCK {
			return false, nil
		}
		return false, fmt.Errorf("flock lockfile: %w", err)
	}

	// Write PID for observability (so admins can identify the holder)
	f.Truncate(0)
	f.Seek(0, 0)
	fmt.Fprintf(f, "%d\n", os.Getpid())
	f.Sync()

	jr.lockFile = f
	jr.logger.Debug("Acquired lockfile: %s", jr.config.LockfilePath)
	return true, nil
}

func (jr *JobRunner) releaseLockfile() {
	if jr.lockFile == nil {
		return
	}

	// Closing the file descriptor releases the flock automatically
	jr.lockFile.Close()
	os.Remove(jr.config.LockfilePath)
	jr.lockFile = nil
	jr.logger.Debug("Released lockfile: %s", jr.config.LockfilePath)
}
