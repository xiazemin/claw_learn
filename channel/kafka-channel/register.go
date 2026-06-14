package kafka

import (
	"encoding/json"
	"fmt"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
)

// ChannelFactory is the factory function type for creating Kafka channels.
// It matches the channels.ChannelFactory signature used by InstanceLoader.
func ChannelFactory(name string, creds json.RawMessage, cfgRaw json.RawMessage,
	msgBus *bus.MessageBus, pairingSvc interface{}) (channels.Channel, error) {

	var cfg Config

	// Parse credentials (bootstrap_servers, consumer_group, etc.)
	if len(creds) > 0 {
		if err := json.Unmarshal(creds, &cfg); err != nil {
			return nil, fmt.Errorf("kafka: failed to unmarshal credentials: %w", err)
		}
	}

	// Parse config overrides (dm_policy, group_policy, etc.)
	if len(cfgRaw) > 0 {
		var extra struct {
			DMPolicy    string `json:"dm_policy"`
			GroupPolicy string `json:"group_policy"`
			SessionMode string `json:"session_mode"`
		}
		if err := json.Unmarshal(cfgRaw, &extra); err != nil {
			return nil, fmt.Errorf("kafka: failed to unmarshal config: %w", err)
		}
		if extra.DMPolicy != "" {
			cfg.DMPolicy = extra.DMPolicy
		}
		if extra.GroupPolicy != "" {
			cfg.GroupPolicy = extra.GroupPolicy
		}
		if extra.SessionMode != "" {
			cfg.SessionMode = extra.SessionMode
		}
	}

	ch, err := New(cfg, msgBus)
	if err != nil {
		return nil, err
	}

	// Set the instance name (may differ from "kafka" for multi-instance)
	ch.SetName(name)

	return ch, nil
}

// RegisterFactory registers the Kafka channel factory with the InstanceLoader.
// Call this during gateway setup to enable DB-based Kafka channel instances.
//
// Usage in gateway_channels_setup.go:
//
//	kafkachannel.RegisterFactory(instanceLoader)
func RegisterFactory(loader *channels.InstanceLoader) {
	loader.RegisterFactory(ChannelType, ChannelFactory)
}
