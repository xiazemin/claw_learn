package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Bridge manages a WebSocket connection to goclaw and provides
// a synchronous request/response API on top of it.
type Bridge struct {
	cfg    *Config
	conn   *websocket.Conn
	nextID atomic.Int64

	// pending maps request ID → response channel
	pending sync.Map

	// eventCh receives server-push events (chunk, tool.call, etc.)
	eventCh chan *EventFrame

	// connected is set after successful connect handshake
	connected atomic.Bool

	// mu protects conn writes
	mu sync.Mutex

	// cancel cancels the read loop
	cancel context.CancelFunc
	// done is closed when the read loop exits
	done chan struct{}
}

// NewBridge creates a new bridge to goclaw.
func NewBridge(cfg *Config) *Bridge {
	return &Bridge{
		cfg:     cfg,
		eventCh: make(chan *EventFrame, 512),
		done:    make(chan struct{}),
	}
}

// Connect establishes the WebSocket connection and performs the handshake.
func (b *Bridge) Connect(ctx context.Context) error {
	slog.Info("connecting to goclaw", "url", b.cfg.GoclawURL)

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, b.cfg.GoclawURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	b.conn = conn

	// Start read loop
	readCtx, cancel := context.WithCancel(ctx)
	b.cancel = cancel
	go b.readLoop(readCtx)

	// Send connect handshake
	connectParams := map[string]any{
		"user_id": b.cfg.UserID,
	}
	if b.cfg.GatewayToken != "" {
		connectParams["token"] = b.cfg.GatewayToken
	}

	resp, err := b.sendRequest("connect", connectParams)
	if err != nil {
		cancel()
		conn.Close()
		return fmt.Errorf("connect handshake: %w", err)
	}

	if !resp.OK {
		cancel()
		conn.Close()
		if resp.Error != nil {
			return fmt.Errorf("connect rejected: %s - %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("connect rejected: unknown error")
	}

	b.connected.Store(true)
	slog.Info("connected to goclaw", "payload", resp.Payload)
	return nil
}

// Disconnect cleanly closes the bridge.
func (b *Bridge) Disconnect() {
	b.connected.Store(false)
	if b.cancel != nil {
		b.cancel()
	}

	b.mu.Lock()
	if b.conn != nil {
		b.conn.Close()
	}
	b.mu.Unlock()

	select {
	case <-b.done:
	case <-time.After(5 * time.Second):
		slog.Warn("bridge read loop did not exit in time")
	}
}

// ChatSend sends a message and waits for the full response (sync mode).
func (b *Bridge) ChatSend(ctx context.Context, message string, agentID string) (string, error) {
	if !b.connected.Load() {
		return "", fmt.Errorf("not connected to goclaw")
	}

	sessionKey := fmt.Sprintf("agent:%s:http:direct:%s", agentID, uuid.NewString())

	params := map[string]any{
		"message":    message,
		"agentId":    agentID,
		"sessionKey": sessionKey,
		"stream":     true,
	}

	resp, err := b.sendRequest("chat.send", params)
	if err != nil {
		return "", fmt.Errorf("chat.send: %w", err)
	}
	if !resp.OK {
		if resp.Error != nil {
			return "", fmt.Errorf("chat failed: %s - %s", resp.Error.Code, resp.Error.Message)
		}
		return "", fmt.Errorf("chat failed: unknown error")
	}

	// Extract content from response payload
	// goclaw sends: {"runId": "...", "content": "...", "usage": {...}}
	if payloadMap, ok := resp.Payload.(map[string]any); ok {
		if content, ok := payloadMap["content"].(string); ok {
			return content, nil
		}
	}

	// Fallback: try as string
	if content, ok := resp.Payload.(string); ok {
		return content, nil
	}

	return fmt.Sprintf("%v", resp.Payload), nil
}

// ChatStream sends a message and returns channels for streaming chunks.
func (b *Bridge) ChatStream(ctx context.Context, message string, agentID string) (<-chan string, <-chan error) {
	chunks := make(chan string, 256)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errCh)

		if !b.connected.Load() {
			errCh <- fmt.Errorf("not connected to goclaw")
			return
		}

		sessionKey := fmt.Sprintf("agent:%s:http:stream:%s", agentID, uuid.NewString())

		params := map[string]any{
			"message":    message,
			"agentId":    agentID,
			"sessionKey": sessionKey,
			"stream":     true,
		}

		_, err := b.sendRequest("chat.send", params)
		if err != nil {
			errCh <- fmt.Errorf("chat.send: %w", err)
			return
		}

		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case event, ok := <-b.eventCh:
				if !ok {
					return
				}

				switch event.Event {
				case "agent":
					var payload AgentEventPayload
					if data, err := json.Marshal(event.Payload); err == nil {
						json.Unmarshal(data, &payload)
					}

					switch payload.Type {
					case "run.completed":
						return
					case "run.failed":
						errCh <- fmt.Errorf("agent run failed")
						return
					case "run.cancelled":
						return
					case "chunk":
						if c := extractNestedContent(payload.Payload); c != "" {
							chunks <- c
						}
					}

				case "chat":
					var payload ChatEventPayload
					if data, err := json.Marshal(event.Payload); err == nil {
						json.Unmarshal(data, &payload)
					}
					if payload.Type == "chunk" && payload.Content != "" {
						chunks <- payload.Content
					}

				case "shutdown":
					errCh <- fmt.Errorf("goclaw shutting down")
					return
				}
			}
		}
	}()

	return chunks, errCh
}

// History retrieves chat history.
func (b *Bridge) History(ctx context.Context, sessionKey string, limit int) (any, error) {
	params := map[string]any{}
	if sessionKey != "" {
		params["sessionKey"] = sessionKey
	}
	if limit > 0 {
		params["limit"] = limit
	}

	resp, err := b.sendRequest("chat.history", params)
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("unknown error")
	}
	return resp.Payload, nil
}

// Abort aborts a running request for a session.
func (b *Bridge) Abort(ctx context.Context, sessionKey string) error {
	params := map[string]any{}
	if sessionKey != "" {
		params["sessionKey"] = sessionKey
	}

	resp, err := b.sendRequest("chat.abort", params)
	if err != nil {
		return err
	}
	if !resp.OK && resp.Error != nil {
		return fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
	}
	return nil
}

// SessionsList lists active sessions.
func (b *Bridge) SessionsList(ctx context.Context) (any, error) {
	resp, err := b.sendRequest("sessions.list", nil)
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("unknown error")
	}
	return resp.Payload, nil
}

// SessionsDelete deletes a session.
func (b *Bridge) SessionsDelete(ctx context.Context, sessionKey string) error {
	resp, err := b.sendRequest("sessions.delete", map[string]any{"sessionKey": sessionKey})
	if err != nil {
		return err
	}
	if !resp.OK && resp.Error != nil {
		return fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
	}
	return nil
}

// SkillsList lists available skills.
func (b *Bridge) SkillsList(ctx context.Context) (any, error) {
	resp, err := b.sendRequest("skills.list", nil)
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("unknown error")
	}
	return resp.Payload, nil
}

// ModelsList lists available models.
func (b *Bridge) ModelsList(ctx context.Context) (any, error) {
	resp, err := b.sendRequest("models.list", nil)
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("unknown error")
	}
	return resp.Payload, nil
}

// AgentInfo gets agent information.
func (b *Bridge) AgentInfo(ctx context.Context, agentKey string) (any, error) {
	params := map[string]any{}
	if agentKey != "" {
		params["agentId"] = agentKey
	}
	resp, err := b.sendRequest("agent", params)
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("unknown error")
	}
	return resp.Payload, nil
}

// Health checks server health.
func (b *Bridge) Health(ctx context.Context) (any, error) {
	resp, err := b.sendRequest("health", nil)
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("unknown error")
	}
	return resp.Payload, nil
}

// Status gets server status.
func (b *Bridge) Status(ctx context.Context) (any, error) {
	resp, err := b.sendRequest("status", nil)
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
		}
		return nil, fmt.Errorf("unknown error")
	}
	return resp.Payload, nil
}

// IsConnected returns whether the bridge is connected.
func (b *Bridge) IsConnected() bool {
	return b.connected.Load()
}

// processAgentEvent processes a single event and returns updated content.
// Returns (content, nil) for ongoing events, (content, err) for terminal events.
func (b *Bridge) processAgentEvent(event *EventFrame, content string) (string, error) {
	switch event.Event {
	case "agent":
		var payload AgentEventPayload
		if data, err := json.Marshal(event.Payload); err == nil {
			json.Unmarshal(data, &payload)
		}

		switch payload.Type {
		case "run.completed":
			if c := extractNestedContent(payload.Payload); c != "" {
				content = c
			}
			return content, nil

		case "run.failed":
			return "", fmt.Errorf("agent run failed")

		case "run.cancelled":
			return content, fmt.Errorf("agent run cancelled")

		case "chunk":
			if c := extractNestedContent(payload.Payload); c != "" {
				content += c
			}
		}

	case "chat":
		var payload ChatEventPayload
		if data, err := json.Marshal(event.Payload); err == nil {
			json.Unmarshal(data, &payload)
		}
		if payload.Type == "chunk" && payload.Content != "" {
			content += payload.Content
		}

	case "chunk":
		var payload ChunkEventPayload
		if data, err := json.Marshal(event.Payload); err == nil {
			json.Unmarshal(data, &payload)
		}
		if payload.Content != "" {
			content += payload.Content
		}

	case "shutdown":
		return content, fmt.Errorf("goclaw shutting down")
	}

	return content, nil
}

// extractNestedContent extracts the "content" field from an AgentEvent's Payload.
// The Payload is typically map[string]any with a "content" key for chunks and run.completed.
func extractNestedContent(payload any) string {
	if payload == nil {
		return ""
	}

	switch p := payload.(type) {
	case map[string]any:
		if c, ok := p["content"].(string); ok {
			return c
		}
	case map[string]string:
		return p["content"]
	}

	// Fallback: try JSON round-trip
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if c, ok := m["content"].(string); ok {
		return c
	}

	return ""
}

// sendRequest sends a request and waits for the matching response.
func (b *Bridge) sendRequest(method string, params map[string]any) (*ResponseFrame, error) {
	id := fmt.Sprintf("req-%d", b.nextID.Add(1))

	var paramsRaw json.RawMessage
	if params != nil {
		var err error
		paramsRaw, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
	}

	frame := RequestFrame{
		Type:   FrameTypeRequest,
		ID:     id,
		Method: method,
		Params: paramsRaw,
	}

	data, err := json.Marshal(frame)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Register pending response
	respCh := make(chan *ResponseFrame, 1)
	b.pending.Store(id, respCh)
	defer b.pending.Delete(id)

	// Send
	b.mu.Lock()
	err = b.conn.WriteMessage(websocket.TextMessage, data)
	b.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("write message: %w", err)
	}

	slog.Debug("sent request", "method", method, "id", id)

	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(180 * time.Second):
		return nil, fmt.Errorf("request timed out (method=%s, id=%s)", method, id)
	}
}

// readLoop reads messages from the WebSocket and dispatches them.
func (b *Bridge) readLoop(ctx context.Context) {
	defer close(b.done)
	defer b.connected.Store(false)

	for {
		b.mu.Lock()
		if b.conn != nil {
			b.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		}
		b.mu.Unlock()

		_, data, err := b.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				slog.Info("websocket connection closed")
			} else if ctx.Err() != nil {
				slog.Info("read loop context cancelled")
			} else {
				slog.Error("websocket read error", "error", err)
			}
			return
		}

		b.dispatchMessage(data)
	}
}

// dispatchMessage routes a raw WebSocket message to the appropriate handler.
func (b *Bridge) dispatchMessage(data []byte) {
	var typeCheck struct {
		Type  string `json:"type"`
		Event string `json:"event,omitempty"`
		ID    string `json:"id,omitempty"`
	}

	if err := json.Unmarshal(data, &typeCheck); err != nil {
		slog.Warn("failed to parse frame type", "error", err)
		return
	}

	switch typeCheck.Type {
	case FrameTypeResponse:
		var resp ResponseFrame
		if err := json.Unmarshal(data, &resp); err != nil {
			slog.Warn("failed to parse response frame", "error", err)
			return
		}

		if ch, ok := b.pending.Load(resp.ID); ok {
			select {
			case ch.(chan *ResponseFrame) <- &resp:
			default:
				slog.Warn("pending response channel full, dropping", "id", resp.ID)
			}
		}

	case FrameTypeEvent:
		var event EventFrame
		if err := json.Unmarshal(data, &event); err != nil {
			slog.Warn("failed to parse event frame", "error", err)
			return
		}

		slog.Debug("received event", "event", event.Event)

		select {
		case b.eventCh <- &event:
		default:
			slog.Warn("event channel full, dropping event", "event", event.Event)
		}
	}
}
