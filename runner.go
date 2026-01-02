package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// JobRunner manages the subprocess execution and WebSocket communication
type JobRunner struct {
	config  *Config
	logger  *Logger
	ws      *WebSocketClient
	process *exec.Cmd
	pgid    int
	
	killed         bool
	killedByTimeout bool
	killMutex      sync.Mutex
	
	outputBuffer   []string
	outputMutex    sync.Mutex
	
	nextSeq        int
	lastAckedSeq   int
	pendingMsgs    []Message
	waitingForAck  bool
	msgMutex       sync.Mutex
}

// NewJobRunner creates a new job runner
func NewJobRunner(config *Config, logger *Logger) *JobRunner {
	return &JobRunner{
		config:       config,
		logger:       logger,
		lastAckedSeq: -1,
	}
}

// Run executes the job and returns the exit code
func (jr *JobRunner) Run() int {
	// Check lockfile if overlap not allowed
	if !jr.config.AllowOverlap {
		acquired, err := jr.acquireLockfile()
		if err != nil {
			jr.logger.Log("Lockfile error: %v", err)
			return 254
		}
		if !acquired {
			jr.logger.Log("Job already running, overlap not allowed")
			return 254
		}
		defer jr.releaseLockfile()
	}

	// Connect to WebSocket
	jr.ws = NewWebSocketClient(jr.config, jr.logger, jr)
	go jr.ws.Run()

	// Wait for initial connection
	time.Sleep(100 * time.Millisecond)

	// Send connect and start messages
	jr.sendConnectMessage()
	jr.sendStartMessage()

	// Start the subprocess
	startTime := time.Now()
	if err := jr.startProcess(); err != nil {
		jr.logger.Log("Failed to start process: %v", err)
		return 1
	}

	// Start output reading goroutines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go jr.readOutput(ctx, jr.process.Stdout)
	go jr.readOutput(ctx, jr.process.Stderr)

	// Periodic tasks
	outputTicker := time.NewTicker(time.Duration(jr.config.OutputIntervalMS) * time.Millisecond)
	defer outputTicker.Stop()

	heartbeatTicker := time.NewTicker(5 * time.Second)
	defer heartbeatTicker.Stop()

	// Main loop
	for {
		select {
		case <-outputTicker.C:
			jr.flushOutput()
			
		case <-heartbeatTicker.C:
			jr.sendHeartbeat()
			
		case <-time.After(100 * time.Millisecond):
			// Check if process finished
			if jr.process.ProcessState != nil {
				cancel() // Stop reading goroutines
				jr.flushOutput()
				exitCode := jr.process.ProcessState.ExitCode()
				jr.sendCompleteMessage(exitCode)
				return exitCode
			}
			
			// Check timeout
			if jr.config.Timeout > 0 && !jr.killed {
				elapsed := time.Since(startTime).Seconds()
				if elapsed > float64(jr.config.Timeout) {
					jr.logger.Log("Timeout exceeded (%.1fs > %d), terminating", elapsed, jr.config.Timeout)
					jr.killProcess(true)
				}
			}
			
			// Handle kill if requested
			if jr.killed && jr.process.ProcessState == nil {
				jr.terminateProcess()
			}
		}
	}
}

func (jr *JobRunner) startProcess() error {
	jr.process = exec.Command("sh", "-c", jr.config.Command)
	
	if jr.config.WorkingDir != "" {
		jr.process.Dir = jr.config.WorkingDir
	}
	
	// Create new session (process group)
	jr.process.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	
	// Setup pipes
	stdout, err := jr.process.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}
	stderr, err := jr.process.StderrPipe()
	if err != nil {
		return fmt.Errorf("create stderr pipe: %w", err)
	}
	
	jr.process.Stdout = stdout
	jr.process.Stderr = stderr
	
	// Start process
	if err := jr.process.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}
	
	// Save process group ID
	jr.pgid = jr.process.Process.Pid
	
	jr.logger.Log("Process started with PID %d, PGID %d", jr.process.Process.Pid, jr.pgid)
	return nil
}

func (jr *JobRunner) readOutput(ctx context.Context, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max line size
	
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			line := scanner.Text() + "\n"
			jr.outputMutex.Lock()
			jr.outputBuffer = append(jr.outputBuffer, line)
			jr.outputMutex.Unlock()
		}
	}
}

func (jr *JobRunner) flushOutput() {
	jr.outputMutex.Lock()
	if len(jr.outputBuffer) == 0 {
		jr.outputMutex.Unlock()
		return
	}
	
	// Combine all buffered output
	var combined string
	for _, line := range jr.outputBuffer {
		combined += line
	}
	jr.outputBuffer = nil
	jr.outputMutex.Unlock()
	
	// Send as output message
	jr.sendOutputMessage(combined)
}

func (jr *JobRunner) killProcess(byTimeout bool) {
	jr.killMutex.Lock()
	defer jr.killMutex.Unlock()
	
	if jr.killed {
		return
	}
	
	jr.killed = true
	jr.killedByTimeout = byTimeout
	
	// Flush output before killing
	jr.flushOutput()
}

func (jr *JobRunner) terminateProcess() {
	if jr.process == nil || jr.process.ProcessState != nil {
		return
	}
	
	reason := "kill"
	if jr.killedByTimeout {
		reason = "timeout"
	}
	
	jr.logger.Log("Sending SIGTERM to process group %d (%s)", jr.pgid, reason)
	
	// Send SIGTERM to process group
	syscall.Kill(-jr.pgid, syscall.SIGTERM)
	
	// Wait 5 seconds for graceful shutdown
	done := make(chan bool)
	go func() {
		jr.process.Wait()
		done <- true
	}()
	
	select {
	case <-done:
		jr.logger.Log("Process died after SIGTERM (%s)", reason)
		return
	case <-time.After(5 * time.Second):
		// Escalate to SIGKILL
		jr.logger.Log("Escalating to SIGKILL (%s)", reason)
	}
	
	// Send SIGKILL with retries
	for attempt := 0; attempt < 10; attempt++ {
		syscall.Kill(-jr.pgid, syscall.SIGKILL)
		
		select {
		case <-done:
			jr.logger.Log("Process killed after SIGKILL")
			return
		case <-time.After(5 * time.Second):
			if attempt < 9 {
				jr.logger.Log("Process still alive after SIGKILL attempt %d, retrying...", attempt+1)
			}
		}
	}
	
	jr.logger.Log("WARNING: Process still alive after 10 SIGKILL attempts")
}
