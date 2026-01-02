package main

import (
	"time"
)

// Message sending methods for JobRunner

func (jr *JobRunner) sendConnectMessage() {
	msg := Message{
		Type:      "connect",
		JobName:   jr.config.JobName,
		Instance:  jr.config.JobInstance,
		Machine:   jr.config.MachineID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	
	jr.msgMutex.Lock()
	jr.pendingMsgs = append(jr.pendingMsgs, msg)
	jr.msgMutex.Unlock()
	
	if jr.ws.IsConnected() {
		jr.ws.Send(msg)
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
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	
	jr.msgMutex.Lock()
	jr.pendingMsgs = append(jr.pendingMsgs, msg)
	jr.msgMutex.Unlock()
	
	if jr.ws.IsConnected() {
		jr.ws.Send(msg)
	}
}

func (jr *JobRunner) sendOutputMessage(data string) {
	jr.msgMutex.Lock()
	defer jr.msgMutex.Unlock()
	
	if jr.waitingForAck {
		return // Wait for ACK before sending more
	}
	
	msg := Message{
		Type:      "output",
		JobName:   jr.config.JobName,
		Instance:  jr.config.JobInstance,
		Machine:   jr.config.MachineID,
		Stream:    "stdout",
		Data:      data,
		Seq:       jr.nextSeq,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	
	jr.pendingMsgs = append(jr.pendingMsgs, msg)
	jr.nextSeq++
	
	if jr.ws.IsConnected() {
		jr.ws.Send(msg)
		jr.waitingForAck = true
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
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	
	jr.ws.Send(msg)
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
	
	msg := Message{
		Type:      "complete",
		JobName:   jr.config.JobName,
		Instance:  jr.config.JobInstance,
		Machine:   jr.config.MachineID,
		RetCode:   finalCode,
		Seq:       jr.nextSeq,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	
	jr.msgMutex.Lock()
	jr.pendingMsgs = append(jr.pendingMsgs, msg)
	jr.msgMutex.Unlock()
	
	// Retry sending completion until success
	for attempt := 0; attempt < 30; attempt++ {
		if jr.ws.IsConnected() {
			jr.ws.Send(msg)
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
		}
		time.Sleep(2 * time.Second)
	}
	
	jr.logger.Log("Failed to send completion after 30 attempts")
}
