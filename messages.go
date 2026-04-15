package main

import (
	"time"
)

// formatTimestamp returns ISO8601 timestamp compatible with Python's fromisoformat
func formatTimestamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000000+00:00")
}

// Message sending methods for JobRunner

func (jr *JobRunner) sendConnectMessage() {
	msg := Message{
		Type:      "connect",
		JobName:   jr.config.JobName,
		Instance:  jr.config.JobInstance,
		Machine:   jr.config.MachineID,
		Timestamp: formatTimestamp(),
	}
	
	jr.logger.Debug("Sending connect message: job=%s, instance=%s, machine=%s", jr.config.JobName, jr.config.JobInstance, jr.config.MachineID)
	
	jr.msgMutex.Lock()
	jr.pendingMsgs = append(jr.pendingMsgs, msg)
	jr.msgMutex.Unlock()
	
	if jr.ws.IsConnected() {
		if err := jr.ws.Send(msg); err != nil {
			jr.logger.Log("Failed to send connect message: %v", err)
		}
	}
}

func (jr *JobRunner) sendStartMessage() {
	msg := Message{
		Type:      "start",
		JobName:   jr.config.JobName,
		Instance:  jr.config.JobInstance,
		Machine:   jr.config.MachineID,
		PID:       jr.process.Process.Pid,
		Version:   Version,
		Timestamp: formatTimestamp(),
	}
	
	jr.logger.Debug("Sending start message: PID=%d", jr.process.Process.Pid)
	
	jr.msgMutex.Lock()
	jr.pendingMsgs = append(jr.pendingMsgs, msg)
	jr.msgMutex.Unlock()
	
	if jr.ws.IsConnected() {
		if err := jr.ws.Send(msg); err != nil {
			jr.logger.Log("Failed to send start message: %v", err)
		}
	}
}

func (jr *JobRunner) sendStartMessageWithError(pid int) {
	msg := Message{
		Type:      "start",
		JobName:   jr.config.JobName,
		Instance:  jr.config.JobInstance,
		Machine:   jr.config.MachineID,
		PID:       pid,
		Version:   Version,
		Timestamp: formatTimestamp(),
	}
	
	jr.msgMutex.Lock()
	jr.pendingMsgs = append(jr.pendingMsgs, msg)
	jr.msgMutex.Unlock()
	
	if jr.ws.IsConnected() {
		if err := jr.ws.Send(msg); err != nil {
			jr.logger.Log("Failed to send start-with-error message: %v", err)
		}
	}
}

func (jr *JobRunner) sendOutputMessage(data string) {
	jr.msgMutex.Lock()
	defer jr.msgMutex.Unlock()
	
	seq := jr.nextSeq // Capture current value
	msg := Message{
		Type:      "output",
		JobName:   jr.config.JobName,
		Instance:  jr.config.JobInstance,
		Machine:   jr.config.MachineID,
		Stream:    "stdout",
		Data:      data,
		Seq:       &seq, // Use captured value
		Timestamp: formatTimestamp(),
	}
	
	jr.logger.Debug("Sending output message: seq=%d, len=%d", seq, len(data))
	
	jr.pendingMsgs = append(jr.pendingMsgs, msg)
	jr.nextSeq++
	
	if jr.ws.IsConnected() {
		if err := jr.ws.Send(msg); err != nil {
			jr.logger.Log("Failed to send output message seq=%d: %v", seq, err)
		} else {
			jr.waitingForAck = true
		}
	}
}

func (jr *JobRunner) sendHeartbeat() {
	if !jr.ws.IsConnected() {
		return
	}
	
	msg := Message{
		Type:      "heartbeat",
		JobName:   jr.config.JobName,
		Instance:  jr.config.JobInstance,
		Machine:   jr.config.MachineID,
		Timestamp: formatTimestamp(),
	}
	
	if err := jr.ws.Send(msg); err != nil {
		jr.logger.Log("Failed to send heartbeat: %v", err)
	}
}

func (jr *JobRunner) sendCompleteMessage(exitCode int) {
	// Determine final exit code
	finalCode := exitCode
	if jr.killed {
		if jr.killedByTimeout {
			finalCode = 124 // GNU timeout convention
		} else {
			finalCode = 143 // SIGTERM
		}
	}
	
	seq := jr.nextSeq // Capture current value
	msg := Message{
		Type:      "complete",
		JobName:   jr.config.JobName,
		Instance:  jr.config.JobInstance,
		Machine:   jr.config.MachineID,
		RetCode:   &finalCode,
		Seq:       &seq, // Use captured value
		Timestamp: formatTimestamp(),
	}
	
	jr.msgMutex.Lock()
	jr.pendingMsgs = append(jr.pendingMsgs, msg)
	jr.msgMutex.Unlock()
	
	jr.logger.Log("Starting completion send (seq=%d, retcode=%d)", seq, finalCode)
	
	// Retry sending completion until ACKed
	for attempt := 0; attempt < 30; attempt++ {
		if jr.ws.IsConnected() {
			if err := jr.ws.Send(msg); err != nil {
				jr.logger.Log("Attempt %d: Failed to send completion: %v", attempt+1, err)
			} else {
				jr.logger.Debug("Attempt %d: Completion message sent, waiting for ACK", attempt+1)
			}
			time.Sleep(2 * time.Second)
			
			// Check if ACKed
			jr.msgMutex.Lock()
			found := false
			for _, pending := range jr.pendingMsgs {
				if pending.Type == "complete" {
					found = true
					break
				}
			}
			jr.msgMutex.Unlock()
			
			if !found {
				jr.logger.Log("Completion message acknowledged")
				return
			}
			jr.logger.Log("Attempt %d: Completion not yet acknowledged, retrying...", attempt+1)
		} else {
			jr.logger.Log("Attempt %d: WebSocket not connected, waiting for reconnection...", attempt+1)
			time.Sleep(2 * time.Second)
		}
	}
	
	jr.logger.Log("Failed to send completion after 30 attempts")
}
