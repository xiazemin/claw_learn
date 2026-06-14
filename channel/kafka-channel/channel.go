// Package kafka implements the channels.Channel interface for Apache Kafka.
//
// It allows GoClaw to consume messages from Kafka topics and produce replies
// to Kafka topics, enabling integration with event-driven architectures.
//
// Usage:
//
//	cfg := kafka.Config{
//	    Enabled:          true,
//	    BootstrapServers: []string{"localhost:9092"},
//	    ConsumerGroup:    "goclaw-consumer",
//	    InboundTopics:    []string{"openclaw.inbound"},
//	    OutboundTopic:    "openclaw.outbound",
//	    DMPolicy:         "open",
//	}
//	ch, err := kafka.New(cfg, msgBus)
//	if err != nil { ... }
//	channelMgr.RegisterChannel("kafka", ch)
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/IBM/sarama"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
)

// ChannelType is the channel type constant.
const ChannelType = "kafka"

// Channel implements the channels.Channel interface for Apache Kafka.
type Channel struct {
	*channels.BaseChannel

	config   Config
	consumer sarama.ConsumerGroup
	producer sarama.SyncProducer
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// New creates a new Kafka channel from the given configuration.
func New(cfg Config, msgBus *bus.MessageBus) (*Channel, error) {
	if len(cfg.BootstrapServers) == 0 {
		return nil, fmt.Errorf("kafka: bootstrap_servers is required")
	}

	// Build Sarama configuration
	saramaConfig := sarama.NewConfig()
	saramaConfig.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	saramaConfig.Consumer.Return.Errors = true
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Net.MaxOpenRequests = 1 // Required for SyncProducer

	// SASL authentication
	if cfg.SASL.Enabled {
		saramaConfig.Net.SASL.Enable = true
		saramaConfig.Net.SASL.User = cfg.SASL.Username
		saramaConfig.Net.SASL.Password = cfg.SASL.Password

		switch cfg.SASL.Mechanism {
		case "SCRAM-SHA-256":
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		case "SCRAM-SHA-512":
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		default:
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		}
	}

	// TLS
	if cfg.TLS.Enabled {
		saramaConfig.Net.TLS.Enable = true
	}

	// Create consumer group
	consumer, err := sarama.NewConsumerGroup(
		cfg.BootstrapServers,
		cfg.ConsumerGroup,
		saramaConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("kafka: failed to create consumer group: %w", err)
	}

	// Create sync producer
	producer, err := sarama.NewSyncProducer(cfg.BootstrapServers, saramaConfig)
	if err != nil {
		consumer.Close()
		return nil, fmt.Errorf("kafka: failed to create producer: %w", err)
	}

	ch := &Channel{
		BaseChannel: channels.NewBaseChannel(ChannelType, msgBus, nil),
		config:      cfg,
		consumer:    consumer,
		producer:    producer,
	}

	// Set default session mode
	if ch.config.SessionMode == "" {
		ch.config.SessionMode = "direct"
	}

	// Validate policy
	ch.ValidatePolicy(cfg.DMPolicy, cfg.GroupPolicy)

	ch.MarkRegistered("Kafka channel configured")

	slog.Info("kafka channel created",
		"bootstrap_servers", cfg.BootstrapServers,
		"consumer_group", cfg.ConsumerGroup,
		"inbound_topics", cfg.InboundTopics,
		"outbound_topic", cfg.OutboundTopic,
		"session_mode", cfg.SessionMode)

	return ch, nil
}

// Start begins consuming Kafka messages. It is non-blocking after initial setup.
func (c *Channel) Start(ctx context.Context) error {
	if c.IsRunning() {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.SetRunning(true)
	c.MarkHealthy("Connected to Kafka")

	// Start consumer group
	handler := &consumerGroupHandler{channel: c}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := c.consumer.Consume(ctx, c.config.InboundTopics, handler); err != nil {
					if ctx.Err() != nil {
						return
					}
					slog.Error("kafka consumer group error, retrying in 5s",
						"error", err,
						"topics", c.config.InboundTopics)
					time.Sleep(5 * time.Second)
				}
			}
		}
	}()

	// Drain consumer errors
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-c.consumer.Errors():
				if !ok {
					return
				}
				slog.Warn("kafka consumer error", "error", err)
			}
		}
	}()

	slog.Info("kafka channel started",
		"topics", c.config.InboundTopics,
		"group", c.config.ConsumerGroup)

	return nil
}

// Stop gracefully shuts down the Kafka channel.
func (c *Channel) Stop(ctx context.Context) error {
	if !c.IsRunning() {
		return nil
	}

	slog.Info("kafka channel stopping...")

	if c.cancel != nil {
		c.cancel()
	}

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		slog.Warn("kafka channel stop timed out")
	}

	if c.consumer != nil {
		if err := c.consumer.Close(); err != nil {
			slog.Error("failed to close kafka consumer", "error", err)
		}
	}

	if c.producer != nil {
		if err := c.producer.Close(); err != nil {
			slog.Error("failed to close kafka producer", "error", err)
		}
	}

	c.SetRunning(false)
	c.MarkStopped("Stopped")

	slog.Info("kafka channel stopped")
	return nil
}

// Send delivers an outbound message to the Kafka outbound topic.
func (c *Channel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if c.config.OutboundTopic == "" {
		slog.Debug("kafka: no outbound topic configured, skipping send")
		return nil
	}

	if !c.IsRunning() {
		return fmt.Errorf("kafka: channel is not running")
	}

	// Build outbound payload
	payload := OutboundPayload{
		Content:  msg.Content,
		ChatID:   msg.ChatID,
		Metadata: msg.Metadata,
		SentAt:   time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("kafka: failed to marshal outbound message: %w", err)
	}

	// Build Kafka producer message
	kafkaMsg := &sarama.ProducerMessage{
		Topic: c.config.OutboundTopic,
		Key:   sarama.StringEncoder(msg.ChatID),
		Value: sarama.ByteEncoder(data),
		Headers: []sarama.RecordHeader{
			{Key: []byte("source"), Value: []byte("goclaw")},
			{Key: []byte("channel"), Value: []byte(ChannelType)},
			{Key: []byte("chat_id"), Value: []byte(msg.ChatID)},
		},
	}

	partition, offset, err := c.producer.SendMessage(kafkaMsg)
	if err != nil {
		slog.Error("kafka: failed to send message",
			"topic", c.config.OutboundTopic,
			"chat_id", msg.ChatID,
			"error", err)
		return fmt.Errorf("kafka: failed to send message: %w", err)
	}

	slog.Debug("kafka: message sent",
		"topic", c.config.OutboundTopic,
		"partition", partition,
		"offset", offset,
		"chat_id", msg.ChatID,
		"content_len", len(msg.Content))

	return nil
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler for message processing.
type consumerGroupHandler struct {
	channel *Channel
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(
	sess sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	for msg := range claim.Messages() {
		start := time.Now()

		// Parse inbound payload
		var payload InboundPayload
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			slog.Warn("kafka: failed to unmarshal inbound message",
				"topic", msg.Topic,
				"partition", msg.Partition,
				"offset", msg.Offset,
				"error", err)
			sess.MarkMessage(msg, "") // Skip malformed messages
			continue
		}

		// Apply defaults
		if payload.SenderID == "" {
			payload.SenderID = fmt.Sprintf("kafka-%s-%d", msg.Topic, msg.Partition)
		}
		if payload.ChatID == "" {
			payload.ChatID = msg.Topic
		}
		if payload.PeerKind == "" {
			payload.PeerKind = "direct"
		}

		// Enrich metadata with Kafka context
		if payload.Metadata == nil {
			payload.Metadata = make(map[string]string)
		}
		payload.Metadata["kafka_topic"] = msg.Topic
		payload.Metadata["kafka_partition"] = fmt.Sprintf("%d", msg.Partition)
		payload.Metadata["kafka_offset"] = fmt.Sprintf("%d", msg.Offset)
		if msg.Key != nil {
			payload.Metadata["kafka_key"] = string(msg.Key)
		}
		if !msg.Timestamp.IsZero() {
			payload.Metadata["kafka_timestamp"] = msg.Timestamp.Format(time.RFC3339)
		}

		slog.Debug("kafka: message received",
			"topic", msg.Topic,
			"partition", msg.Partition,
			"offset", msg.Offset,
			"sender_id", payload.SenderID,
			"content_len", len(payload.Content),
			"elapsed_ms", time.Since(start).Milliseconds())

		// Publish to MessageBus via BaseChannel.HandleMessage
		h.channel.HandleMessage(
			payload.SenderID,
			payload.ChatID,
			payload.Content,
			nil, // media
			payload.Metadata,
			payload.PeerKind,
		)

		sess.MarkMessage(msg, "")
	}

	return nil
}
