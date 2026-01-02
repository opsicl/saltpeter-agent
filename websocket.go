package main

import (
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

// Message types
type Message struct {
	Type      string `json:"type"`
	JobName   string `json:"job_name"`
	Instance  string `json:"job_instance"`
	Machine   string `json:"machine"`
	Timestamp string `json:"timestamp"`
	
	// Type-specific fields
	PID     int    `json:"pid,omitempty"`
	Version string `json:"version,omitempty"`
	Stream  string `json:"stream,omitempty"`
	Data    string `json:"data,omitempty"`
	Seq     *int   `json:"seq,omitempty"` // Pointer to distinguish missing vs 0
	RetCode *int   `json:"retcode,omitempty"` // Pointer so 0 is sent (not omitted)
	Message string `json:"message,omitempty"` // For NACK and error messages
	
	// ACK fields
	AckType     string `json:"ack_type,omitempty"`
	LastAckedSeq int   `json:"last_acked_seq,omitempty"`
	NextSeq     int    `json:"next_seq,omitempty"`
	ExpectedSeq int    `json:"expected_seq,omitempty"`
	LastSeq     int    `json:"last_seq,omitempty"`
}

// WebSocketClient manages WebSocket connection and messaging
type WebSocketClient struct {
	config *Config
	logger *Logger
	runner *JobRunner
	conn   *websocket.Conn
}

// NewWebSocketClient creates a new WebSocket client
func NewWebSocketClient(config *Config, logger *Logger, runner *JobRunner) *WebSocketClient {
	return &WebSocketClient{
		config: config,
		logger: logger,
		runner: runner,
	}
}

// Run starts the WebSocket client loop
func (wsc *WebSocketClient) Run() {
	retryInterval := 2 * time.Second
	
	for {
		// Try to connect
		conn, _, err := websocket.DefaultDialer.Dial(wsc.config.WebSocketURL, nil)
		if err != nil {
			wsc.logger.Debug("Connection failed: %v", err)
			time.Sleep(retryInterval)
			continue
		}
		
		wsc.conn = conn
		wsc.logger.Debug("WebSocket connected")
		
		// Handle messages
		wsc.handleConnection()
		
		// Connection closed, retry
		wsc.conn = nil
		time.Sleep(retryInterval)
	}
}

func (wsc *WebSocketClient) handleConnection() {
	// Start message reader
	go func() {
		for {
			var msg Message
			err := wsc.conn.ReadJSON(&msg)
			if err != nil {
				return
			}
			wsc.handleIncomingMessage(msg)
		}
	}()
	
	// Keep connection alive
	for wsc.conn != nil {
		time.Sleep(100 * time.Millisecond)
	}
}

func (wsc *WebSocketClient) handleIncomingMessage(msg Message) {
	wsc.logger.Debug("Received message: type=%s", msg.Type)
	
	switch msg.Type {
	case "ack":
		wsc.handleAck(msg)
	case "nack":
		wsc.logger.Debug("NACK received: expected_seq=%d", msg.ExpectedSeq)
	case "sync_response":
		wsc.handleSyncResponse(msg)
	case "kill":
		wsc.logger.Log("KILL message received from server")
		wsc.runner.killProcess(false)
	}
}

func (wsc *WebSocketClient) handleAck(msg Message) {
	wsc.runner.msgMutex.Lock()
	defer wsc.runner.msgMutex.Unlock()
	
	// Check sequence-based ACK first (output messages)
	if msg.Seq != nil {
		if *msg.Seq > wsc.runner.lastAckedSeq {
			wsc.logger.Debug("ACK received: seq=%d", *msg.Seq)
			wsc.runner.lastAckedSeq = *msg.Seq
			wsc.runner.waitingForAck = false
			// Remove ACKed messages
			newPending := []Message{}
			for _, pending := range wsc.runner.pendingMsgs {
				if pending.Seq != nil && *pending.Seq > *msg.Seq {
					newPending = append(newPending, pending)
				}
			}
			wsc.runner.pendingMsgs = newPending
		}
	} else if msg.AckType != "" {
		// Type-based ACK (connect, start, complete)
		wsc.logger.Debug("ACK received for %s", msg.AckType)
		// Remove from pending
		newPending := []Message{}
		for _, pending := range wsc.runner.pendingMsgs {
			if pending.Type != msg.AckType {
				newPending = append(newPending, pending)
			}
		}
		wsc.runner.pendingMsgs = newPending
	}
}

func (wsc *WebSocketClient) handleSyncResponse(msg Message) {
	wsc.runner.msgMutex.Lock()
	defer wsc.runner.msgMutex.Unlock()
	
	wsc.logger.Debug("Sync response: server_last=%d, our_last_acked=%d", msg.LastSeq, wsc.runner.lastAckedSeq)
	
	if msg.LastSeq == -1 {
		// Job doesn't exist on server
		wsc.logger.Log("Job no longer exists on server, stopping retries")
		wsc.runner.waitingForAck = false
		// Clear pending output
		newPending := []Message{}
		for _, pending := range wsc.runner.pendingMsgs {
			if pending.Type != "output" {
				newPending = append(newPending, pending)
			}
		}
		wsc.runner.pendingMsgs = newPending
	} else if msg.LastSeq >= wsc.runner.lastAckedSeq {
		wsc.runner.lastAckedSeq = msg.LastSeq
		wsc.runner.waitingForAck = false
		// Remove ACKed messages
		newPending := []Message{}
		for _, pending := range wsc.runner.pendingMsgs {
			if pending.Seq != nil && *pending.Seq > msg.LastSeq {
				newPending = append(newPending, pending)
			}
		}
		wsc.runner.pendingMsgs = newPending
	}
}

func (wsc *WebSocketClient) Send(msg Message) error {
	if wsc.conn == nil {
		return fmt.Errorf("not connected")
	}
	return wsc.conn.WriteJSON(msg)
}

func (wsc *WebSocketClient) IsConnected() bool {
	return wsc.conn != nil
}
