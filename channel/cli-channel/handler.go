package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Handler provides HTTP endpoints for the channel bridge.
type Handler struct {
	bridge *Bridge
	cfg    *Config
}

// NewHandler creates a new HTTP handler.
func NewHandler(bridge *Bridge, cfg *Config) *Handler {
	return &Handler{bridge: bridge, cfg: cfg}
}

// RegisterRoutes registers all HTTP routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", h.handleIndex)
	mux.HandleFunc("/chat", h.handleChat)
	mux.HandleFunc("/chat/stream", h.handleChatStream)
	mux.HandleFunc("/history", h.handleHistory)
	mux.HandleFunc("/abort", h.handleAbort)
	mux.HandleFunc("/sessions", h.handleSessions)
	mux.HandleFunc("/skills", h.handleSkills)
	mux.HandleFunc("/models", h.handleModels)
	mux.HandleFunc("/agent", h.handleAgent)
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/status", h.handleStatus)
}

// handleIndex shows usage information.
func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"service":   "goclaw-channel-bridge",
		"version":   "1.0.0",
		"connected": h.bridge.IsConnected(),
		"endpoints": map[string]string{
			"POST /chat":         "Send a message and get full response (sync)",
			"POST /chat/stream":  "Send a message and get streaming response (SSE)",
			"GET  /history":      "Get chat history (?sessionKey=xxx&limit=50)",
			"POST /abort":        "Abort running request",
			"GET  /sessions":     "List sessions",
			"GET  /skills":       "List available skills",
			"GET  /models":       "List available models",
			"GET  /agent":        "Get agent info (?agent_key=xxx)",
			"GET  /health":       "Health check",
			"GET  /status":       "Bridge status",
		},
	})
}

// ChatRequest is the JSON body for /chat and /chat/stream.
type ChatRequest struct {
	Message  string `json:"message"`
	Content  string `json:"content,omitempty"` // alias for message
	AgentID  string `json:"agent_id,omitempty"`
	AgentKey string `json:"agent_key,omitempty"` // alias for agent_id
}

// handleChat handles synchronous chat requests.
func (h *Handler) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	req, err := h.parseChatRequest(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	agentID := req.AgentID
	if agentID == "" {
		agentID = h.cfg.AgentKey
	}
	if agentID == "" {
		agentID = "default"
	}

	slog.Info("chat request", "message_len", len(req.Message), "agent_id", agentID)
	start := time.Now()

	content, err := h.bridge.ChatSend(r.Context(), req.Message, agentID)
	if err != nil {
		slog.Error("chat failed", "error", err, "elapsed", time.Since(start))
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	slog.Info("chat response", "content_len", len(content), "elapsed", time.Since(start))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"content": content,
		"elapsed": time.Since(start).String(),
	})
}

// handleChatStream handles streaming chat requests via Server-Sent Events.
func (h *Handler) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	req, err := h.parseChatRequest(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	agentID := req.AgentID
	if agentID == "" {
		agentID = h.cfg.AgentKey
	}
	if agentID == "" {
		agentID = "default"
	}

	slog.Info("stream request", "message_len", len(req.Message), "agent_id", agentID)

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ctx := r.Context()
	chunks, errCh := h.bridge.ChatStream(ctx, req.Message, agentID)

	for {
		select {
		case <-ctx.Done():
			h.writeSSE(w, "done", `{"done":true}`)
			flusher.Flush()
			return

		case chunk, ok := <-chunks:
			if !ok {
				h.writeSSE(w, "done", `{"done":true}`)
				flusher.Flush()
				return
			}
			data, _ := json.Marshal(map[string]string{"content": chunk})
			h.writeSSE(w, "chunk", string(data))
			flusher.Flush()

		case err, ok := <-errCh:
			if ok && err != nil {
				data, _ := json.Marshal(map[string]string{"error": err.Error()})
				h.writeSSE(w, "error", string(data))
				flusher.Flush()
				return
			}
		}
	}
}

// handleHistory handles history requests.
func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	sessionKey := r.URL.Query().Get("sessionKey")
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}

	payload, err := h.bridge.History(r.Context(), sessionKey, limit)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"payload": payload,
	})
}

// handleAbort handles abort requests.
func (h *Handler) handleAbort(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionKey := r.URL.Query().Get("sessionKey")
	if err := h.bridge.Abort(r.Context(), sessionKey); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// handleSessions handles session list requests.
func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	payload, err := h.bridge.SessionsList(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"payload": payload,
	})
}

// handleSkills handles skill list requests.
func (h *Handler) handleSkills(w http.ResponseWriter, r *http.Request) {
	payload, err := h.bridge.SkillsList(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"payload": payload,
	})
}

// handleModels handles model list requests.
func (h *Handler) handleModels(w http.ResponseWriter, r *http.Request) {
	payload, err := h.bridge.ModelsList(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"payload": payload,
	})
}

// handleAgent handles agent info requests.
func (h *Handler) handleAgent(w http.ResponseWriter, r *http.Request) {
	agentKey := r.URL.Query().Get("agent_key")
	payload, err := h.bridge.AgentInfo(r.Context(), agentKey)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"payload": payload,
	})
}

// handleHealth checks both the bridge and goclaw health.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := map[string]any{
		"bridge":    "ok",
		"connected": h.bridge.IsConnected(),
	}

	if h.bridge.IsConnected() {
		if payload, err := h.bridge.Health(r.Context()); err == nil {
			status["goclaw"] = "ok"
			status["goclaw_detail"] = payload
		} else {
			status["goclaw"] = "error"
			status["goclaw_error"] = err.Error()
		}
	} else {
		status["goclaw"] = "disconnected"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleStatus returns bridge status.
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"service":     "goclaw-channel-bridge",
		"version":     "1.0.0",
		"connected":   h.bridge.IsConnected(),
		"goclaw_url":  h.cfg.GoclawURL,
		"agent_key":   h.cfg.AgentKey,
		"user_id":     h.cfg.UserID,
		"listen_addr": h.cfg.ListenAddr,
	})
}

// --- Helpers ---

func (h *Handler) parseChatRequest(r *http.Request) (*ChatRequest, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB limit
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	// Support both "message" and "content" fields
	if req.Message == "" {
		req.Message = req.Content
	}
	if req.AgentID == "" {
		req.AgentID = req.AgentKey
	}

	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		return nil, fmt.Errorf("message (or content) is required")
	}

	return &req, nil
}

func (h *Handler) writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"ok":    false,
		"error": msg,
	})
}

func (h *Handler) writeSSE(w http.ResponseWriter, event string, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
}
