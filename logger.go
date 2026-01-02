package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Logger handles wrapper logging to file
type Logger struct {
	file      *os.File
	level     string
	jobName   string
	instance  string
	disabled  bool
}

// newLogger creates a new logger instance
func newLogger(config *Config) (*Logger, error) {
	logger := &Logger{
		level:    config.LogLevel,
		jobName:  config.JobName,
		instance: config.JobInstance,
		disabled: config.LogLevel == "off",
	}

	if logger.disabled {
		return logger, nil
	}

	// Create log directory
	if err := os.MkdirAll(config.LogDir, 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	// Open log file in append mode
	logPath := filepath.Join(config.LogDir, fmt.Sprintf("%s.log", config.JobName))
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	logger.file = file
	return logger, nil
}

// Log writes a normal log message
func (l *Logger) Log(format string, args ...interface{}) {
	l.log("normal", format, args...)
}

// Debug writes a debug log message (only if level is debug)
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log("debug", format, args...)
}

// log writes a log message with timestamp and prefix
func (l *Logger) log(level string, format string, args ...interface{}) {
	if l.disabled {
		return
	}
	if level == "debug" && l.level != "debug" {
		return
	}

	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("%s [%s] %s\n", timestamp, l.instance, message)

	if l.file != nil {
		l.file.WriteString(logLine)
	}
}

// Close closes the log file
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
