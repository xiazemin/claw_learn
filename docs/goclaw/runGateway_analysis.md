# `runGateway()` 深度解析

> 基于 `cmd/gateway.go` 及相关源码的逐行分析，覆盖执行步骤、外部组件依赖、协程清单、channel 数据流。

---

## 1. 执行步骤总览（按调用顺序）

`runGateway()` 是 goclaw 网关的核心启动函数，约 400 行代码，可分为 **12 个阶段**：

```
┌─────────────────────────────────────────────────────────────┐
│  Phase 1:  日志与配置加载                                      │
│  Phase 2:  核心组件创建（MessageBus、DomainEventBus）            │
│  Phase 3:  Provider 注册与模型解析                              │
│  Phase 4:  工具注册（Tools、Sandbox、Browser、MCP）              │
│  Phase 5:  存储层与 Tracing 初始化                              │
│  Phase 6:  记忆与巩固管线（Consolidation Pipeline）               │
│  Phase 7:  Vault 富化 Worker                                   │
│  Phase 8:  Agent、Skills、Subagent 系统                        │
│  Phase 9:  Gateway Server 与 HTTP/RPC 注册                    │
│  Phase 10: Channel 管理器与消息通道路由                          │
│  Phase 11: Scheduler（Lane-based）+ Cron + Heartbeat           │
│  Phase 12: 生命周期管理（Consumer、Shutdown、Server Start）       │
└─────────────────────────────────────────────────────────────┘
```

### Phase 1: 日志与配置加载

```go
// 1. 配置 slog 日志等级（DEBUG/INFO/WARN/ERROR）
logLevel := slog.LevelInfo
if verbose { logLevel = slog.LevelDebug }
// 环境变量覆盖：GOCLAW_LOG_LEVEL=debug|info|warn|error

// 2. 日志输出（stdout + 可选文件）
logOutput := gatewayLogOutput()  // GOCLAW_LOG_FILE 或 /var/log/goclaw/goclaw.log

// 3. 加载配置文件
cfg := config.Load(cfgPath)       // YAML/JSON 配置
config.ValidateGatewayAuth(cfg.Gateway)  // 安全校验

// 4. 版本覆盖（GOCLAW_EDITION=lite|standard）
edition.SetCurrent(edition.Lite)
```

**外部依赖**：文件系统（配置文件、日志目录）

---

### Phase 2: 核心组件创建

```go
// 1. MessageBus —— Channel 消息路由总线（双向 buffered channel）
msgBus := bus.New()
//   inbound:  make(chan InboundMessage, 1000)   // Channel → Consumer
//   outbound: make(chan OutboundMessage, 1000)   // Agent → Channel

// 2. DomainEventBus —— V3 巩固管线的事件总线（worker pool 模式）
domainBus := eventbus.NewDomainEventBus(eventbus.Config{
    QueueSize:   1000,
    WorkerCount: 2,          // 2 个 worker goroutine
})
domainBus.Start(ctx)         // 启动 2 个 goroutine
defer domainBus.Drain(10s)   // 优雅关闭
```

**MessageBus 结构**：
```
MessageBus {
    inbound:  chan InboundMessage  (cap=1000)   ─── Channel → Consumer goroutine
    outbound: chan OutboundMessage (cap=1000)   ─── Agent → Channel dispatcher goroutine
    handlers: map[channel]MessageHandler        ─── Channel 发送器注册
    subscribers: map[id]EventHandler            ─── WebSocket 事件订阅
}
```

---

### Phase 3: Provider 注册与模型解析

```go
// 模型注册表（前向兼容解析器）
modelReg := providers.NewInMemoryRegistry()
modelReg.RegisterResolver("anthropic", &providers.AnthropicForwardCompat{})
modelReg.RegisterResolver("openai", &providers.OpenAIForwardCompat{})

// Provider 注册表
providerRegistry := providers.NewRegistry(store.TenantIDFromContext)
registerProviders(providerRegistry, cfg, modelReg)  // 从配置文件注册

// 后续从 DB 注册（覆盖配置文件）
registerProvidersFromDB(providerRegistry, pgStores.Providers, ...)
```

**外部依赖**：PostgreSQL（providers 表）

---

### Phase 4: 工具注册

```go
toolsReg, execApprovalMgr, mcpMgr, sandboxMgr, browserMgr,
    webFetchTool, ttsTool, audioMgr, permPE, toolPE,
    dataDir, agentCfg := setupToolRegistry(cfg, workspace, providerRegistry)
```

这一步注册了所有 Agent 可调用的工具，包括：
- **exec** (代码执行) + **sandbox** (Docker 沙箱)
- **web_fetch** (网页抓取)
- **read_image** (图片读取)
- **message** (跨 Channel 发消息)
- **spawn** (子 Agent 生成)
- **MCP bridge** (连接外部 MCP 服务器)

**外部依赖**：Docker (sandbox)、网络 (web_fetch, MCP)

---

### Phase 5: 存储层与 Tracing

```go
pgStores, traceCollector, snapshotWorker := setupStoresAndTracing(cfg, dataDir, msgBus)
// pgStores 包含：
//   Sessions, Agents, Memory, Episodic, KnowledgeGraph, Vault,
//   Cron, Skills, Hooks, Teams, Workstations, Providers, ConfigSecrets,
//   ChannelInstances, BitrixPortals, MCP, SubagentTasks, PendingMessages,
//   BrowserCookies, APIKeys, Tenants, SystemConfigs, Contacts, etc.
```

**外部依赖**：PostgreSQL (所有 pgStores)、可选 Redis (cache)、可选 OTel (tracing)

---

### Phase 6: 记忆巩固管线（Consolidation Pipeline）

```go
// V3: session.completed → episodic → semantic(KG) → dreaming
if pgStores.Episodic != nil && bgProvider != nil {
    kgExtractor := kg.NewExtractor(bgProvider, bgModel, 0)
    
    cleanupConsolidation := consolidation.Register(consolidation.ConsolidationDeps{
        EpisodicStore: pgStores.Episodic,
        MemoryStore:   pgStores.Memory,
        KGStore:       pgStores.KnowledgeGraph,
        SessionStore:  pgStores.Sessions,
        EventBus:      domainBus,           // V3 domain event bus
        // ...
    })
}
```

**事件链路**：
```
session.completed ──→ episodicWorker.Handle()
    ↓ (publishes episodic.created)
episodic.created  ──→ semanticWorker.Handle()  [提取 KG 实体]
                  ──→ dreamingWorker.Handle()  [合成语义记忆]
    ↓ (publishes entity.upserted)
entity.upserted   ──→ dedupWorker.Handle()     [实体去重]
```

---

### Phase 7: Vault 富化 Worker

```go
cleanupVaultEnrich, ep, ew := vault.RegisterEnrichWorker(vault.EnrichWorkerDeps{
    VaultStore: pgStores.Vault,
    EventBus:   domainBus,    // 订阅 vault.doc_upserted
    MsgBus:     msgBus,       // 广播进度事件到 WebSocket
})
```

Vault Worker 为上传的文档生成摘要、嵌入向量和自动链接。通过 `semaphore.Weighted(3)` 限制并发 LLM 调用。

---

### Phase 8: Agent、Skills、Subagent 系统

```go
// Agent Router —— 延迟从 DB 解析 Agent
agentRouter := agent.NewRouter()
// agent 延迟创建，不预先加载

// Subagent 系统
subagentMgr := setupSubagents(...)

// Skills 系统
skillsLoader, skillSearchTool, globalSkillsDir, bundledSkillsDir, builtinSkillsDir :=
    setupSkillsSystem(cfg, workspace, dataDir, pgStores, toolsReg, providerRegistry, msgBus)

// Skills 目录监听
skillsWatcher, _ := skills.NewWatcher(skillsLoader)
skillsWatcher.Start(ctx)   // 启动 goroutine 监听目录变化
```

---

### Phase 9: Gateway Server 与 HTTP/RPC 注册

```go
// 创建 Gateway Server
server := gateway.NewServer(cfg, msgBus, agentRouter, pgStores.Sessions, toolsReg)

// HTTP 端点
server.BuildMux() →
    /ws                    WebSocket 连接
    /health                健康检查
    /v1/chat/completions   OpenAI 兼容 API
    /v1/responses          OpenResponses 协议
    /v1/tools/invoke       直接工具调用
    /mcp/bridge            MCP 桥接
    /                       嵌入式 Web UI

// RPC 方法注册
registerAllMethods(server, ...) →
    chat.*                 聊天操作
    agents.*               Agent CRUD
    sessions.*             会话管理
    cron.*                 定时任务
    skills.*               技能管理
    pairing.*              设备配对
    heartbeat.*            心跳
    hooks.*                钩子
    workstations.*         工作站
```

---

### Phase 10: Channel 管理器

```go
channelMgr := channels.NewManager(msgBus)

// 从 DB 加载 Channel 实例
instanceLoader := channels.NewInstanceLoader(pgStores.ChannelInstances, ...)
instanceLoader.RegisterFactory("telegram", telegram.Factory...)
instanceLoader.RegisterFactory("discord", discord.Factory...)
instanceLoader.RegisterFactory("feishu", feishu.Factory...)
instanceLoader.RegisterFactory("whatsapp", whatsapp.Factory...)
instanceLoader.RegisterFactory("slack", slackchannel.Factory...)
instanceLoader.RegisterFactory("bitrix24", bitrix24.Factory...)
// ... 共 10+ 种 Channel

instanceLoader.LoadAll(ctx)

// 启动所有 Channel
channelMgr.StartAll(ctx)  // 启动每个 Channel + outbound dispatcher goroutine
```

---

### Phase 11: Scheduler + Cron + Heartbeat

```go
// Lane-based Scheduler（匹配 TS CommandLane 模式）
sched := scheduler.NewScheduler(
    []LaneConfig{
        {Name: "main",     Concurrency: 30},  // 默认对话
        {Name: "subagent", Concurrency: 50},  // 子 Agent
        {Name: "team",     Concurrency: 100}, // 团队任务
        {Name: "cron",     Concurrency: 30},  // 定时任务
    },
    DefaultQueueConfig(),   // queue 模式, cap=10, debounce=800ms
    makeSchedulerRunFunc(agentRouter, cfg),
)

// Cron + Heartbeat ticker
heartbeatTicker := startCronAndHeartbeat(...)
```

**Scheduler 结构**：
```
Scheduler {
    lanes: LaneManager {
        "main":     Lane{sem: chan struct{}(30)}
        "subagent": Lane{sem: chan struct{}(50)}
        "team":     Lane{sem: chan struct{}(100)}
        "cron":     Lane{sem: chan struct{}(30)}
    }
    sessions: map[sessionKey]*SessionQueue  // 每个 session 一个队列
    runFn:    RunFunc                       // 实际执行 agent loop
}
```

---

### Phase 12: 生命周期管理

```go
deps.runLifecycle(ctx, cancel, lifecycleDeps{...})
```

`runLifecycle` 内部做了以下事情：

1. **Config 热重载订阅**（quota、cron、web_fetch、shell_deny、TTS）
2. **Inbound Consumer 启动** —— `go consumeInboundMessages(ctx, ...)`
3. **Webhook Worker 启动** —— `go ww.Run(workerCtx)`
4. **Task Recovery Ticker 启动**
5. **Signal Handler 启动** —— `go func() { sig := <-sigCh ... }()`
6. **Server 启动** —— `server.Start(ctx)` → `ListenAndServe()`

---

## 2. 外部组件依赖

| 组件 | 用途 | 必需/可选 |
|------|------|-----------|
| **PostgreSQL** | 所有持久化存储（sessions, agents, memory, vault, cron, skills...） | 必需 |
| **Redis** | 缓存加速（需 `-tags redis` 编译） | 可选 |
| **Docker** | Sandbox 代码执行隔离 | 可选 |
| **OTel Collector** | 分布式追踪（需 `-tags otel` 编译） | 可选 |
| **Tailscale** | 安全隧道访问（需 `-tags tsnet` 编译） | 可选 |
| **LLM Provider** | OpenAI / Anthropic API 调用 | 必需 |
| **Telegram/Discord/Feishu/Slack/WhatsApp/Bitrix24** | IM Channel 连接 | 可选 |
| **ElevenLabs** | TTS 语音合成 | 可选 |
| **文件系统** | workspace、配置文件、日志 | 必需 |

---

## 3. 协程（Goroutine）清单

以下是 `runGateway()` 启动的所有协程：

### 基础设施协程

| # | 协程 | 来源 | 作用 | 生命周期 |
|---|------|------|------|----------|
| 1 | `domainBus.worker()` × 2 | `domainBus.Start(ctx)` | 处理 V3 领域事件 | 直到 `Drain()` |
| 2 | `updateChecker` goroutine | `server.StartUpdateChecker(ctx)` | 定期检查 GitHub 新版本 | 直到 ctx cancel |
| 3 | `httpServer.ListenAndServe()` | `server.Start(ctx)` | HTTP/WS 监听 | 直到 ctx cancel + shutdown |
| 4 | httpServer shutdown goroutine | `server.Start(ctx)` 内部 | 监听 ctx.Done 执行优雅关闭 | 直到 shutdown 完成 |
| 5 | Signal handler goroutine | `runLifecycle` | 监听 SIGINT/SIGTERM | 直到收到信号 |

### 消息处理协程

| # | 协程 | 来源 | 作用 | 生命周期 |
|---|------|------|------|----------|
| 6 | `consumeInboundMessages()` | `runLifecycle` | 消费 inbound channel → 路由到 scheduler | 直到 ctx cancel |
| 7 | `channelMgr.dispatchOutbound()` | `channelMgr.StartAll(ctx)` | 消费 outbound channel → 发送到 Channel | 直到 ctx cancel |
| 8 | `channel.Start()` × N | `channelMgr.StartAll(ctx)` | 每个 Channel 一个连接协程 | 直到 ctx cancel |

### 后台工作协程

| # | 协程 | 来源 | 作用 | 生命周期 |
|---|------|------|------|----------|
| 9 | `ww.Run()` (Webhook Worker) | `runLifecycle` | 处理异步 webhook 回调 | 直到 ctx cancel |
| 10 | `taskTicker` goroutine | `runLifecycle` | 定期重派发卡住的团队任务 | 直到 Stop() |
| 11 | `consolidation` pruning goroutine | `consolidation.Register()` | 每 6h 清理过期 episodic 摘要 | 直到 pruneStop |
| 12 | `skillsWatcher` goroutine | `runGateway` | 监听 skills 目录变化 | 直到 ctx cancel |

### 动态创建的协程（运行时按需）

| # | 协程 | 来源 | 作用 | 生命周期 |
|---|------|------|------|----------|
| 13 | `Lane.Submit()` goroutine | Scheduler.Lane.Submit | 每个 Agent run 一个 | 直到 run 完成 |
| 14 | `agent.Loop` goroutine | Scheduler.RunFunc | 实际的 Agent 推理循环 | 直到 run 完成 |
| 15 | `processNormalMessage()` goroutine | Consumer（escalation 消息） | 即时处理系统升级消息 | 直到处理完成 |

---

## 4. Channel 数据流详解

### 4.1 入站消息流（Channel → Agent → Response）

```
外部 IM 平台
    │
    ▼
┌─────────────────────────┐
│  Channel Implementation │  (Telegram/Discord/Feishu/...)
│  .Start() goroutine     │  监听平台 Webhook/Bot API
└───────────┬─────────────┘
            │ msgBus.PublishInbound(InboundMessage)
            ▼
┌─────────────────────────────────────────────────┐
│          MessageBus.inbound chan (cap=1000)      │
└───────────┬─────────────────────────────────────┘
            │ msgBus.ConsumeInbound(ctx)
            ▼
┌─────────────────────────────────────────────────┐
│        consumeInboundMessages() goroutine        │
│                                                  │
│  ┌──────────┐   dedup    ┌──────────┐           │
│  │ DedupCache│──────────→│ Debouncer│           │
│  └──────────┘            └────┬─────┘           │
│                               │ flush           │
│                               ▼                 │
│                    ┌─────────────────┐           │
│                    │processNormalMsg  │           │
│                    │  1. Quota check  │           │
│                    │  2. Agent resolve│           │
│                    │  3. Build request│           │
│                    └────────┬────────┘           │
└─────────────────────────────┼───────────────────┘
                              │ sched.Schedule(ctx, "main", req)
                              ▼
┌─────────────────────────────────────────────────┐
│              Scheduler                           │
│                                                  │
│  ┌──────────────────────────┐                    │
│  │ SessionQueue (per session)│                    │
│  │  - enqueue + wait turn    │                    │
│  └──────────┬───────────────┘                    │
│             │ Lane.Submit(ctx, fn)                │
│             ▼                                    │
│  ┌──────────────────────────┐                    │
│  │ Lane "main" (sem=30)     │                    │
│  │  go fn() ──────────────────────────┐          │
│  └────────────────────────────────────┼──────────┘
└───────────────────────────────────────┼──────────┘
                                        │
                                        ▼
┌───────────────────────────────────────────────────┐
│            Agent.Run() (Loop goroutine)            │
│                                                    │
│  1. Load session history                           │
│  2. Build system prompt + context injection        │
│  3. Call LLM Provider (OpenAI/Anthropic)           │
│  4. Parse tool calls → execute tools               │
│  5. Loop: LLM → tools → LLM → ... (max iters)    │
│  6. Publish bus.Event{Agent, OutboundMessage}      │
│                                                    │
│  bus.Event {                                       │
│    Name: "agent"                                   │
│    Payload: OutboundMessage{                       │
│      Channel: "telegram"                           │
│      ChatID: "12345"                               │
│      Content: "这是 Agent 的回复..."               │
│    }                                               │
│  }                                                 │
└─────────────────────┬─────────────────────────────┘
                      │ eventPub.Broadcast(event)
                      ▼
┌───────────────────────────────────────────────────┐
│     MessageBus.subscribers (WebSocket clients)     │
│     + MessageBus.outbound chan                     │
│                                                    │
│  - WebSocket clients 收到实时推送                   │
│  - outbound chan 被 dispatchOutbound 消费           │
└─────────────────────┬─────────────────────────────┘
                      │ bus.SubscribeOutbound(ctx)
                      ▼
┌───────────────────────────────────────────────────┐
│   channelMgr.dispatchOutbound() goroutine          │
│                                                    │
│  msg.Channel = "telegram"                          │
│  → channel = m.channels["telegram"]                │
│  → channel.Send(ctx, msg)                          │
│    → Telegram Bot API: sendMessage / editMessage   │
└─────────────────────┬─────────────────────────────┘
                      │
                      ▼
                  外部 IM 平台
```

### 4.2 V3 记忆巩固管线数据流

```
Agent Loop 完成 session
    │
    │ domainBus.Publish(DomainEvent{
    │   Type: "session.completed",
    │   Payload: SessionCompletedPayload{...}
    │ })
    ▼
┌───────────────────────────────────────────────────┐
│         DomainEventBus.queue (cap=1000)            │
│         worker goroutine × 2                       │
└───────────┬───────────────────────────────────────┘
            │ dispatch → episodicWorker.Handle()
            ▼
┌───────────────────────────────────────────────────┐
│  episodicWorker                                    │
│  1. 读取 session 历史消息                           │
│  2. 调用 LLM 生成 episodic summary                 │
│  3. 存储到 episodic_summaries 表                    │
│  4. Publish(episodic.created)                      │
└───────────┬───────────────┬───────────────────────┘
            │               │
            ▼               ▼
┌──────────────────┐  ┌──────────────────┐
│ semanticWorker   │  │ dreamingWorker   │
│  提取 KG 实体    │  │  等待 debounce   │
│  存储到           │  │  (10min)         │
│  knowledge_graph │  │  收集 ≥5 条后    │
│                  │  │  调用 LLM 合成   │
│ Publish(         │  │  语义记忆         │
│ entity.upserted) │  │  存储到           │
└───────┬──────────┘  │  memory 表       │
        │             └──────────────────┘
        ▼
┌──────────────────┐
│ dedupWorker      │
│  实体去重检测     │
│  合并/标记重复    │
│  (管道终端)       │
└──────────────────┘
```

### 4.3 Vault 文档富化数据流

```
用户上传文档到 Vault
    │
    │ domainBus.Publish(DomainEvent{
    │   Type: "vault.doc_upserted"
    │ })
    ▼
┌───────────────────────────────────────────────────┐
│  vault.EnrichWorker                                │
│  sem.Acquire(3)     // 限制并发 3                   │
│                                                    │
│  1. 读取文档内容                                    │
│  2. 调用 LLM 生成 summary + key_entities           │
│  3. 计算 content_hash (SHA-256) 去重               │
│  4. 调用 LLM 分类文档 (embed + classify)           │
│  5. 搜索相似文档 → 自动建立链接                     │
│  6. 存储到 vault_documents + vault_links 表        │
│  7. msgBus.Broadcast → WebSocket 进度推送           │
└───────────────────────────────────────────────────┘
```

### 4.4 事件广播数据流（Bus → WebSocket Client）

```
内部事件源 (Agent/Cron/Channel/...)
    │
    │ msgBus.Broadcast(Event{Name: "chat", Payload: ...})
    ▼
┌───────────────────────────────────────────────────┐
│  MessageBus.subscribers map                        │
│  client1.SendEvent(event)                          │
│  client2.SendEvent(event)                          │
│  ...                                               │
└───────────┬───────────────────────────────────────┘
            │ WebSocket frame
            ▼
┌───────────────────────────────────────────────────┐
│  WebSocket Clients                                 │
│  - Web UI                                         │
│  - Desktop App                                    │
│  - CLI client                                     │
│  - Channel Instance (MCP bridge)                  │
└───────────────────────────────────────────────────┘
```

---

## 5. 关键 Channel 详解

### MessageBus.inbound (cap=1000)

- **写入方**: Channel 实现（Telegram/Discord/...）调用 `msgBus.PublishInbound()`
- **读取方**: `consumeInboundMessages()` goroutine 调用 `msgBus.ConsumeInbound()`
- **协议**: `InboundMessage` 结构体（Channel, SenderID, ChatID, Content, Media, TenantID, AgentID）

### MessageBus.outbound (cap=1000)

- **写入方**: Agent Loop 完成后通过 `eventPub.Broadcast()` → `msgBus.PublishOutbound()`
- **读取方**: `channelMgr.dispatchOutbound()` goroutine 调用 `msgBus.SubscribeOutbound()`
- **协议**: `OutboundMessage` 结构体（Channel, ChatID, Content, Media, Metadata, TenantID）

### DomainEventBus.queue (cap=1000)

- **写入方**: Agent Loop（session.completed）、Episodic Worker（episodic.created）、Vault Worker（vault.doc_upserted）
- **读取方**: 2 个 worker goroutine 轮询处理
- **协议**: `DomainEvent` 结构体（Type, TenantID, AgentID, Payload）

### ActiveRun.InjectCh (cap=16)

- **写入方**: WS RPC `chat.send` 调用 `router.InjectMessage()`
- **读取方**: Agent Loop 在 tool iteration 间隙检查 `InjectCh`

### SessionQueue 内部 channels

- **enqueueCh**: 新请求进入排队
- **runCh**: 被 scheduler 调度执行
- 实现 per-session 串行执行（`MaxConcurrent=1`）

---

## 6. Lane 并发模型

```
                    ┌──────────────────────────────┐
                    │        Scheduler              │
                    │                               │
                    │  SessionKey → SessionQueue     │
                    │                               │
    Inbound ────────┤  "telegram:123:alice"         │
    Request         │    → SessionQueue (FIFO)       │
                    │    → Lane.Submit("main")       │
                    │                               │
                    │  "discord:456:bob"             │
                    │    → SessionQueue (FIFO)       │
                    │    → Lane.Submit("main")       │
                    │                               │
                    │  "cron:heartbeat:789"          │
                    │    → SessionQueue              │
                    │    → Lane.Submit("cron")       │
                    └───────────┬───────────────────┘
                                │
            ┌───────────────────┼───────────────────┐
            │                   │                   │
            ▼                   ▼                   ▼
    ┌───────────────┐  ┌───────────────┐  ┌───────────────┐
    │ Lane "main"   │  │ Lane "team"   │  │ Lane "cron"   │
    │ sem=30 slots  │  │ sem=100 slots │  │ sem=30 slots  │
    │               │  │               │  │               │
    │ [run] [run]   │  │ [run] [run]   │  │ [run]         │
    │ [run] [wait]  │  │ [run] [run]   │  │ [wait]        │
    │  ...          │  │  ...          │  │  ...          │
    └───────────────┘  └───────────────┘  └───────────────┘
```

**Session 串行保证**: 同一个 `sessionKey` 的请求在 `SessionQueue` 中排队，`MaxConcurrent=1` 保证同一 session 同时只有一个 Agent Run 在执行。不同 session 可以并行执行（受 Lane 并发限制）。

---

## 7. 优雅关闭流程

```
SIGINT / SIGTERM
    │
    ▼
signal handler goroutine
    │
    ├─ BroadcastEvent(protocol.EventShutdown)   → 通知所有 WS 客户端
    ├─ channelMgr.StopAll()                     → 停止所有 Channel + outbound dispatcher
    ├─ pgStores.Cron.Stop()                     → 停止 cron 调度
    ├─ heartbeatTicker.Stop()                   → 停止心跳
    ├─ taskTicker.Stop()                        → 停止任务恢复
    ├─ webhookWorkerCancel()                    → 停止 webhook 回调
    ├─ close(auditCh)                           → 排空审计日志队列
    ├─ providerRegistry.Close()                 → 释放 Provider 资源
    ├─ permCache.Close()                        → 停止权限缓存扫描
    ├─ sandboxMgr.Stop() + ReleaseAll()         → 释放沙箱容器
    ├─ sched.Stop() + sleep(5s)                 → 排空活跃 run（5s 超时）
    └─ cancel()                                 → 通知所有 ctx.Done() 监听者
         │
         ▼
    server.Shutdown(5s timeout)                 → 优雅关闭 HTTP 服务器
    domainBus.Drain(10s)                        → 排空剩余领域事件
    defer: mcpMgr.Stop()                       → 停止 MCP 连接池
    defer: browserMgr.Close()                  → 关闭浏览器管理器
```

---

## 8. 快速理解与使用指南

### 作为开发者

1. **入口点**: `cmd/gateway.go` → `runGateway()`，约 400 行，按 Phase 顺序阅读
2. **消息理解**: 从 `bus.InboundMessage` 出发，追踪 `consumeInboundMessages` → `processNormalMessage` → `scheduler.Schedule` → `agent.Run`
3. **新 Channel 开发**: 实现 `channels.Channel` 接口，在 `gateway_channels_setup.go` 注册 Factory
4. **新工具开发**: 在 `internal/tools/` 下实现，通过 `tools.Registry.Register()` 注册
5. **V3 记忆管线**: 理解 `consolidation/workers.go` 中的事件订阅链路

### 作为运维

1. **启动**: `goclaw gateway`（需要 PostgreSQL，可选 Redis）
2. **配置**: `~/.goclaw/config.yaml` 或环境变量
3. **日志**: `GOCLAW_LOG_LEVEL=debug` 查看详细日志
4. **健康检查**: `GET /health` → `{"status":"ok","protocol":3}`
5. **优雅关闭**: 发送 `SIGINT`/`SIGTERM`，等待所有 run 完成
