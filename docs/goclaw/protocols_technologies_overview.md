# GoClaw 全协议与技术栈总览

> 基于 go.mod 依赖分析 + 源码 grep 扫描  
> 覆盖所有通信协议、存储技术、安全机制、前端框架、LLM 适配、IM 通道、外部集成

---

## 1. 通信协议全景

```
┌─────────────────────────────────────────────────────────────────┐
│                     GoClaw 协议栈                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ Agent 间协议层                                            │  │
│  │  MCP (Model Context Protocol) — 工具扩展                  │  │
│  │  ACP (Agent Communication Protocol) — 子进程编排          │  │
│  │  内部 delegate — Agent 委派                               │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ 实时通信层                                                │  │
│  │  WebSocket (RFC 6455) — 客户端 ↔ 网关双向通信            │  │
│  │  JSON-RPC 2.0 — WebSocket 帧协议 v3                      │  │
│  │  SSE (Server-Sent Events) — 流式进度推送                  │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ LLM API 层                                               │  │
│  │  OpenAI Chat Completions API — 流式/非流式                │  │
│  │  Anthropic Messages API — 流式/非流式                     │  │
│  │  OpenAI Responses API — 结构化响应                        │  │
│  │  Google Vertex AI — OpenAI 兼容模式                       │  │
│  │  DashScope (阿里百炼) — OpenAI 兼容模式                   │  │
│  │  OpenAI Embeddings API — 向量嵌入                         │  │
│  │  OpenAI TTS API — 文字转语音                              │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ IM 通道协议层                                             │  │
│  │  Telegram Bot API — 长轮询                               │  │
│  │  Discord Gateway (WebSocket + 语音)                       │  │
│  │  Slack Events API + Socket Mode                           │  │
│  │  WhatsApp Business API (whatsmeow)                        │  │
│  │  飞书 Open API + WebSocket                               │  │
│  │  Bitrix24 REST API + OAuth                               │  │
│  │  Facebook Graph API                                      │  │
│  │  Zalo Personal (WebSocket)                               │  │
│  │  Pancake API                                             │  │
│  │  Kafka (Consumer/Producer)                               │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ 网络与安全层                                              │  │
│  │  HTTP/1.1 + HTTP/2 — REST API                            │  │
│  │  TLS — HTTPS 加密                                         │  │
│  │  PROXY Protocol v1/v2 — 反向代理透传                      │  │
│  │  Tailscale tsnet — 内网穿透                              │  │
│  │  OAuth 2.0 — 第三方登录 (OpenAI/ChatGPT)                  │  │
│  │  HMAC-SHA256 — Webhook 签名验证                           │  │
│  │  AES-256-GCM — 敏感数据加密                               │  │
│  │  SHA-256 — 内容哈希                                       │  │
│  │  Bearer Token — API 认证                                  │  │
│  │  SSRF 防护 — URL 安全检查                                 │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ 追踪与观测层                                              │  │
│  │  OpenTelemetry (OTLP) — 分布式追踪                       │  │
│  │  OTLP/gRPC — 追踪数据导出 (Collector)                    │  │
│  │  OTLP/HTTP — 追踪数据导出 (备用)                         │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ 浏览器自动化层                                            │  │
│  │  Chrome DevTools Protocol (CDP) — 浏览器控制              │  │
│  │  go-rod — CDP Go 客户端                                  │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. 协议详解

### 2.1 WebSocket + JSON-RPC 2.0（帧协议 v3）

**用途**: 客户端（Web UI / CLI / Desktop）与 Gateway 网关的实时双向通信

**传输**: WebSocket 连接 (`/ws`)

**帧类型**:

| 帧类型 | 方向 | 结构 |
|--------|------|------|
| `req` | Client→Server | `{ type:"req", id, method, params }` |
| `res` | Server→Client | `{ type:"res", id, ok, payload, error }` |
| `event` | Server→Client | `{ type:"event", event, payload, seq, stateVersion }` |

**错误码枚举**:
```
UNAVAILABLE / NOT_LINKED / NOT_PAIRED / AGENT_TIMEOUT /
UNAUTHORIZED / NOT_FOUND / ALREADY_EXISTS / RESOURCE_EXHAUSTED /
FAILED_PRECONDITION / INTERNAL / TENANT_ACCESS_REVOKED / NOT_IMPLEMENTED
```

**角色权限**:
```
owner > admin > operator > viewer
```

**方法总数**: ~130 个（详见 api_interfaces_analysis.md）

---

### 2.2 SSE (Server-Sent Events)

**用途**: HTTP 长连接流式进度推送

**实现**: 标准 `text/event-stream` 响应 + `http.Flusher`

**使用场景**:
- Agent 导出/导入进度
- 备份进度
- MCP 导出/导入
- S3 备份上传
- 系统升级

**事件格式**:
```
event: progress
data: {"phase":"init","status":"ok","detail":"starting"}

event: complete
data: {"token":"download-xxx"}

event: error
data: {"phase":"import","status":"error","detail":"invalid format"}
```

---

### 2.3 MCP (Model Context Protocol)

**用途**: 给 Agent 增加外部工具能力

**传输方式**: Streamable HTTP (`/mcp/bridge`) 或 stdio

**协议格式**: JSON-RPC 2.0

**工具发现**: `tools/list` → 返回工具名、描述、参数 Schema

**工具调用**: `tools/call` → 返回执行结果

**在 GoClaw 中**: `internal/mcp/` 包实现完整 MCP Client + Bridge Server

---

### 2.4 ACP (Agent Communication Protocol)

**用途**: 主进程编排外部 Agent 子进程（如 Gemini CLI、Codex）

**传输**: stdin/stdout（换行分隔 JSON-RPC 2.0）

**协议版本**: v1

**核心方法**: `initialize` → `session/new` → `session/prompt` → `session/cancel`

**工具桥接**: Agent 通过 `fs/readTextFile`、`terminal/create` 等请求调用 GoClaw 执行操作

**详见**: `acp_protocol_analysis.md`

---

### 2.5 OpenAI Chat Completions API

**用途**: 兼容 OpenAI 的 LLM 调用（也用于 Anthropic/Vertex/DashScope 适配）

**端点**: `POST /v1/chat/completions`

**流式**: `stream: true` → SSE 格式 `data: {"choices":[{"delta":{"content":"..."}}]}`

**核心字段**:
```json
{
  "model": "gpt-4o",
  "messages": [{"role":"system","content":"..."},{"role":"user","content":"..."}],
  "tools": [{"type":"function","function":{"name":"...","parameters":{...}}}],
  "stream": true,
  "temperature": 0.7,
  "max_tokens": 4096,
  "tool_choice": "auto"
}
```

**角色枚举**: `system` / `user` / `assistant` / `tool`

**停止原因**: `stop` / `length` / `tool_calls` / `content_filter`

---

### 2.6 Anthropic Messages API

**用途**: Claude 系列模型调用

**端点**: `POST /v1/messages`

**核心差异**:
- 系统提示词通过 `system` 字段传递（非消息数组）
- 工具结果通过 `tool_result` content block 传递
- 思考过程通过 `thinking` content block 返回

**核心字段**:
```json
{
  "model": "claude-sonnet-4-20250514",
  "max_tokens": 8192,
  "system": "You are a helpful assistant...",
  "messages": [{"role":"user","content":"..."}],
  "tools": [{"name":"...","description":"...","input_schema":{...}}]
}
```

---

### 2.7 OpenAI Responses API

**用途**: 新一代结构化响应 API

**端点**: `POST /v1/responses`

**核心差异**: 响应结构化为 `output` 数组（含 `message`、`function_call`、`reasoning` 等项）

---

### 2.8 OpenAI Embeddings API

**用途**: 文本向量化（用于 RAG 语义搜索）

**端点**: `POST /v1/embeddings`

**模型**: `text-embedding-3-small` (1536 维)、`text-embedding-3-large` (3072 维)

**核心字段**:
```json
{
  "model": "text-embedding-3-small",
  "input": ["text to embed"],
  "dimensions": 1536
}
```

---

### 2.9 OpenTelemetry (OTLP)

**用途**: 分布式追踪数据导出

**导出方式**:
- **OTLP/gRPC** — 默认（`otlptracegrpc`）
- **OTLP/HTTP** — 备用（`otlptracehttp`）

**数据模型**: Traces → Spans（父子层级）

**GoClaw 实现**: `internal/tracing/` 包，追踪覆盖 LLM 调用、工具执行、Hook 执行

---

### 2.10 Chrome DevTools Protocol (CDP)

**用途**: 浏览器自动化（无头 Chrome 控制）

**Go 客户端**: `go-rod` 库

**使用场景**:
- 浏览器 Cookie 同步（`browser_cookie_store`）
- 远程浏览器执行（`workstation_exec`）
- 网页内容抓取辅助

---

### 2.11 PROXY Protocol (v1/v2)

**用途**: 反向代理后透传真实客户端 IP

**库**: `pires/go-proxyproto`

**场景**: GoClaw 部署在 Nginx/HAProxy 后面时，保留原始连接信息

---

### 2.12 Tailscale tsnet

**用途**: 内网穿透（WireGuard VPN 隧道）

**库**: `tailscale.com`

**场景**: 无需公网 IP 即可远程访问 GoClaw Gateway

---

### 2.13 Kafka (扩展)

**用途**: 消息入站/出站通道（可选扩展）

**路径**: `extensions/kafka-channel/`

**角色**: Consumer 读取用户消息 → GoClaw 处理 → Producer 发送回复

---

## 3. 安全机制全景

### 3.1 加密

| 机制 | 算法 | 用途 | 实现 |
|------|------|------|------|
| **对称加密** | AES-256-GCM | 敏感字段加密存储 | `internal/crypto/aes.go` |
| **内容哈希** | SHA-256 | 文件去重、完整性校验 | 标准库 `crypto/sha256` |
| **密钥派生** | — | 从主密钥派生子密钥 | `GOCLAW_ENCRYPTION_KEY` 环境变量 |

**加密字段**（AES-256-GCM）:
- `llm_providers.api_key`
- `channel_instances.credentials`
- `config_secrets.value`
- `mcp_user_credentials.headers/env`
- `secure_cli_binaries.encrypted_env`
- `bitrix_portals.credentials/state`
- `workstations.metadata/default_env`
- `browser_cookies.encrypted_value`

### 3.2 认证

| 机制 | 用途 | 实现 |
|------|------|------|
| **Bearer Token** | Gateway API 认证 | `Authorization: Bearer <token>` |
| **API Key** | 第三方 API 调用 | SHA-256 哈希存储 |
| **OAuth 2.0** | OpenAI/ChatGPT 登录 | `golang.org/x/oauth2` |
| **HMAC-SHA256** | Webhook 签名验证 | 自实现 |
| **Pairing Code** | 设备配对（8 位码） | 随机生成 + 过期时间 |

### 3.3 授权

| 机制 | 用途 |
|------|------|
| **RBAC** | 4 级角色: owner > admin > operator > viewer |
| **Agent Shares** | Agent 级用户授权 |
| **Skill Grants** | 技能→Agent/用户授权 |
| **MCP Grants** | MCP 服务器→Agent/用户授权 |
| **Config Permissions** | 配置变更权限控制 |
| **Workstation Permissions** | 工作站二进制白名单 |

### 3.4 安全防护

| 防护 | 说明 | 实现 |
|------|------|------|
| **Shell 拒绝模式** | 13 类危险命令模式 | `tools/shell_deny_groups.go` |
| **SSRF 防护** | URL 安全检查 | `tools/web_shared.go` |
| **路径逃逸防护** | 文件操作限制在 workspace 内 | `sandbox/fsbridge.go` |
| **环境变量过滤** | 子进程不继承敏感变量 | `acp/helpers.go` |
| **凭证脱敏** | 日志/API 响应中自动隐藏 | `tools/scrub.go` |
| **终端输出上限** | 命令输出最多 10MB | `acp/terminal.go` |

---

## 4. 存储技术

### 4.1 数据库

| 技术 | 角色 | 用途 |
|------|------|------|
| **PostgreSQL** | 主存储 | 全部 73 张表 + pgvector 向量搜索 + FTS 全文搜索 |
| **SQLite** | 嵌入式存储 | 开发/单机环境，73 张表镜像实现 |
| **Redis** | 可选缓存 | `agent_ctx_cache` / `user_ctx_cache` |

### 4.2 PostgreSQL 扩展

| 扩展 | 用途 |
|------|------|
| **pgvector** | `vector(1536)` 余弦相似度搜索 |
| **tsvector/tsquery** | 全文搜索 + GIN 索引 |
| **pg_trgm** | 模糊匹配（隐含） |

### 4.3 文件存储

| 技术 | 路径 | 用途 |
|------|------|------|
| **本地文件系统** | `.media/{sessionHash}/` | 媒体文件（图片/音频/视频/文档） |
| **本地文件系统** | `skills-store/{slug}/` | 技能文件（SKILL.md + 资源） |
| **Docker 卷** | `.sandboxes/` | 沙箱 workspace |
| **S3 兼容存储** | `{prefix}/goclaw-backups/` | 备份上传（可选） |

### 4.4 外部服务

| 技术 | 角色 | 用途 |
|------|------|------|
| **Docker** | 容器运行时 | 沙箱执行环境 |
| **Kafka** (扩展) | 消息队列 | 消息通道 |

---

## 5. LLM 适配层

### 5.1 支持的 LLM Provider

| Provider | 适配方式 | 模型 |
|----------|----------|------|
| **OpenAI** | 原生 API | GPT-4o, GPT-4.1, o3, o4-mini |
| **Anthropic** | 原生 API | Claude Sonnet 4, Claude Opus 4 |
| **Google Vertex AI** | OpenAI 兼容 | Gemini 2.5 Pro/Flash |
| **Google Gemini** | OpenAI 兼容 | Gemini 系列 |
| **DashScope** (阿里百炼) | OpenAI 兼容 | Qwen 系列 |
| **OpenRouter** | OpenAI 兼容 | 多模型聚合 |
| **Codex** (OpenAI) | 原生 API | Codex 系列 |
| **Claude CLI** | 子进程 (ACP) | Claude Code |
| **ACP** (通用) | 子进程 (JSON-RPC) | 任意 ACP 兼容 Agent |

### 5.2 Provider 能力

| 能力 | 说明 |
|------|------|
| **Chat Completions** | 对话（流式/非流式）|
| **Embeddings** | 文本向量化 |
| **TTS** | 文字转语音 |
| **Image Generation** | 图片生成 |
| **Video Generation** | 视频生成 |
| **Audio Generation** | 音频生成 |
| **Reasoning/Thinking** | 深度推理（extended thinking）|

### 5.3 高级特性

| 特性 | 说明 |
|------|------|
| **Failover** | 多 Provider 故障转移 |
| **Model Fallback** | 主模型失败自动切换备用模型 |
| **Cooldown** | Provider 限流冷却 |
| **Retry** | 指数退避重试 |
| **Cache Middleware** | Prompt 缓存控制 |
| **Service Tier** | 服务层级选择 |
| **Schema Normalizer** | 工具 Schema 自动适配各 Provider |

---

## 6. 前端技术栈

### 6.1 Web UI

| 技术 | 用途 |
|------|------|
| **React** | UI 框架 |
| **TypeScript** | 类型安全 |
| **Vite** | 构建工具 |
| **React Router** | 路由 |

### 6.2 Desktop App

| 技术 | 用途 |
|------|------|
| **Wails v2** | Go + Web 桌面应用框架 |
| **WebView** | 原生 WebView 渲染 |

### 6.3 CLI/TUI

| 技术 | 用途 |
|------|------|
| **Cobra** | CLI 命令框架 |
| **bubbletea** | TUI 交互式界面 |
| **huh** | 表单输入 |
| **lipgloss** | TUI 样式 |

---

## 7. IM 通道技术

| 通道 | SDK/协议 | 连接方式 |
|------|----------|----------|
| **Telegram** | `telego` | Bot API 长轮询 |
| **Discord** | `discordgo` | WebSocket Gateway |
| **Slack** | `slack-go` | Events API + Socket Mode |
| **WhatsApp** | `whatsmeow` | WebSocket (E2EE) |
| **飞书** | 自实现 | Open API + WebSocket |
| **Bitrix24** | 自实现 | REST API + OAuth |
| **Facebook** | 自实现 | Graph API Webhook |
| **Zalo** | 自实现 | WebSocket (Personal) |
| **Pancake** | 自实现 | HTTP Webhook |
| **Kafka** (扩展) | Sarama | Consumer/Producer |

---

## 8. 开发工具与库

### 8.1 核心依赖

| 库 | 用途 |
|------|------|
| `pgx/v5` | PostgreSQL 驱动 |
| `sqlx` | SQL 扩展 |
| `modernc.org/sqlite` | 纯 Go SQLite |
| `go-redis/v9` | Redis 客户端 |
| `gorilla/websocket` | WebSocket |
| `golang-migrate/migrate` | 数据库迁移 |
| `goja` | JavaScript 运行时（Hook 脚本）|
| `cel-go` | CEL 表达式引擎（Hook 条件）|
| `tiktoken-go` | Token 计数 |
| `go-keyring` | 系统密钥链 |
| `fsnotify` | 文件监控（配置热重载）|
| `uuid` | UUID 生成（v7 时间有序）|
| `yaml.v3` | YAML 解析 |
| `json5` | JSON5 容错解析 |
| `imaging` | 图片处理 |
| `qrcode` | QR 码生成 |

### 8.2 运行时检测

| 运行时 | 用途 |
|------|------|
| **Node.js/npm/pnpm** | 技能依赖安装 |
| **Python/pip/pipx** | 技能依赖安装 |
| **Go** | 技能依赖安装 |
| **apk** (Alpine) | 包管理 |

---

## 9. 规则引擎

| 技术 | 用途 | 实现 |
|------|------|------|
| **CEL** (Common Expression Language) | Hook 条件表达式 | `google/cel-go` |
| **正则表达式** | Shell 拒绝模式、工具名匹配 | Go `regexp` |
| **JSON Schema** | 工具参数验证 | 各 Provider Schema 规范 |

---

## 10. 协议在 GoClaw 中的位置关系

```
                    ┌──────────────────────────┐
                    │        用户               │
                    │  (Web UI / CLI / Desktop) │
                    └───────────┬──────────────┘
                                │
              WebSocket + JSON-RPC 2.0 (帧协议 v3)
              HTTP REST API (管理面)
                                │
                    ┌───────────▼──────────────┐
                    │     GoClaw Gateway        │
                    │   (HTTP Server + WS)      │
                    └───────────┬──────────────┘
                                │
                    ┌───────────▼──────────────┐
                    │     Agent Router          │
                    │   (按 agent_key 分发)     │
                    └───────────┬──────────────┘
                                │
              ┌─────────────────┼─────────────────┐
              ▼                 ▼                 ▼
    ┌─────────────┐   ┌─────────────┐   ┌─────────────┐
    │ Agent Loop   │   │ Agent Loop   │   │ Agent Loop   │
    │ (Pipeline)   │   │ (Pipeline)   │   │ (Pipeline)   │
    └──────┬──────┘   └──────┬──────┘   └──────┬──────┘
           │                  │                  │
    ┌──────▼──────┐   ┌──────▼──────┐   ┌──────▼──────┐
    │ LLM Provider│   │ ACP Provider│   │ MCP Manager  │
    │ (OpenAI等)   │   │ (子进程)     │   │ (工具扩展)   │
    └─────────────┘   └─────────────┘   └─────────────┘
           │                  │                  │
    OpenAI/Anthropic    JSON-RPC 2.0       JSON-RPC 2.0
    Chat Completions    (stdio)            (HTTP/stdio)
    SSE 流式                                     │
                                         ┌───────▼───────┐
                                         │ 外部 MCP 服务器│
                                         │ (filesystem等) │
                                         └───────────────┘
```

---

## 11. go.mod 关键依赖分类

### 通信
| 依赖 | 版本 | 用途 |
|------|------|------|
| `gorilla/websocket` | v1.5.4 | WebSocket |
| `coder/websocket` | v1.8.14 | WebSocket (indirect) |
| `pires/go-proxyproto` | v0.8.1 | PROXY Protocol |
| `tailscale.com` | v1.94.2 | Tailscale tsnet |

### 存储
| 依赖 | 版本 | 用途 |
|------|------|------|
| `jackc/pgx/v5` | v5.6.0 | PostgreSQL |
| `jmoiron/sqlx` | v1.4.0 | SQL 扩展 |
| `modernc.org/sqlite` | v1.47.0 | SQLite (纯 Go) |
| `redis/go-redis/v9` | v9.18.0 | Redis |
| `golang-migrate/migrate` | v4.19.1 | 数据库迁移 |

### IM 通道
| 依赖 | 版本 | 用途 |
|------|------|------|
| `mymmrac/telego` | v1.6.0 | Telegram |
| `bwmarrin/discordgo` | v0.29.0 | Discord |
| `slack-go/slack` | v0.19.0 | Slack |
| `go.mau.fi/whatsmeow` | v0.0.0 | WhatsApp |
| `aws/aws-sdk-go-v2` | v1.41.5 | S3 备份 |

### LLM & AI
| 依赖 | 版本 | 用途 |
|------|------|------|
| `pkoukk/tiktoken-go` | v0.1.8 | Token 计数 |
| `google/cel-go` | v0.28.0 | CEL 表达式引擎 |
| `dop251/goja` | v0.0.0 | JavaScript 运行时 |

### 追踪
| 依赖 | 版本 | 用途 |
|------|------|------|
| `go.opentelemetry.io/otel` | v1.40.0 | OpenTelemetry |
| `otel/exporters/otlp/otlptracegrpc` | v1.40.0 | OTLP/gRPC 导出 |
| `otel/exporters/otlp/otlptracehttp` | v1.40.0 | OTLP/HTTP 导出 |
| `google.golang.org/grpc` | v1.78.0 | gRPC (OTLP 依赖) |

### 浏览器
| 依赖 | 版本 | 用途 |
|------|------|------|
| `go-rod/rod` | v0.116.2 | Chrome DevTools Protocol |

### TUI / CLI
| 依赖 | 版本 | 用途 |
|------|------|------|
| `spf13/cobra` | v1.10.2 | CLI 框架 |
| `charmbracelet/bubbletea` | v1.3.6 | TUI 框架 |
| `charmbracelet/huh` | v0.8.0 | 表单输入 |
| `charmbracelet/lipgloss` | v1.1.0 | TUI 样式 |

### Desktop
| 依赖 | 版本 | 用途 |
|------|------|------|
| `wailsapp/wails/v2` | v2.12.0 | 桌面应用框架 |

### 安全
| 依赖 | 版本 | 用途 |
|------|------|------|
| `golang.org/x/oauth2` | v0.34.0 | OAuth 2.0 |
| `zalando/go-keyring` | v0.2.8 | 系统密钥链 |
| `golang.org/x/image` | v0.27.0 | 图片处理 |

### 工具
| 依赖 | 版本 | 用途 |
|------|------|------|
| `fsnotify/fsnotify` | v1.9.0 | 文件监控 |
| `hashicorp/golang-lru/v2` | v2.0.7 | LRU 缓存 |
| `google/uuid` | v1.6.0 | UUID 生成 |
| `titanous/json5` | v1.0.0 | JSON5 解析 |
| `disintegration/imaging` | v1.6.2 | 图片处理 |
| `skip2/go-qrcode` | v0.0.0 | QR 码生成 |
| `golang.org/x/mod` | v0.35.0 | Go 模块工具 |
| `golang.org/x/time` | v0.14.0 | 限流 |

---

## 12. 协议选型总结

| 需求 | 选什么协议/技术 | GoClaw 中的实现 |
|------|-----------------|-----------------|
| 客户端实时交互 | WebSocket + JSON-RPC 2.0 | `internal/gateway/` |
| 管理面 CRUD | HTTP REST API | `internal/http/` |
| Agent 调用 LLM | OpenAI/Anthropic API | `internal/providers/` |
| Agent 使用工具 | MCP (JSON-RPC 2.0) | `internal/mcp/` |
| 编排外部 Agent | ACP (JSON-RPC 2.0 stdio) | `internal/providers/acp/` |
| Agent 间委派 | 内部 delegate (Go 函数调用) | `internal/tools/delegate.go` |
| 文本向量化 | OpenAI Embeddings API | `internal/memory/embeddings.go` |
| 语义搜索 | pgvector cosine similarity | `internal/store/pg/memory_search.go` |
| 全文搜索 | PostgreSQL FTS / SQLite FTS5 | `internal/store/pg/*.go` |
| 流式进度 | SSE (Server-Sent Events) | `internal/http/sse.go` |
| 分布式追踪 | OpenTelemetry (OTLP) | `internal/tracing/` |
| 浏览器自动化 | Chrome DevTools Protocol | `pkg/browser/` |
| 消息通道 | IM SDK (Telegram/Discord/Slack/...) | `internal/channels/` |
| 备份上传 | S3 API (AWS SDK v2) | `internal/backup/s3_client.go` |
| 缓存 | Redis / Go 内存 LRU | `internal/cache/` |
| 加密存储 | AES-256-GCM | `internal/crypto/` |
| Webhook 验证 | HMAC-SHA256 | `internal/http/webhooks_auth.go` |
| 规则引擎 | CEL + 正则 | `internal/hooks/matcher.go` |
| Token 计数 | tiktoken (BPE) | `internal/tokencount/` |
| 配置热重载 | fsnotify (inotify) | `internal/config/hotreload.go` |
| 内网穿透 | Tailscale tsnet (WireGuard) | `cmd/gateway_tsnet.go` |
| 反向代理 | PROXY Protocol v1/v2 | `pires/go-proxyproto` |
| 桌面应用 | Wails v2 (Go + WebView) | `ui/desktop/` |
| CLI | Cobra + bubbletea | `cmd/` |
