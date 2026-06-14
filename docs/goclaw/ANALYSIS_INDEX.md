# GoClaw 项目全面分析索引

> 2026-06-12 会话产出 · 共 10 份分析文档，总计约 305KB

---

## 文档清单

| # | 文件名 | 大小 | 主题 |
|---|--------|------|------|
| 1 | `runGateway_analysis.md` | 31KB | runGateway 启动流程 |
| 2 | `sandbox_analysis.md` | 39KB | 沙箱全流程 |
| 3 | `prompt_context_rag_analysis.md` | 32KB | 提示词 / 上下文 / RAG |
| 4 | `skills_analysis.md` | 27KB | 技能系统 |
| 5 | `storage_analysis.md` | 33KB | 73 张表完整 Schema |
| 6 | `files_functions_index.md` | 30KB | 1299 个 Go 文件函数索引 |
| 7 | `api_interfaces_analysis.md` | 46KB | HTTP / WebSocket / MCP 接口 |
| 8 | `acp_protocol_analysis.md` | — | ACP 协议详解 |
| 9 | `a2a_vs_acp_analysis.md` | 16KB | A2A vs ACP 对比 |
| 10 | `protocols_technologies_overview.md` | 27KB | 全协议与技术栈 |

---

## 一、GoClaw 是什么

多租户 AI Agent 平台。Go 单二进制 (~25MB)，PostgreSQL，10+ 通道，20+ LLM Provider。

---

## 二、启动流程（runGateway_analysis.md）

12 个阶段：日志配置 → Provider 加载 → 工具注册(50+) → 存储初始化 → 记忆巩固管线 → Vault 富化 → Agent 路由 → Gateway HTTP Server(40+ 路由) → Channel 管理(10+) → Scheduler(4 Lane) → Consumer goroutine → Signal handler。

约 15 种协程：DomainEventBus workers, consumeInboundMessages, dispatchOutbound, Channel.Start, Scheduler Lane, Webhook Worker, Signal handler, HTTP server。

Channel 间数据流：
```
Channel → msgBus.inbound(1000) → Consumer → Scheduler → Agent Loop
                                                          │
WebSocket Client ← msgBus.subscribers ← eventPub ←──────┘
Channel ← msgBus.outbound(1000) ← dispatchOutbound ←─────┘
```

---

## 三、沙箱系统（sandbox_analysis.md）

**Docker 通信**：直接调用 Docker CLI（非 SDK）：
```
创建: docker run -d --name goclaw-sbx-xxx --read-only --cap-drop ALL --network none ... sleep infinity
执行: docker exec -w /workspace [-e KEY=VAL] <id> sh -c "command"
销毁: docker rm -f <id>
```

**三维隔离**：Mode(off/non-main/all) × Scope(session/agent/shared) × Workspace(none/ro/rw)

**状态保持**：`sleep infinity` 保持容器，文件系统在 Agent Run 间持久；闲置 >24h 或 >7 天自动清理。

**结果回传**：ExecResult → Result{ForLLM} → providers.Message{role:"tool"} → MessageBuffer → 下次 LLM 调用。

**上下文存储**：FinalizeStage → FlushPending() → PostgreSQL sessions.messages (JSONB)。

---

## 四、提示词 / 上下文 / RAG（prompt_context_rag_analysis.md）

**系统提示词 16 层**：Identity → Bootstrap → Persona → Tooling → Execution Bias → Tool Call Style → Credentialed CLI → Voice → Safety → Self-Evolution → Skills → MCP Tools → Delegation → Workspace → Team → User Identity → **[CACHE BOUNDARY]** → Time → Channel Hints → Extra Context → Project Context → SubAgent → Runtime → Recency Reinforcement → AutoInject L0 记忆。

**三级记忆 RAG**：
| 级别 | 存储 | 搜索 | 触发 |
|------|------|------|------|
| L0 Episodic | `episodic_summaries` | FTS(70%)+Vector(30%) | 自动注入 system prompt |
| L1 Semantic | `memory_chunks` | FTS(30%)+Vector(70%) | `memory_search` 工具 |
| L2 KG | `knowledge_graph` | 实体-关系查询 | `knowledge_graph_search` |

**向量化流程**：文档 → PutDocument → ChunkText → ContentHash(SHA-256) → 查 embedding_cache → OpenAI Embedding API(text-embedding-3-small, 1536 维) → 写 embedding_cache → INSERT memory_chunks ... embedding $1::vector。

---

## 五、技能系统（skills_analysis.md）

**双存储**：文件系统（SKILL.md + assets/）+ PostgreSQL（`skills` 表 + embedding vector）。

5 级优先级：`workspace/skills/ > .agents/skills/ > ~/.agents/skills/ > ~/.goclaw/skills/ > builtin`

**LLM 使用流程**：system prompt `<available_skills>` 摘要 → `skill_search`(BM25+pgvector) → `use_skill(name)` → `read_file(SKILL.md)`。

**权限**：private/internal/public + Grant 自动升降级 + 系统技能硬保护 + GuardSkillContent 安全扫描。

---

## 六、存储层（storage_analysis.md）

**73 张表**，按功能域分组：

| 功能域 | 表数 | 核心表 |
|--------|------|--------|
| 多租户 | 2 | `tenants`, `tenant_users` |
| Agent | 8 | `agents`, `agent_shares`, `agent_context_files`, ... |
| 团队 | 9 | `agent_teams`, `team_tasks`, `team_task_comments`, ... |
| 会话 | 1 | `sessions` |
| 三级记忆 | 7 | `memory_documents`, `memory_chunks`, `embedding_cache`, `episodic_summaries`, `kg_entities`, `kg_relations`, `kg_dedup_candidates` |
| Skills | 4 | `skills`, `skill_agent_grants`, `skill_user_grants`, `skill_tenant_configs` |
| Cron | 2 | `cron_jobs`, `cron_run_logs` |
| 可观测性 | 2 | `traces`, `spans` |
| MCP | 5 | `mcp_servers`, `mcp_agent_grants`, `mcp_user_grants`, `mcp_access_requests`, `mcp_user_credentials` |
| Channel | 3 | `channel_instances`, `channel_pending_messages`, `channel_contacts` |
| Vault | 2 | `vault_documents`, `vault_links` |
| 安全 CLI | 3 | `secure_cli_binaries`, `secure_cli_agent_grants`, `secure_cli_user_credentials` |
| Webhook | 2 | `webhooks`, `webhook_calls` |
| Workstation | 4 | `workstations`, `agent_workstation_links`, `workstation_permissions`, `workstation_activity` |
| Hooks | 3 | `hooks`, `hook_agents`, `hook_executions` |
| 其他 | 12 | `api_keys`, `heartbeats`, `config_secrets`, ... |

**PG 独有**：pgvector `vector(1536)` 列（7 张表）+ tsvector 全文搜索列（11 张表）+ GIN/HNSW 索引。

---

## 七、文件与函数索引（files_functions_index.md）

**1299 个 Go 源文件**，按 20+ 包组织：

| 模块 | 文件数 | 职责 |
|------|--------|------|
| `cmd/` | ~60 | CLI + 网关启动编排 |
| `internal/agent/` | ~50 | Agent 循环/提示词/意图 |
| `internal/providers/` | ~45 | LLM 适配层 |
| `internal/tools/` | ~80 | 50+ 工具 |
| `internal/http/` | ~70 | HTTP API Handler |
| `internal/store/` + pg + sqlite | ~195 | 存储层 |
| `internal/channels/` | ~30 | 10+ IM 通道 |
| `internal/skills/` | ~35 | 技能系统 |
| `internal/pipeline/` | ~15 | 运行管线（7 Stage）|
| `internal/backup/` | ~18 | 备份恢复 |
| 其他 | ~150 | MCP/Hook/缓存/加密/... |

---

## 八、接口全景（api_interfaces_analysis.md）

| 体系 | 数量 | 协议 | 用途 |
|------|------|------|------|
| HTTP REST | ~230 路由 | REST/JSON | 管理面 CRUD |
| WebSocket RPC | ~130 方法 | 帧协议 v3 (JSON-RPC 2.0) | 实时交互面 |
| MCP 工具 | ~50 工具 | MCP (JSON-RPC 2.0) | LLM 工具调用 |
| WebSocket 事件 | ~65 事件 | SSE-style push | 实时推送 |

**角色权限**：owner > admin > operator > viewer

**错误码**：UNAUTHORIZED / NOT_FOUND / AGENT_TIMEOUT / RESOURCE_EXHAUSTED / ...

---

## 九、协议与技术栈（protocols_technologies_overview.md + a2a_vs_acp_analysis.md）

### 协议分层

| 层级 | 协议 |
|------|------|
| Agent 间 | MCP (工具扩展) / ACP (子进程编排) / delegate (内部委派) |
| 实时通信 | WebSocket + JSON-RPC 2.0 / SSE |
| LLM API | OpenAI Chat Completions / Anthropic Messages / Responses / Embeddings |
| IM 通道 | Telegram Bot API / Discord Gateway / Slack Events API / WhatsApp WebSocket / 飞书 / Bitrix24 / Kafka |
| 追踪 | OpenTelemetry (OTLP/gRPC + HTTP) |
| 浏览器 | Chrome DevTools Protocol (go-rod) |
| 网络 | HTTP/1.1 + HTTP/2 / TLS / PROXY Protocol / Tailscale tsnet |
| 安全 | OAuth 2.0 / Bearer Token / HMAC-SHA256 / AES-256-GCM / SHA-256 |
| 规则 | CEL + 正则表达式 |
| 存储 | PostgreSQL + pgvector + FTS / SQLite + FTS5 / Redis / S3 / Docker |

### A2A vs ACP

| | A2A (Google) | ACP (BeeAI) |
|-|-------------|-------------|
| 传输 | HTTP + SSE | stdin/stdout |
| 拓扑 | Peer-to-Peer | Parent-Child |
| 发现 | `/.well-known/agent.json` | config 配置 |
| 会话 | Task-based (异步) | Session-based (同步) |
| 跨组织 | ✅ | ❌ |

GoClaw 使用 ACP 编排外部 Agent（Gemini CLI 等），未实现 A2A（delegate + agent_links 在内部实现类似功能）。

---

## 十、客户端与编译

### 三种客户端

| 形态 | 技术 | 用途 |
|------|------|------|
| **Web UI** | React + TypeScript + Vite | 标准版主力界面 |
| **Desktop** | Wails v2 (Go + React + SQLite) | GoClaw Lite 桌面版 |
| **CLI** | Cobra + bubbletea | 命令行管理 |

### 编译方式

```bash
# Web UI 版（生产推荐）
cd ui/web && pnpm install && pnpm build
cd ../.. && make build-full                    # → goclaw 二进制

# Desktop 版
make desktop-dev                              # 开发热重载
make desktop-build                            # → goclaw-lite.app/.exe
make desktop-dmg VERSION=0.1.0                # → .dmg (macOS)

# Docker 版
make up-build                                 # → Docker 容器
docker compose up -d                          # → 拉镜像运行

# 一键安装
curl -fsSL .../install.sh | bash              # Linux/macOS
```

### Build Tags

| Tag | 作用 |
|-----|------|
| `embedui` | 内嵌 Web UI |
| `sqliteonly` | SQLite 单机版（Desktop） |
| `redis` | Redis 缓存 |
| `otel` | OpenTelemetry |
| `tsnet` | Tailscale |
| `tui` | TUI 交互式 CLI |

---

## 关键数据

| 指标 | 数值 |
|------|------|
| Go 源文件数 | 1299 |
| 数据库表数 | 73 |
| HTTP 路由数 | ~230 |
| WebSocket 方法数 | ~130 |
| WebSocket 事件数 | ~65 |
| MCP 工具数 | ~50 |
| 工具注册数 | ~50 |
| LLM Provider | 20+ |
| IM 通道 | 10+ |
| 代码行数估算 | ~200K+ (Go) |
| 二进制大小 | ~25MB |
| 启动时间 | <1s |
