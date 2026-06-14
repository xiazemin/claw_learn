package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds the bridge configuration.
type Config struct {
	// Goclaw gateway WebSocket URL (e.g., "ws://localhost:18790/ws")
	GoclawURL string `json:"goclaw_url"`

	// Gateway token for authentication
	GatewayToken string `json:"gateway_token"`

	// User ID for the WebSocket connection
	UserID string `json:"user_id"`

	// Agent key/ID to target (e.g., "agent:fox-spirit")
	AgentKey string `json:"agent_key"`

	// HTTP listen address for the bridge API
	ListenAddr string `json:"listen_addr"`

	// Session mode: "direct" (per-user) or "shared" (all users share one session)
	SessionMode string `json:"session_mode"`

	// StreamMode: "sse" (Server-Sent Events), "sync" (wait for full response), "ws" (WebSocket upgrade)
	StreamMode string `json:"stream_mode"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		GoclawURL:   "ws://localhost:18790/ws",
		GatewayToken: "",
		UserID:      "channel-bridge",
		AgentKey:    "",
		ListenAddr:  ":18800",
		SessionMode: "direct",
		StreamMode:  "sse",
	}
}

// LoadConfig loads config from a JSON file, falling back to env vars and defaults.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Try loading from file
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := json.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parse config file: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config file: %w", err)
		}
	}

	// Override with env vars
	if v := os.Getenv("GOCLAW_GATEWAY_URL"); v != "" {
		cfg.GoclawURL = v
	}
	if v := os.Getenv("GOCLAW_GATEWAY_TOKEN"); v != "" {
		cfg.GatewayToken = v
	}
	if v := os.Getenv("GOCLAW_USER_ID"); v != "" {
		cfg.UserID = v
	}
	if v := os.Getenv("GOCLAW_AGENT_KEY"); v != "" {
		cfg.AgentKey = v
	}
	if v := os.Getenv("GOCLAW_LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}

	return cfg, nil
}
