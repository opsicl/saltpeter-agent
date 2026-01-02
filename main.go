package main

import (
	"fmt"
	"os"
)

// Version is the wrapper version - update this for new releases
const Version = "1.0.0-beta.1"

func main() {
	// Check if we're already daemonized
	if os.Getenv("_SP_DAEMON") != "1" {
		// Not daemonized yet - re-exec as daemon
		if err := daemonize(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to daemonize: %v\n", err)
			os.Exit(1)
		}
		// Parent process - return success to Salt immediately
		fmt.Println("Saltpeter Wrapper version", Version)
		fmt.Println("Wrapper started successfully")
		os.Exit(0)
	}

	// We're the daemon - run the actual job
	if err := runWrapper(); err != nil {
		// Log error but don't exit - wrapper handles its own lifecycle
		logError("Wrapper error: %v", err)
	}
}

func runWrapper() error {
	// Parse configuration from environment
	config, err := parseConfig()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Initialize logging
	logger, err := newLogger(config)
	if err != nil {
		return fmt.Errorf("logger error: %w", err)
	}
	defer logger.Close()

	logger.Log("Starting job: %s", config.Command)

	// Run the job with WebSocket streaming
	runner := NewJobRunner(config, logger)
	exitCode := runner.Run()

	logger.Log("Job completed with exit code: %d", exitCode)
	return nil
}

func logError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
