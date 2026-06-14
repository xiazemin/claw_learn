# GoClaw Kafka Channel Extension
  extensions/kafka-channel/
  
Apache Kafka 渠道适配器，让 GoClaw 可以从 Kafka Topic 消费消息并将 AI 回复发送回 Kafka Topic。

## 架构

```
┌──────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Kafka Topic  │────▶│  Sarama Consumer  │────▶│  GoClaw Channel  │
│  (inbound)    │     │  Group            │     │  HandleMessage() │
└──────────────┘     └──────────────────┘     └────────┬────────┘
                                                        │
                                                        ▼
                                               ┌─────────────────┐
                                               │   MessageBus     │
                                               └────────┬────────┘
                                                        │
                                                        ▼
                                               ┌─────────────────┐
                                               │  Agent Pipeline  │
                                               │  (8-Stage Loop)  │
                                               └────────┬────────┘
                                                        │
                                                        ▼
┌──────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Kafka Topic  │◀────│  Sarama Sync      │◀────│  Channel.Send() │
│  (outbound)   │     │  Producer         │     │                  │
└──────────────┘     └──────────────────┘     └─────────────────┘
```

## 快速开始

### 1. 添加依赖

在 GoClaw 根目录执行：

```bash
go mod edit -replace github.com/nextlevelbuilder/goclaw/extensions/kafka-channel=./extensions/kafka-channel
go mod tidy
```

### 2. 配置

在 `config.json` 中添加 Kafka 渠道配置：

```json
{
  "channels": {
    "kafka": {
      "enabled": true,
      "bootstrap_servers": ["localhost:9092"],
      "consumer_group": "goclaw-consumer",
      "inbound_topics": ["openclaw.inbound"],
      "outbound_topic": "openclaw.outbound",
      "dm_policy": "open",
      "session_mode": "direct"
    }
  }
}
```

### 3. 编译运行

```bash
go build -o goclaw .
./goclaw start
```

## 消息格式

### 入站消息 (Kafka → GoClaw)

发送到 `inbound_topics` 的消息必须是 JSON 格式：

```json
{
  "sender_id": "user-123",
  "chat_id": "user-123",
  "content": "帮我查一下最近的订单状态",
  "peer_kind": "direct",
  "metadata": {
    "order_id": "ORD-2026001"
  }
}
```

字段说明：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `sender_id` | string | 否 | 发送者 ID，默认 `kafka-{topic}-{partition}` |
| `chat_id` | string | 否 | 会话 ID，默认为 Kafka topic 名 |
| `content` | string | 是 | 消息内容 |
| `peer_kind` | string | 否 | `direct`（默认）或 `group` |
| `metadata` | map | 否 | 自定义元数据，会透传到 Agent |

### 出站消息 (GoClaw → Kafka)

发送到 `outbound_topic` 的消息格式：

```json
{
  "content": "您的订单 #12345 状态为：已发货",
  "chat_id": "user-123",
  "metadata": {
    "order_id": "ORD-2026001"
  },
  "sent_at": "2026-06-10T10:30:00Z"
}
```

## 配置项

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | `false` | 是否启用 |
| `bootstrap_servers` | []string | - | Kafka Broker 地址列表 |
| `consumer_group` | string | - | 消费者组名称 |
| `inbound_topics` | []string | - | 入站 Topic 列表 |
| `outbound_topic` | string | - | 出站 Topic（空则不发送回复到 Kafka） |
| `dm_policy` | string | `open` | DM 策略：`open`/`allowlist`/`pairing`/`disabled` |
| `group_policy` | string | `open` | 群聊策略：`open`/`allowlist`/`disabled` |
| `session_mode` | string | `direct` | 会话隔离模式：`direct`/`topic`/`topic-peer` |
| `sasl.enabled` | bool | `false` | 是否启用 SASL 认证 |
| `sasl.mechanism` | string | `PLAIN` | SASL 机制：`PLAIN`/`SCRAM-SHA-256`/`SCRAM-SHA-512` |
| `sasl.username` | string | - | SASL 用户名 |
| `sasl.password` | string | - | SASL 密码 |
| `tls.enabled` | bool | `false` | 是否启用 TLS |

## 会话隔离模式

| 模式 | Session Key 格式 | 说明 |
|------|-----------------|------|
| `direct` | `agent:{id}:kafka:direct:{senderId}` | 每个 sender 独立会话（默认） |
| `topic` | `agent:{id}:kafka:group:{topic}` | 每个 Kafka Topic 共享一个会话 |
| `topic-peer` | `agent:{id}:kafka:group:{topic}:thread:{senderId}` | Topic + Sender 双重隔离 |

## 测试

使用 `kcat`（原 `kafkacat`）发送测试消息：

```bash
# 发送测试消息
kcat -b localhost:9092 -t openclaw.inbound -P \
  -k "user-123" \
  -H "content-type=application/json" \
  <<EOF
{"sender_id":"user-123","chat_id":"user-123","content":"你好，请介绍一下你自己","peer_kind":"direct"}
EOF

# 消费 AI 回复
kcat -b localhost:9092 -t openclaw.outbound -C -o beginning
```

## 多实例支持

通过 GoClaw 的 DB Instance 机制，可以在数据库中配置多个 Kafka 渠道实例：

```sql
INSERT INTO channel_instances (name, channel_type, credentials, config, agent_id, enabled)
VALUES (
  'kafka-orders',
  'kafka',
  '{"bootstrap_servers":["broker1:9092"],"consumer_group":"goclaw-orders","inbound_topics":["orders"],"outbound_topic":"orders-reply"}',
  '{"dm_policy":"open","session_mode":"topic"}',
  'default',
  true
);
```

## 文件结构

```
extensions/kafka-channel/
├── channel.go      # Channel 接口实现，生命周期管理
├── config.go       # 配置结构体定义
├── register.go     # 工厂注册（用于 DB Instance）
├── go.mod          # Go 模块定义
└── README.md       # 本文档
```

## 参考

- [GoClaw Channel 接口](../../internal/channels/channel.go)
- [GoClaw Session Key](../../internal/sessions/key.go)
- [GoClaw MessageBus](../../internal/bus/bus.go)
- [Sarama 文档](https://pkg.go.dev/github.com/IBM/sarama)
