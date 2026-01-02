package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)

// Lockfile handling to prevent job overlap

func (jr *JobRunner) acquireLockfile() (bool, error) {
	if jr.config.LockfilePath == "" {
		return true, nil
	}
	
	// Check if lockfile exists
	if _, err := os.Stat(jr.config.LockfilePath); err == nil {
		// Lockfile exists, check if process is still running
		data, err := os.ReadFile(jr.config.LockfilePath)
		if err == nil {
			pidStr := string(data)
			if pid, err := strconv.Atoi(pidStr); err == nil {
				// Check if process exists
				if err := syscall.Kill(pid, 0); err == nil {
					// Process exists, overlap not allowed
					return false, nil
				}
			}
		}
		// Stale lockfile, remove it
		os.Remove(jr.config.LockfilePath)
	}
	
	// Write our PID to lockfile
	pid := os.Getpid()
	if err := os.WriteFile(jr.config.LockfilePath, []byte(fmt.Sprintf("%d\n", pid)), 0644); err != nil {
		return false, fmt.Errorf("write lockfile: %w", err)
	}
	
	jr.logger.Debug("Acquired lockfile: %s", jr.config.LockfilePath)
	return true, nil
}

func (jr *JobRunner) releaseLockfile() {
	if jr.config.LockfilePath == "" {
		return
	}
	
	if err := os.Remove(jr.config.LockfilePath); err != nil {
		jr.logger.Debug("Failed to remove lockfile: %v", err)
	} else {
		jr.logger.Debug("Released lockfile: %s", jr.config.LockfilePath)
	}
}
