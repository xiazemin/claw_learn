# GoClaw 全量文件-函数索引

> 项目共 **1299 个 Go 源文件**（不含 _test.go），按模块分层索引  
> 每个文件列出其 **导出函数** 及一句话功能说明

---

## 目录总览

```
goclaw/
├── cmd/                          # CLI 入口（cobra 命令）~60 文件
├── internal/
│   ├── agent/                    # Agent 核心循环与提示词 ~50 文件
│   ├── audio/                    # TTS/STT 多引擎 ~15 文件
│   ├── backup/                   # 数据库备份与恢复 ~18 文件
│   ├── bgalert/                  # 后台告警
│   ├── bootstrap/                # 引导文件系统
│   ├── bus/                      # 消息总线
│   ├── cache/                    # 缓存抽象（内存/Redis）
│   ├── channels/                 # 10+ IM 通道实现 ~30 文件
│   ├── config/                   # 配置加载与热重载
│   ├── consolidation/            # V3 记忆巩固管线
│   ├── cron/                     # 定时任务执行
│   ├── crypto/                   # AES 加密
│   ├── edition/                  # 版本标识
│   ├── eventbus/                 # 域事件总线
│   ├── gateway/                  # WebSocket Gateway 服务端
│   ├── heartbeat/                # 心跳调度
│   ├── hooks/                    # Hook 系统（拦截器）
│   ├── http/                     # HTTP API Handler 层 ~70 文件
│   ├── i18n/                     # 国际化
│   ├── knowledgegraph/           # 知识图谱提取
│   ├── logs/                     # 运行时日志
│   ├── mcp/                      # MCP 协议管理
│   ├── media/                    # 媒体文件存储
│   ├── memory/                   # 记忆自动注入
│   ├── oauth/                    # OAuth 登录（OpenAI）
│   ├── orchestration/            # 任务编排
│   ├── permissions/              # 权限策略
│   ├── pipeline/                 # Agent 运行管线
│   ├── providers/                # LLM Provider 适配层 ~45 文件
│   ├── sandbox/                  # Docker 沙箱管理
│   ├── scheduler/                # 调度器（Lane 并发控制）
│   ├── skills/                   # 技能系统 ~35 文件
│   ├── store/                    # 数据存储层 ~55 文件
│   │   ├── pg/                   # PostgreSQL 实现 ~80 文件
│   │   └── sqlitestore/          # SQLite 实现 ~60 文件
│   ├── tools/                    # 工具注册表 ~80 文件
│   ├── tracing/                  # 追踪与成本计算
│   └── tts/                      # TTS 管理器
├── extensions/                   # 可选扩展（Kafka/Chrome Cookie）
├── migrations/                   # 73 个 SQL 迁移文件
└── ui/                           # 前端（React）
```

---

## 1. `cmd/` — CLI 入口层

### agent.go
| 函数 | 作用 |
|------|------|
| `agentCmd()` | 创建 `goclaw agent` 命令组 |
| `agentListCmd()` | 列出所有 Agent |
| `runAgentList()` | 执行 Agent 列表查询 |
| `agentAddCmd()` | 交互式添加 Agent |
| `runAgentAdd()` | 执行添加 Agent |
| `fetchProviders()` | 从网关获取 Provider 列表 |
| `selectModel()` | 交互式选择模型 |
| `agentDeleteCmd()` | 删除 Agent |
| `runAgentDelete()` | 执行删除 Agent |

### agent_chat.go / agent_chat_client.go
| 函数 | 作用 |
|------|------|
| `agentChatCmd()` | 创建 `agent chat` 命令 |
| `runAgentChat()` | 执行 Agent 对话 |
| `isGatewayRunning()` | 检查网关是否运行中 |
| `runClientMode()` | WebSocket 客户端模式连接网关 |
| `wsConnect()` | 建立 WebSocket 连接 |
| `wsChatSend()` | 通过 WebSocket 发送消息并等待响应 |
| `handleCLIEvent()` | 处理 CLI 端事件帧 |

### gateway.go
| 函数 | 作用 |
|------|------|
| `gatewayLogOutput()` | 配置网关日志输出 |
| `runGateway()` | **核心**: 启动完整网关服务 |
| `resolveBackgroundProvider()` | 解析后台任务用的 LLM Provider |

### gateway_*.go（网关子模块，共 30+ 文件）
| 文件 | 核心函数 | 作用 |
|------|----------|------|
| `gateway_agents.go` | `resolveEmbeddingProvider()` | 解析嵌入向量 Provider |
| | `setupSubagents()` | 初始化子 Agent 系统 |
| | `setupTTS()` | 初始化 TTS 管理器 |
| `gateway_consumer.go` | `consumeInboundMessages()` | **核心**: 消费入站消息并调度 |
| `gateway_consumer_handlers.go` | `handleSubagentAnnounce()` | 处理子 Agent 通知 |
| | `handleTeammateMessage()` | 处理队友消息 |
| | `handleResetCommand()` | 处理会话重置 |
| `gateway_consumer_debounce.go` | `prepareInboundDebounceMessage()` | 入站消息去抖 |
| | `resolveInboundDebounceDelay()` | 计算去抖延迟 |
| `gateway_channels_setup.go` | `registerConfigChannels()` | 注册配置文件中的通道 |
| | `wireChannelRPCMethods()` | 注册通道 RPC 方法 |
| `gateway_methods.go` | `registerAllMethods()` | 注册全部 RPC 方法 |
| `gateway_http_handlers.go` | `wireHTTP()` | 注册 HTTP Handler |
| `gateway_http_wiring.go` | `wireHTTPHandlersOnServer()` | 将 Handler 挂载到 HTTP Server |
| `gateway_hooks.go` | `buildHookHandlers()` | 构建 Hook 处理器映射 |
| `gateway_cron.go` | `makeCronJobHandler()` | 创建 Cron 执行函数 |
| `gateway_heartbeat.go` | `startCronAndHeartbeat()` | 启动 Cron 和 Heartbeat 调度 |
| `gateway_lifecycle.go` | `runLifecycle()` | 运行优雅生命周期管理 |
| `gateway_events.go` | `wireEventSubscribers()` | 注册域事件订阅者 |
| `gateway_evolution_cron.go` | `runEvolutionCron()` | 运行 Agent 自进化定时任务 |
| `gateway_builtin_tools.go` | `seedBuiltinTools()` | 种子内建工具到数据库 |
| `gateway_providers.go` | `registerProviders()` | 注册 LLM Provider |
| | `registerProvidersFromDB()` | 从数据库加载 Provider |
| `gateway_stores_pg.go` | `setupStoresAndTracing()` | 初始化 PG 存储层 |
| `gateway_managed.go` | `wireExtras()` | 额外组件注入 |
| `gateway_export.go` | `RunGateway()` | 公开的网关启动入口 |

### 其他 cmd 文件
| 文件 | 核心函数 | 作用 |
|------|----------|------|
| `migrate.go` | `migrateUpCmd()`, `migrateDownCmd()` | 数据库迁移 |
| `onboard.go` | `runOnboard()` | 首次安装引导 |
| `backup.go` | `uploadBackupToS3()` | 上传备份到 S3 |
| `restore.go` | `restoreCmd()` | 从 S3 恢复 |
| `doctor.go` | `runDoctor()` | 系统健康检查 |
| `upgrade.go` | `runUpgrade()` | 版本升级检查 |
| `pairing.go` | `pairingApproveCmd()` | 设备配对审批 |
| `channels_cmd.go` | `channelsAddCmd()` | 添加通道实例 |
| `providers_cmd.go` | `providersAddCmd()` | 添加 LLM Provider |
| `cron_cmd.go` | `cronListCmd()`, `cronDeleteCmd()` | 管理定时任务 |
| `sessions_cmd.go` | `sessionsListCmd()`, `sessionsDeleteCmd()` | 管理会话 |
| `skills_cmd.go` | `skillsListCmd()` | 列出技能 |

---

## 2. `internal/agent/` — Agent 核心循环

### systemprompt.go / systemprompt_sections.go
| 函数 | 作用 |
|------|------|
| `BuildSystemPrompt()` | **核心**: 构建 16 层系统提示词 |

### loop.go / loop_*.go（20+ 文件）
| 文件 | 核心函数 | 作用 |
|------|----------|------|
| `loop_types.go` | `NewLoop()` | 创建 Agent 运行循环 |
| `loop_run.go` | *(Loop.Run)* | **核心**: 运行 Agent 主循环 |
| `loop_tools.go` | *(Loop.executeTools)* | 执行工具调用 |
| `loop_compact.go` | *(Loop.compact)* | 消息压缩（LLM 摘要化） |
| `loop_context.go` | *(Loop.buildContext)* | 构建 LLM 请求上下文 |
| `loop_history.go` | *(Loop.loadHistory)* | 加载会话历史 |
| `loop_history_skills.go` | *(Loop.injectSkills)* | 注入技能到提示词 |
| `loop_history_supplement.go` | *(Loop.supplementHistory)* | 补充上下文信息 |
| `loop_pipeline_adapter.go` | *(Loop.runPipeline)* | 运行 Agent 管线 |
| `loop_tracing.go` | `EstimateTokens()` | Token 估算 |
| `loop_input_media.go` | *(Loop.processInputMedia)* | 处理输入媒体文件 |
| `loop_mcp_user.go` | *(Loop.mcpUserCreds)* | 注入 MCP 用户凭证 |

### 其他 agent 文件
| 文件 | 核心函数 | 作用 |
|------|----------|------|
| `router.go` | `NewRouter()` | 创建 Agent 路由器（按 key 分发） |
| `resolver.go` | `NewManagedResolver()` | Agent 配置解析器 |
| `inject.go` | *(inject.go)* | 提示词注入（记忆、上下文） |
| `media.go` | *(media.go)* | 媒体处理 |
| `toolloop.go` | *(toolloop.go)* | 工具循环执行 |
| `intent_classify.go` | `ClassifyIntent()` | 用户意图分类 |
| | `FormatStatusReply()` | 格式化状态回复 |
| `title_generate.go` | `GenerateTitle()` | 自动生成会话标题 |
| `pruning.go` | `DefaultPruningValues()` | 默认裁剪配置 |
| `memoryflush.go` | `ResolveMemoryFlushSettings()` | 记忆刷盘设置 |
| `preview_prompt.go` | `BuildPreviewPrompt()` | 构建预览提示词 |
| `suggestion_engine.go` | `NewSuggestionEngine()` | 自进化建议引擎 |
| `evolution_guardrails.go` | `ApplySuggestion()` | 应用自进化建议 |
| `orchestration_mode.go` | `ResolveOrchestrationMode()` | 解析编排模式 |
| `sanitize.go` | `SanitizeAssistantContent()` | 清理助手输出 |
| | `StripConfigLeak()` | 防止配置泄露 |
| | `IsSilentReply()` | 检测静默回复 |
| `skill_slash_commands.go` | *(slash command matching)* | 斜杠命令匹配 |
| `extractive_memory.go` | `ExtractiveMemoryFallback()` | 提取式记忆回退 |

---

## 3. `internal/providers/` — LLM Provider 适配层

### registry.go
| 函数 | 作用 |
|------|------|
| `NewRegistry()` | 创建 Provider 注册表（按租户隔离） |

### openai.go / openai_*.go（10+ 文件）
| 函数 | 作用 |
|------|------|
| `NewOpenAIProvider()` | 创建 OpenAI 兼容 Provider |
| *(OpenAIProvider.Chat)* | OpenAI Chat Completions 调用 |
| *(OpenAIProvider.Stream)* | OpenAI 流式响应 |

### anthropic.go
| 函数 | 作用 |
|------|------|
| `NewAnthropicProvider()` | 创建 Anthropic Claude Provider |

### claude_cli.go / claude_cli_*.go
| 函数 | 作用 |
|------|------|
| `NewClaudeCLIProvider()` | 创建 Claude CLI 本地 Provider |
| `CheckClaudeAuthStatus()` | 检查 Claude CLI 认证状态 |
| `BuildCLIMCPConfigData()` | 构建 MCP 配置传递给 Claude CLI |
| `ResetCLISession()` | 重置 Claude CLI 会话 |

### codex.go / dashscope.go / vertex.go
| 函数 | 作用 |
|------|------|
| `NewCodexProvider()` | 创建 OpenAI Codex Provider |
| `NewDashScopeProvider()` | 创建阿里百炼 Provider |
| `NewVertexProvider()` | 创建 Google Vertex AI Provider |
| `NewACPProvider()` | 创建 ACP 协议 Provider |

### failover.go / retry.go / cooldown.go
| 函数 | 作用 |
|------|------|
| `RunWithFailover()` | 多 Provider 故障转移 |
| `RetryDo()` | 指数退避重试 |
| `NewCooldownTracker()` | Provider 冷却跟踪 |

### schema_*.go
| 函数 | 作用 |
|------|------|
| `CleanToolSchemas()` | 清理工具 Schema 以适配各 Provider |
| `NormalizeSchema()` | 标准化 JSON Schema |

### model_registry.go
| 函数 | 作用 |
|------|------|
| `NewInMemoryRegistry()` | 创建内存模型注册表 |
| `SeedDefaultModels()` | 注入默认模型列表 |

### model_fallback.go
| 函数 | 作用 |
|------|------|
| `NewModelFallbackProvider()` | 模型故障转移 Provider |

### middleware.go / middleware_*.go
| 函数 | 作用 |
|------|------|
| `ComposeMiddlewares()` | 组合请求中间件 |
| `CacheMiddleware()` | 缓存控制中间件 |
| `ServiceTierMiddleware()` | 服务层级中间件 |

### chatgpt_oauth_router.go
| 函数 | 作用 |
|------|------|
| `NewChatGPTOAuthRouter()` | ChatGPT OAuth 路由（按用户选择路由到 OpenAI 或 Anthropic） |

### error_classify.go
| 函数 | 作用 |
|------|------|
| `ClassifyHTTPError()` | 分类 HTTP 错误（可重试/不可重试/上下文溢出） |

---

## 4. `internal/tools/` — 工具注册表

### registry.go
| 函数 | 作用 |
|------|------|
| `NewRegistry()` | 创建工具注册表 |

### shell.go / exec_*.go
| 函数 | 作用 |
|------|------|
| `NewExecTool()` | 创建 Shell 执行工具 |
| `NewSandboxedExecTool()` | 沙箱化 Shell 工具 |
| `DefaultDenyPatterns()` | 默认禁止命令模式 |
| `NewExecApprovalManager()` | 命令审批管理器 |

### filesystem.go / filesystem_*.go
| 函数 | 作用 |
|------|------|
| `NewReadFileTool()` | 读文件工具 |
| `NewWriteFileTool()` | 写文件工具 |
| `NewListFilesTool()` | 列目录工具 |
| `NewEditTool()` | 精确文本编辑工具 |
| `NewSandboxedReadFileTool()` | 沙箱化读文件工具 |

### web_fetch.go / web_search.go
| 函数 | 作用 |
|------|------|
| `NewWebFetchTool()` | 网页抓取工具 |
| `NewWebSearchTool()` | 网页搜索工具（支持 Brave/DuckDuckGo/Tavily/Exa） |

### subagent*.go（8 文件）
| 函数 | 作用 |
|------|------|
| `NewSubagentManager()` | 创建子 Agent 管理器 |
| `NewSpawnTool()` | 子 Agent 创建工具 |
| `FilterDenyList()` | 过滤子 Agent 禁用列表 |

### team_tasks_*.go（8 文件）
| 函数 | 作用 |
|------|------|
| `NewTeamTasksTool()` | 团队任务管理工具 |
| `NewTeamToolManager()` | 团队工具管理器 |

### memory*.go
| 函数 | 作用 |
|------|------|
| `NewMemorySearchTool()` | 记忆搜索工具 |
| `NewMemoryGetTool()` | 记忆获取工具 |
| `NewMemoryInterceptor()` | 记忆拦截器（自动写入） |

### knowledge_graph.go
| 函数 | 作用 |
|------|------|
| `NewKnowledgeGraphSearchTool()` | 知识图谱搜索工具 |

### vault_*.go
| 函数 | 作用 |
|------|------|
| `NewVaultInterceptor()` | Vault 拦截器（自动追踪文件链接） |
| `NewVaultReadTool()` | Vault 读取工具 |
| `NewVaultSearchTool()` | Vault 搜索工具 |

### 其他工具
| 函数 | 作用 |
|------|------|
| `NewCronTool()` | 定时任务管理工具 |
| `NewHeartbeatTool()` | 心跳管理工具 |
| `NewTtsTool()` | 文字转语音工具 |
| `NewCreateImageTool()` | 图片生成工具 |
| `NewCreateVideoTool()` | 视频生成工具 |
| `NewCreateAudioTool()` | 音频生成工具 |
| `NewCreateForumTopicTool()` | Telegram 论坛话题创建 |
| `NewSessionsListTool()` | 会话列表工具 |
| `NewSessionsSendTool()` | 跨会话发消息工具 |
| `NewSkillSearchTool()` | 技能搜索工具 |
| `NewSkillManageTool()` | 技能管理工具 |
| `NewPublishSkillTool()` | 技能发布工具 |
| `NewUseSkillTool()` | 技能使用标记工具 |
| `NewDelegateTool()` | Agent 委派工具 |
| `NewMessageTool()` | 消息发送工具 |
| `NewDateTimeTool()` | 日期时间工具 |
| `NewWaitTool()` | 等待工具（延迟执行） |
| `NewWorkstationExecTool()` | 远程工作站执行工具 |
| `NewClaudeRemoteTool()` | Claude 远程执行工具 |

---

## 5. `internal/pipeline/` — Agent 运行管线

| 文件 | 核心函数 | 作用 |
|------|----------|------|
| `pipeline.go` | `NewPipeline()` | 创建管线（setup → iteration × N → finalize） |
| | `NewDefaultPipeline()` | 创建默认管线 |
| `run_state.go` | `NewRunState()` | 运行状态容器 |
| `message_buffer.go` | `NewMessageBuffer()` | 消息缓冲区管理 |
| `context_stage.go` | `NewContextStage()` | **Setup**: 构建系统提示词 + 加载上下文 |
| `think_stage.go` | `NewThinkStage()` | **Iteration**: 调用 LLM |
| `tool_stage.go` | `NewToolStage()` | **Iteration**: 执行工具调用 |
| `prune_stage.go` | `NewPruneStage()` | **Iteration**: 裁剪历史消息 |
| `memory_flush_stage.go` | `NewMemoryFlushStage()` | **Iteration**: 刷盘记忆到 PG |
| `observe_stage.go` | `NewObserveStage()` | **Iteration**: 观察与记录 |
| `checkpoint_stage.go` | `NewCheckpointStage()` | **Iteration**: 持久化检查点 |
| `finalize_stage.go` | `NewFinalizeStage()` | **Finalize**: 写入会话到 PG + 工具清理 |

---

## 6. `internal/sandbox/` — Docker 沙箱

| 文件 | 核心函数 | 作用 |
|------|----------|------|
| `sandbox.go` | `DefaultConfig()` | 默认沙箱配置 |
| | `ApplyExecOpts()` | 应用执行选项 |
| `docker.go` | `CheckDockerAvailable()` | 检查 Docker 是否可用 |
| | `NewDockerManager()` | 创建 Docker 沙箱管理器 |
| | *(DockerManager.Create)* | 创建沙箱容器 |
| | *(DockerManager.Exec)* | 在容器内执行命令 |
| | *(DockerManager.Destroy)* | 销毁容器 |
| | *(DockerManager.Prune)* | 清理闲置容器 |
| `docker_resolve.go` | *(image resolution)* | 沙箱镜像解析与构建 |
| `fsbridge.go` | `NewFsBridge()` | 宿主-容器文件桥接 |

---

## 7. `internal/scheduler/` — 调度器

| 文件 | 核心函数 | 作用 |
|------|----------|------|
| `scheduler.go` | `NewScheduler()` | 创建调度器 |
| `lanes.go` | `NewLane()` | 创建并发 Lane |
| | `NewLaneManager()` | 管理多个 Lane |
| | `DefaultLanes()` | 默认 4 Lane 配置 |
| `queue.go` | `NewSessionQueue()` | 创建 Per-Session 串行队列 |

---

## 8. `internal/bus/` — 消息总线

| 文件 | 核心函数 | 作用 |
|------|----------|------|
| `bus.go` | `NewMessageBus()` | 创建双向消息总线 |
| | *(MessageBus.Inbound)* | 入站消息 channel |
| | *(MessageBus.Outbound)* | 出站消息 channel |
| | *(MessageBus.Subscribe)* | 订阅广播事件 |
| | *(MessageBus.Broadcast)* | 广播域事件 |

---

## 9. `internal/channels/` — IM 通道

### 核心框架
| 函数 | 作用 |
|------|------|
| `NewManager()` | 创建通道管理器 |
| `NewBaseChannel()` | 通道基类 |
| `NewInstanceLoader()` | 从 DB 加载通道实例 |
| `ChunkMarkdown()` | 长消息分片 |
| `CompactGroup()` | 群聊消息压缩 |
| `ClassifyChannelError()` | 通道错误分类 |

### 各通道实现
| 目录 | 作用 |
|------|------|
| `telegram/` | Telegram Bot API（含语音防护） |
| `discord/` | Discord Bot |
| `slack/` | Slack Bot |
| `whatsapp/` | WhatsApp Business API |
| `feishu/` | 飞书机器人 |
| `facebook/` | Facebook Messenger |
| `bitrix24/` | Bitrix24 CRM |
| `zalo/` | Zalo 聊天 |
| `pancake/` | Pancake 平台 |

---

## 10. `internal/http/` — HTTP API Handler 层（70+ 文件）

### 认证与权限
| 函数 | 作用 |
|------|------|
| `InitGatewayToken()` | 初始化网关 Token |
| `ResolveAPIKey()` | 解析 API Key |
| `WebhookAuthMiddleware()` | Webhook 认证中间件 |

### 核心 Handler
| Handler | 函数 | 作用 |
|---------|------|------|
| `AgentsHandler` | `NewAgentsHandler()` | Agent CRUD API |
| `SessionsHandler` | `NewSessionsHandler()` | 会话管理 API |
| `ProvidersHandler` | `NewProvidersHandler()` | Provider 管理 API |
| `SkillsHandler` | `NewSkillsHandler()` | 技能管理 API |
| `MCPHandler` | `NewMCPHandler()` | MCP 服务器管理 API |
| `ChannelInstancesHandler` | `NewChannelInstancesHandler()` | 通道实例管理 API |
| `TracesHandler` | `NewTracesHandler()` | 追踪查询 API |
| `UsageHandler` | `NewUsageHandler()` | 使用量统计 API |
| `KnowledgeGraphHandler` | `NewKnowledgeGraphHandler()` | 知识图谱 API |
| `MemoryHandler` | `NewMemoryHandler()` | 记忆管理 API |
| `EpisodicHandler` | `NewEpisodicHandler()` | 情景记忆 API |
| `BuiltinToolsHandler` | `NewBuiltinToolsHandler()` | 内建工具管理 API |
| `VaultHandler` | `NewVaultHandler()` | Vault 知识库 API |
| `VaultGraphHandler` | `NewVaultGraphHandler()` | Vault 图谱 API |
| `BackupHandler` | `NewBackupHandler()` | 备份 API |
| `RestoreHandler` | `NewRestoreHandler()` | 恢复 API |
| `TTSHandler` | `NewTTSHandler()` | TTS API |
| `WebhooksAdminHandler` | `NewWebhooksAdminHandler()` | Webhook 管理 API |
| `WebhooksLLMHandler` | `NewWebhookLLMHandler()` | LLM Webhook 入口 |
| `WebhooksMessageHandler` | `NewWebhookMessageHandler()` | 消息 Webhook 入口 |
| `SecureCLIHandler` | `NewSecureCLIHandler()` | 安全 CLI 管理 API |
| `TenantsHandler` | `NewTenantsHandler()` | 租户管理 API |
| `SystemConfigsHandler` | `NewSystemConfigsHandler()` | 系统配置 API |
| `EvolutionHandler` | `NewEvolutionHandler()` | 自进化 API |
| `WorkstationsHandler` | `NewWorkstationsHandler()` | 工作站管理 API |
| `BrowserCookiesHandler` | `NewBrowserCookiesHandler()` | 浏览器 Cookie 管理 API |
| `ActivityHandler` | `NewActivityHandler()` | 活动日志 API |
| `APIKeysHandler` | `NewAPIKeysHandler()` | API Key 管理 API |
| `PackagesHandler` | `NewPackagesHandler()` | 技能包管理 API |
| `ChatCompletionsHandler` | `NewChatCompletionsHandler()` | OpenAI 兼容 API |
| `ResponsesHandler` | `NewResponsesHandler()` | OpenAI Responses API |
| `FilesHandler` | `NewFilesHandler()` | 文件服务 API |
| `AgentSummoner` | `NewAgentSummoner()` | Agent 召唤器（从用户消息创建临时 Agent） |

---

## 11. `internal/store/` — 存储接口层（55 文件）

### stores.go — 顶层容器
```go
type Stores struct {
    Sessions, Memory, Cron, Pairing, Skills, Agents,
    Providers, Tracing, MCP, ChannelInstances, ConfigSecrets,
    AgentLinks, Teams, BuiltinTools, PendingMessages,
    KnowledgeGraph, Contacts, Activity, Snapshots,
    BrowserCookies, SecureCLI, SecureCLIGrants, APIKeys,
    Heartbeats, ConfigPermissions, Tenants, SubagentTasks,
    Vault, Episodic, EvolutionMetrics, EvolutionSuggestions,
    BitrixPortals, Hooks, Webhooks, WebhookCalls,
    Workstations, WorkstationLinks, WorkstationPermissions,
    WorkstationActivity, UsageCaps
    // ... 42 个 Store 接口
}
```

### 各 Store 接口（每个文件一个）
| 文件 | 接口 | 核心方法 |
|------|------|----------|
| `session_store.go` | `SessionStore` | GetHistory, UpdateMessages, ListByTenant |
| `agent_store.go` | `AgentStore` | GetByKey, List, Create, Update, Delete |
| `memory_store.go` | `MemoryStore` | PutDocument, Search, ListChunks |
| `knowledge_graph_store.go` | `KnowledgeGraphStore` | UpsertEntity, Search, GetGraph |
| `episodic_store.go` | `EpisodicStore` | Upsert, Search, Promote |
| `skill_store.go` | `SkillStore` | CRUD + Search + Grant 管理 |
| `vault_store.go` | `VaultStore` | CRUD + Search + Backlinks |
| `tracing_store.go` | `TracingStore` | Trace/Span CRUD |
| `cron_store.go` | `CronStore` | Job CRUD + Schedule 管理 |
| `mcp_store.go` | `MCPServerStore` | MCP 服务器 CRUD + Grant |
| `team_store.go` | `TeamStore` | 团队/任务/评论/事件 CRUD |
| `provider_store.go` | `ProviderStore` | Provider CRUD + 凭证加密 |
| `config_secrets_store.go` | `ConfigSecretsStore` | 加密配置读写 |
| `secure_cli_store.go` | `SecureCLIStore` | 安全 CLI CRUD |
| `api_key_store.go` | `APIKeyStore` | API Key CRUD |
| `webhook_store.go` | `WebhookStore` | Webhook CRUD |
| `snapshot_store.go` | `SnapshotStore` | 使用量快照查询 |
| `usage_caps.go` | `UsageCapStore` | 配额控制 CRUD |
| `heartbeat_store.go` | `HeartbeatStore` | 心跳 CRUD + 调度查询 |
| `workstation_store.go` | `WorkstationStore` | 工作站 CRUD |
| `browser_cookie_store.go` | `BrowserCookieStore` | Cookie CRUD |

### context.go — Context 工具
```go
WithUserID, WithAgentID, WithSenderID, WithTenantID,
WithLocale, WithSharedMemory, WithShellDenyGroups, ...
// 30+ With*/From Context 函数
```

---

## 12. `internal/store/pg/` — PostgreSQL 实现（80 文件）

每个文件实现一个 Store 接口的 PG 版本：

| 文件 | 关键实现 |
|------|----------|
| `factory.go` | `NewPGStores()` — 创建全部 PG Store |
| `pool.go` | `OpenDB()` — PG 连接池 |
| `sessions.go` | 会话 CRUD（JSONB messages） |
| `agents.go` | Agent CRUD + 向量嵌入 |
| `memory_search.go` | **混合搜索**: FTS + pgvector |
| `memory_docs.go` | 记忆文档 CRUD |
| `knowledge_graph.go` | 知识图谱 CRUD + 混合搜索 |
| `kg_entities.go` | KG 实体 CRUD |
| `kg_relations.go` | KG 关系 CRUD |
| `kg_graph.go` | KG 图遍历 |
| `kg_traversal.go` | KG 多跳遍历 |
| `episodic_summaries.go` | 情景记忆 CRUD + 混合搜索 |
| `episodic_search.go` | FTS + Vector 情景搜索 |
| `skills.go` | 技能 CRUD |
| `skills_grants.go` | 技能授权 |
| `skills_embedding.go` | 技能向量嵌入 |
| `vault_documents.go` | Vault 文档 CRUD |
| `vault_links.go` | Vault 链接 CRUD |
| `vault_graph.go` | Vault 图查询 |
| `tracing_spans.go` | Span 写入 |
| `providers.go` | Provider CRUD（加密存储） |
| `mcp_servers.go` | MCP 服务器 CRUD |
| `mcp_user_credentials.go` | MCP 凭证（加密） |
| `channel_instances.go` | 通道实例 CRUD |
| `cron_crud.go` | Cron Job CRUD |
| `cron_exec.go` | Cron 执行记录 |
| `teams.go` | 团队 CRUD |
| `teams_tasks.go` | 团队任务 CRUD |
| `hooks.go` | Hook CRUD |
| `webhooks.go` | Webhook CRUD |
| `workstations.go` | 工作站 CRUD（加密） |

---

## 13. `internal/store/sqlitestore/` — SQLite 实现（60 文件）

结构与 PG 镜像，核心差异：
| 差异点 | 说明 |
|--------|------|
| `schema.go` + `schema.sql` | 自动建表（73 表） |
| 无向量列 | `embedding`/`tsvector` 省略，Go 侧计算 |
| FTS5 | 用 FTS5 虚拟表替代 PG FTS |
| 无原生数组 | JSON 数组存储替代 `text[]` |

---

## 14. `internal/mcp/` — MCP 协议

| 函数 | 作用 |
|------|------|
| `NewManager()` | 创建 MCP 管理器 |
| `NewBridgeServer()` | 创建 MCP Bridge Server |
| `NewBridgeTool()` | 将 MCP 工具桥接为内部工具 |
| `NewPool()` | MCP 连接池 |
| `DiscoverTools()` | 发现 MCP 工具 |
| `NewMCPToolSearchTool()` | MCP 工具搜索工具 |
| `NewStoreGrantChecker()` | Store-based Grant 检查 |
| `IsToolAllowed()` | 工具白名单/黑名单过滤 |
| `ValidateServerConfig()` | 验证 MCP 服务器配置 |

---

## 15. `internal/memory/` — 记忆系统

| 函数 | 作用 |
|------|------|
| `NewAutoInjector()` | 记忆自动注入器 |
| `ContentHash()` | 文本 SHA-256 |
| `ChunkText()` | 文本分块 |
| `NewOpenAIEmbeddingProvider()` | OpenAI 嵌入向量 Provider |
| `CosineSimilarity()` | 余弦相似度计算 |
| `DefaultMemoryConfig()` | 默认记忆配置 |

---

## 16. `internal/consolidation/` — 记忆巩固管线

| 函数 | 作用 |
|------|------|
| `Register()` | 注册巩固管线 Worker |
| `ComputeRecallScore()` | 计算情景记忆召回分数 |

Worker 类型：
- `episodicWorker`: session.completed → 情景摘要
- `semanticWorker`: 情景摘要 → KG 实体/关系
- `dreamingWorker`: 定期蒸馏（promote 低分记忆）
- `dedupWorker`: KG 实体去重

---

## 17. `internal/hooks/` — Hook 系统

| 函数 | 作用 |
|------|------|
| `NewStdDispatcher()` | 创建标准事件分发器 |
| `CompileMatcher()` | 编译工具名匹配器 |
| `CompileCELExpr()` | 编译 CEL 表达式 |
| `NewAuditWriter()` | 审计日志写入器 |
| `EmitHookSpan()` | 发射 Hook 追踪 Span |
| `SubscribeDelegateEvents()` | 订阅委派事件 |
| `DisableLegacyCommandHooks()` | 禁用旧版命令 Hook |

---

## 18. `internal/crypto/` — 加密

| 函数 | 作用 |
|------|------|
| `Encrypt()` | AES-256-GCM 加密 |
| `Decrypt()` | AES-256-GCM 解密 |

---

## 19. `internal/backup/` — 备份与恢复

| 函数 | 作用 |
|------|------|
| `Run()` | 执行完整备份 |
| `Restore()` | 执行完整恢复 |
| `DumpDatabase()` | pg_dump 数据库 |
| `RestoreDatabase()` | pg_restore 数据库 |
| `ArchiveDirectory()` | tar 归档目录 |
| `NewS3Client()` | 创建 S3 客户端 |
| `RunPreflight()` | 备份前检查 |
| `TenantBackup()` | 租户级备份 |
| `TenantRestore()` | 租户级恢复 |
| `TenantTables()` | 获取租户相关表定义 |
| `ExportTable()` | 导出单表 |
| `ImportTableRows()` | 导入表数据 |

---

## 20. 其他 internal 包

| 包 | 核心函数 | 作用 |
|----|----------|------|
| `config` | `Load()`, `Default()`, `Save()` | 配置加载/保存/热重载 |
| | `NewWatcher()` | 配置文件监控 |
| `cache` | `NewInMemoryCache()`, `NewRedisCache()` | 缓存抽象 |
| `eventbus` | `NewDomainEventBus()` | 域事件总线 |
| `permissions` | `NewPolicyEngine()` | 权限策略引擎 |
| `orchestration` | `CaptureFromRunResult()` | 任务编排结果转换 |
| `cron` | `NewService()`, `ExecuteWithRetry()` | Cron 执行服务 |
| `i18n` | `T()`, `IsSupported()` | 国际化翻译 |
| `edition` | `Current()`, `SetCurrent()` | 版本标识 |
| `knowledgegraph` | `NewExtractor()` | KG 实体提取 |
| | `JaroWinkler()` | 字符串相似度 |
| `media` | `NewStore()` | 媒体文件存储 |
| `tts` | *(Manager)* | TTS 管理器 |
| `logs` | *(runtime snapshotter)* | 运行时日志 |
| `bgalert` | *(background alert)* | 后台告警 |
| `oauth` | `StartLoginOpenAI()`, `LoginOpenAI()` | OpenAI OAuth |
| | `NewDBTokenSource()` | 数据库 Token 管理 |
| `bootstrap` | `EnsureWorkspaceFiles()` | 引导工作区文件 |
| | `LoadWorkspaceFiles()` | 加载工作区文件 |

---

## 21. `extensions/` — 可选扩展

| 扩展 | 文件 | 作用 |
|------|------|------|
| `kafka-channel` | `channel.go` | Kafka 消息通道 |
| | `internal/consumer/` | Kafka 消费者 |
| | `internal/producer/` | Kafka 生产者 |
| `chrome-selected-cookie-sync` | — | Chrome Cookie 同步 |

---

## 统计概览

| 模块 | 文件数 | 核心职责 |
|------|--------|----------|
| `cmd/` | ~60 | CLI 命令 + 网关启动编排 |
| `internal/agent/` | ~50 | Agent 循环 + 提示词 + 意图 |
| `internal/providers/` | ~45 | LLM 适配（OpenAI/Anthropic/Codex/Vertex/...） |
| `internal/tools/` | ~80 | 50+ 工具实现 |
| `internal/http/` | ~70 | HTTP API Handler |
| `internal/store/` | ~55 | 存储接口定义 |
| `internal/store/pg/` | ~80 | PostgreSQL 实现 |
| `internal/store/sqlitestore/` | ~60 | SQLite 实现 |
| `internal/channels/` | ~30 | 10+ IM 通道 |
| `internal/skills/` | ~35 | 技能系统 |
| `internal/pipeline/` | ~15 | 运行管线 |
| `internal/sandbox/` | ~5 | Docker 沙箱 |
| `internal/scheduler/` | ~4 | 调度器 |
| `internal/mcp/` | ~12 | MCP 协议 |
| `internal/consolidation/` | ~8 | 记忆巩固 |
| `internal/hooks/` | ~12 | Hook 拦截器 |
| `internal/backup/` | ~18 | 备份恢复 |
| 其他 | ~50 | 配置/缓存/加密/i18n/... |
| **总计** | **~1299** | — |
