package main

import "encoding/json"

// Goclaw WebSocket protocol frame types.
// Matches the protocol defined in goclaw/websocket-protocol.md

// Frame types
const (
	FrameTypeRequest  = "req"
	FrameTypeResponse = "res"
	FrameTypeEvent    = "event"
)

// RequestFrame is a client-to-server request.
type RequestFrame struct {
	Type   string          `json:"type"`
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// ResponseFrame is a server-to-client response.
type ResponseFrame struct {
	Type    string      `json:"type"`
	ID      string      `json:"id"`
	OK      bool        `json:"ok"`
	Payload any         `json:"payload,omitempty"`
	Error   *ErrorShape `json:"error,omitempty"`
}

// EventFrame is a server-to-client push event.
type EventFrame struct {
	Type    string `json:"type"`
	Event   string `json:"event"`
	Payload any    `json:"payload,omitempty"`
}

// ErrorShape represents an RPC error.
type ErrorShape struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ChatSendParams matches goclaw's chat.send parameters.
type ChatSendParams struct {
	Message    string   `json:"message"`
	AgentID    string   `json:"agentId"`
	SessionKey string   `json:"sessionKey"`
	Stream     bool     `json:"stream"`
	Media      []string `json:"media,omitempty"`
}

// ConnectParams for the connect handshake.
type ConnectParams struct {
	Token    string `json:"token,omitempty"`
	UserID   string `json:"user_id,omitempty"`
	SenderID string `json:"sender_id,omitempty"`
}

// ConnectResponsePayload is returned after successful connect.
type ConnectResponsePayload struct {
	Protocol int    `json:"protocol"`
	Role     string `json:"role"`
	UserID   string `json:"user_id"`
	Server   struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"server"`
}

// ChunkEventPayload is the payload for streaming chunk events.
type ChunkEventPayload struct {
	Content string `json:"content"`
}

// AgentEventPayload is the payload for agent.* events.
// Matches the AgentEvent struct in goclaw/internal/agent/loop_types.go
type AgentEventPayload struct {
	Type    string `json:"type"` // run.started, run.completed, run.failed, chunk, tool.call, etc.
	AgentID string `json:"agentId"`
	RunID   string `json:"runId,omitempty"`
	Payload any    `json:"payload,omitempty"` // nested payload (map[string]string for chunks, map[string]any for run.completed)
}

// ChatEventPayload is the payload for chat.* events.
type ChatEventPayload struct {
	Type    string `json:"type"` // chunk, message, thinking
	Content string `json:"content,omitempty"`
}
