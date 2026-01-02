package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// Config holds all wrapper configuration from environment variables
type Config struct {
	WebSocketURL     string
	JobName          string
	JobInstance      string
	MachineID        string
	Command          string
	WorkingDir       string
	User             string
	Timeout          int // seconds, 0 = no timeout
	OutputIntervalMS int // milliseconds
	LogLevel         string
	LogDir           string
	AllowOverlap     bool
	LockfilePath     string
}

// parseConfig reads and validates environment variables
func parseConfig() (*Config, error) {
	config := &Config{
		WebSocketURL:     os.Getenv("SP_WEBSOCKET_URL"),
		JobName:          os.Getenv("SP_JOB_NAME"),
		JobInstance:      os.Getenv("SP_JOB_INSTANCE"),
		MachineID:        os.Getenv("SP_MACHINE_ID"),
		Command:          os.Getenv("SP_COMMAND"),
		WorkingDir:       os.Getenv("SP_CWD"),
		User:             os.Getenv("SP_USER"),
		OutputIntervalMS: 1000, // default 1 second
		LogLevel:         "normal",
		LogDir:           "/var/log/sp_wrapper",
		AllowOverlap:     true,
		LockfilePath:     os.Getenv("SP_LOCKFILE"),
	}

	// Required fields
	if config.WebSocketURL == "" {
		return nil, fmt.Errorf("SP_WEBSOCKET_URL not set")
	}
	if config.JobName == "" {
		return nil, fmt.Errorf("SP_JOB_NAME not set")
	}
	if config.JobInstance == "" {
		return nil, fmt.Errorf("SP_JOB_INSTANCE not set")
	}
	if config.Command == "" {
		return nil, fmt.Errorf("SP_COMMAND not set")
	}

	// Default machine ID to FQDN (like Python wrapper uses socket.getfqdn())
	if config.MachineID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("get hostname: %w", err)
		}
		
		// Try to get FQDN by looking up the hostname
		addrs, err := net.LookupHost(hostname)
		if err == nil && len(addrs) > 0 {
			// Reverse lookup first IP to get FQDN
			names, err := net.LookupAddr(addrs[0])
			if err == nil && len(names) > 0 {
				// Use first FQDN, strip trailing dot if present
				fqdn := names[0]
				if len(fqdn) > 0 && fqdn[len(fqdn)-1] == '.' {
					fqdn = fqdn[:len(fqdn)-1]
				}
				config.MachineID = fqdn
			} else {
				config.MachineID = hostname
			}
		} else {
			config.MachineID = hostname
		}
	}

	// Parse timeout
	if timeoutStr := os.Getenv("SP_TIMEOUT"); timeoutStr != "" {
		timeout, err := strconv.Atoi(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SP_TIMEOUT: %w", err)
		}
		config.Timeout = timeout
	}

	// Parse output interval
	if intervalStr := os.Getenv("SP_OUTPUT_INTERVAL_MS"); intervalStr != "" {
		interval, err := strconv.Atoi(intervalStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SP_OUTPUT_INTERVAL_MS: %w", err)
		}
		config.OutputIntervalMS = interval
	}

	// Parse log level
	if logLevel := os.Getenv("SP_WRAPPER_LOGLEVEL"); logLevel != "" {
		config.LogLevel = strings.ToLower(logLevel)
	}

	// Parse log directory
	if logDir := os.Getenv("SP_WRAPPER_LOGDIR"); logDir != "" {
		config.LogDir = logDir
	}

	// Parse allow overlap
	if overlapStr := os.Getenv("SP_ALLOW_OVERLAP"); overlapStr != "" {
		overlapStr = strings.ToLower(overlapStr)
		config.AllowOverlap = overlapStr == "true" || overlapStr == "1" || overlapStr == "yes"
	}

	// Default lockfile path
	if !config.AllowOverlap && config.LockfilePath == "" {
		config.LockfilePath = fmt.Sprintf("/tmp/sp_wrapper_%s.lock", config.JobName)
	}

	return config, nil
}
