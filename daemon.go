package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// daemonize re-execs the current process as a daemon
func daemonize() error {
	// Get path to current executable
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	// Prepare command to re-exec ourselves
	cmd := exec.Command(executable)
	cmd.Env = append(os.Environ(), "_SP_DAEMON=1")

	// Create new session (detach from controlling terminal)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Equivalent to os.setsid() in Python
	}

	// Redirect stdio to /dev/null
	devNull, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open /dev/null: %w", err)
	}
	defer devNull.Close()

	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull

	// Start the daemon process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	// Daemon started successfully, parent can exit
	return nil
}
