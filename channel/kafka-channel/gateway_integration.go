package kafka

// This file shows how to integrate the Kafka channel into the GoClaw gateway.
// Copy the relevant snippets into the gateway setup code.
//
// This is NOT a standalone runnable file — it's a reference implementation
// for developers integrating the Kafka channel into the GoClaw codebase.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// ============================================================================
// Integration Point 1: Config-based registration
// ============================================================================
// Add this to cmd/gateway_channels_setup.go → registerConfigChannels()

func registerKafkaFromConfig(
	cfg *config.Config,
	channelMgr *channels.Manager,
	msgBus *bus.MessageBus,
) {
	// Add Kafka to the ChannelsConfig struct:
	//   type ChannelsConfig struct {
	//       ...
	//       Kafka KafkaConfig `json:"kafka"`
	//   }
	//
	// For this example, we use a simple struct:
	type KafkaChannelConfig struct {
		Enabled          bool     `json:"enabled"`
		BootstrapServers []string `json:"bootstrap_servers"`
		ConsumerGroup    string   `json:"consumer_group"`
		InboundTopics    []string `json:"inbound_topics"`
		OutboundTopic    string   `json:"outbound_topic"`
		DMPolicy         string   `json:"dm_policy"`
		GroupPolicy      string   `json:"group_policy"`
		SessionMode      string   `json:"session_mode"`
	}

	// Read from cfg (hypothetical path)
	var kafkaCfg KafkaChannelConfig
	// In practice, this would come from cfg.Channels.Kafka
	_ = kafkaCfg

	// Example config:
	kafkaCfg = KafkaChannelConfig{
		Enabled:          true,
		BootstrapServers: []string{"localhost:9092"},
		ConsumerGroup:    "goclaw-consumer",
		InboundTopics:    []string{"openclaw.inbound"},
		OutboundTopic:    "openclaw.outbound",
		DMPolicy:         "open",
		GroupPolicy:      "open",
		SessionMode:      "direct",
	}

	if !kafkaCfg.Enabled {
		return
	}

	// Convert to internal Config type
	internalCfg := Config{
		Enabled:          kafkaCfg.Enabled,
		BootstrapServers: kafkaCfg.BootstrapServers,
		ConsumerGroup:    kafkaCfg.ConsumerGroup,
		InboundTopics:    kafkaCfg.InboundTopics,
		OutboundTopic:    kafkaCfg.OutboundTopic,
		DMPolicy:         kafkaCfg.DMPolicy,
		GroupPolicy:      kafkaCfg.GroupPolicy,
		SessionMode:      kafkaCfg.SessionMode,
	}

	ch, err := New(internalCfg, msgBus)
	if err != nil {
		channelMgr.RecordFailure(ChannelType, "", err)
		slog.Error("failed to initialize kafka channel", "error", err)
		return
	}

	channelMgr.RegisterChannel(ChannelType, ch)
	slog.Info("kafka channel enabled (config)")
}

// ============================================================================
// Integration Point 2: DB Instance registration
// ============================================================================
// Add this to cmd/gateway_channels_setup.go → after instanceLoader creation

func registerKafkaFactoryExample(instanceLoader *channels.InstanceLoader) {
	// Register the factory
	RegisterFactory(instanceLoader)

	// This enables:
	//   1. Loading Kafka instances from the `channel_instances` DB table
	//   2. Dynamic reload via InstanceLoader.Reload()
	//   3. Multi-instance support (multiple Kafka channels with different configs)
}

// ============================================================================
// Integration Point 3: Config struct addition
// ============================================================================
// Add to internal/config/config.go in the ChannelsConfig struct:

/*
type ChannelsConfig struct {
	Telegram        TelegramConfig        `json:"telegram"`
	Discord         DiscordConfig         `json:"discord"`
	Slack           SlackConfig           `json:"slack"`
	Feishu          FeishuConfig          `json:"feishu"`
	WhatsApp        WhatsAppConfig        `json:"whatsapp"`
	Zalo            ZaloConfig            `json:"zalo"`
	ZaloPersonal    ZaloPersonalConfig    `json:"zalo_personal"`
	Kafka           KafkaConfig           `json:"kafka"`  // ADD THIS
}

type KafkaConfig struct {
	Enabled          bool       `json:"enabled"`
	BootstrapServers []string   `json:"bootstrap_servers"`
	ConsumerGroup    string     `json:"consumer_group"`
	InboundTopics    []string   `json:"inbound_topics"`
	OutboundTopic    string     `json:"outbound_topic"`
	DMPolicy         string     `json:"dm_policy"`
	GroupPolicy      string     `json:"group_policy"`
	SessionMode      string     `json:"session_mode"`
	SASL             SASLConfig `json:"sasl"`
	TLS              TLSConfig  `json:"tls"`
}
*/

// ============================================================================
// Integration Point 4: Type constant
// ============================================================================
// Add to internal/channels/channel.go:

/*
const (
	TypeBitrix24     = "bitrix24"
	TypeDiscord      = "discord"
	TypeFacebook     = "facebook"
	TypeFeishu       = "feishu"
	TypeKafka        = "kafka"     // ADD THIS
	TypePancake      = "pancake"
	TypeSlack        = "slack"
	TypeTelegram     = "telegram"
	TypeWhatsApp     = "whatsapp"
	TypeZaloOA       = "zalo_oa"
	TypeZaloPersonal = "zalo_personal"
)
*/

// ============================================================================
// Integration Point 5: Setup wizard (optional)
// ============================================================================
// Add to cmd/setup_channel.go → setupChannelStep()

func setupKafkaWizard() {
	// Add to the channelType options:
	//   channelOptions := []SelectOption[string]{
	//       {"Telegram", "telegram"},
	//       {"Discord", "discord"},
	//       {"Slack", "slack"},
	//       {"Kafka", "kafka"},  // ADD THIS
	//   }

	// Add credential prompts for Kafka:
	//   case "kafka":
	//       brokers, err := promptString("Bootstrap servers", "", "localhost:9092")
	//       if err != nil || brokers == "" { return }
	//       group, err := promptString("Consumer group", "", "goclaw-consumer")
	//       if err != nil || group == "" { return }
	//       topics, err := promptString("Inbound topics", "", "openclaw.inbound")
	//       if err != nil || topics == "" { return }
	//       creds["bootstrap_servers"] = strings.Split(brokers, ",")
	//       creds["consumer_group"] = group
	//       creds["inbound_topics"] = strings.Split(topics, ",")
}

// ============================================================================
// Integration Point 6: Webhook mode (optional)
// ============================================================================
// For HTTP-based message ingestion (alternative to long-polling), implement
// the WebhookChannel interface:

func webhookExample() {
	// The Kafka channel could also expose an HTTP endpoint for external systems
	// to push messages directly:
	//
	// POST /api/channels/kafka/inbound
	// Content-Type: application/json
	//
	// {
	//   "sender_id": "user-123",
	//   "content": "Hello from HTTP",
	//   "peer_kind": "direct"
	// }
	//
	// This is useful for webhook-based integrations where the external system
	// can't produce to Kafka directly.

	_ = fmt.Sprintf("webhook example placeholder")
}

// ============================================================================
// Helper: Convert internal Config from DB credentials
// ============================================================================

// ParseCredentials converts DB-stored credentials JSON to Config.
func ParseCredentials(creds json.RawMessage) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(creds, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse credentials: %w", err)
	}
	return cfg, nil
}

// ParseConfigJSON converts DB-stored config JSON to partial Config overrides.
func ParseConfigJSON(cfgJSON json.RawMessage, base *Config) error {
	if len(cfgJSON) == 0 {
		return nil
	}

	var overrides struct {
		DMPolicy    string `json:"dm_policy"`
		GroupPolicy string `json:"group_policy"`
		SessionMode string `json:"session_mode"`
	}

	if err := json.Unmarshal(cfgJSON, &overrides); err != nil {
		return err
	}

	if overrides.DMPolicy != "" {
		base.DMPolicy = overrides.DMPolicy
	}
	if overrides.GroupPolicy != "" {
		base.GroupPolicy = overrides.GroupPolicy
	}
	if overrides.SessionMode != "" {
		base.SessionMode = overrides.SessionMode
	}

	return nil
}

// _ = context.Background() // ensure import is used
var _ = context.Background
