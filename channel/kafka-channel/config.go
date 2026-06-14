package kafka

// Config holds the configuration for the Kafka channel.
type Config struct {
	Enabled          bool       `json:"enabled"`
	BootstrapServers []string   `json:"bootstrap_servers"`
	ConsumerGroup    string     `json:"consumer_group"`
	InboundTopics    []string   `json:"inbound_topics"`
	OutboundTopic    string     `json:"outbound_topic"`
	DMPolicy         string     `json:"dm_policy"`          // "open", "allowlist", "pairing", "disabled"
	GroupPolicy      string     `json:"group_policy"`       // "open", "allowlist", "disabled"
	SessionMode      string     `json:"session_mode"`       // "direct", "topic", "topic-peer"
	SASL             SASLConfig `json:"sasl"`
	TLS              TLSConfig  `json:"tls"`
}

// SASLConfig holds SASL authentication configuration.
type SASLConfig struct {
	Enabled   bool   `json:"enabled"`
	Mechanism string `json:"mechanism"` // "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"
	Username  string `json:"username"`
	Password  string `json:"password"`
}

// TLSConfig holds TLS configuration.
type TLSConfig struct {
	Enabled bool `json:"enabled"`
}

// InboundPayload is the expected JSON structure for inbound Kafka messages.
// Producers should send messages in this format to the inbound topic.
type InboundPayload struct {
	SenderID string            `json:"sender_id"`
	ChatID   string            `json:"chat_id"`
	Content  string            `json:"content"`
	PeerKind string            `json:"peer_kind"` // "direct" or "group"
	Metadata map[string]string `json:"metadata,omitempty"`
}

// OutboundPayload is the JSON structure for outbound Kafka messages.
// The channel produces messages in this format to the outbound topic.
type OutboundPayload struct {
	Content  string            `json:"content"`
	ChatID   string            `json:"chat_id"`
	Metadata map[string]string `json:"metadata,omitempty"`
	SentAt   string            `json:"sent_at"`
}
