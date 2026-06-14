# goclaw 依赖包分析

> 基于 `go.mod` 解析，结合源码实际调用，按功能域分类梳理每个依赖的用途及 goclaw 如何使用它。

---

## 目录

1. [CLI 框架与终端 UI](#1-cli-框架与终端-ui)
2. [LLM 提供商与 Token 计算](#2-llm-提供商与-token-计算)
3. [消息总线与事件系统](#3-消息总线与事件系统)
4. [数据库与存储](#4-数据库与存储)
5. [Web 框架与网络通信](#5-web-框架与网络通信)
6. [消息通道集成](#6-消息通道集成)
7. [浏览器自动化](#7-浏览器自动化)
8. [MCP 协议](#8-mcp-协议)
9. [定时任务与调度](#9-定时任务与调度)
10. [云存储 (AWS S3)](#10-云存储-aws-s3)
11. [分布式追踪 (OpenTelemetry)](#11-分布式追踪-opentelemetry)
12. [缓存 (Redis)](#12-缓存-redis)
13. [加密与安全](#13-加密与安全)
14. [知识图谱与记忆系统](#14-知识图谱与记忆系统)
15. [远程执行 (Tailscale/SSH)](#15-远程执行-tailscalessh)
16. [桌面 GUI (Wails)](#16-桌面-gui-wails)
17. [JavaScript 运行时](#17-javascript-运行时)
18. [图像处理](#18-图像处理)
19. [YAML/JSON 配置解析](#19-yamljson-配置解析)
20. [其他辅助包](#20-其他辅助包)
21. [依赖关系总览图](#21-依赖关系总览图)

---

## 1. CLI 框架与终端 UI

### github.com/spf13/cobra
- **用途**：Go 语言最流行的 CLI 框架，定义子命令、参数、帮助信息。
- **goclaw 用法**：整个 `cmd/` 包基于 cobra 构建。`main.go` 调用 `cmd.Execute()`，所有子命令（`gateway`、`agent`、`tui`、`doctor`、`upgrade`、`migrate`、`backup`、`cron`、`channels` 等）均通过 `cobra.Command` 注册。

### github.com/charmbracelet/bubbletea
- **用途**：Elm 架构的终端 UI 框架，用于构建交互式 TUI。
- **goclaw 用法**：`cmd/tui_*.go` 和 `ui/desktop/` 中的终端界面，实现 setup wizard、onboard 流程等交互式 TUI。

### github.com/charmbracelet/huh
- **用途**：基于 bubbletea 的高级表单组件（输入框、选择框、确认框）。
- **goclaw 用法**：TUI setup 流程中的交互式表单（`cmd/tui_setup.go`、`cmd/tui_onboard.go`）。

### github.com/charmbracelet/lipgloss
- **用途**：终端样式库（颜色、边框、布局）。
- **goclaw 用法**：`cmd/tui_styles.go` 中定义 TUI 全局样式。

### github.com/spf13/pflag
- **用途**：cobra 底层的命令行参数解析库。
- **goclaw 用法**：与 cobra 配合，处理 `--verbose`、`--config` 等标志。

---

## 2. LLM 提供商与 Token 计算

### github.com/pkoukk/tiktoken-go
- **用途**：OpenAI 的 tiktoken 分词器的 Go 实现，精确计算 token 数量。
- **goclaw 用法**：`internal/agent/` 中用于估算用户消息和系统提示的 token 开销，支持自适应节流（adaptive throttle）和上下文窗口管理。

### github.com/google/cel-go
- **用途**：Google Common Expression Language 的 Go 实现，用于策略表达式求值。
- **goclaw 用法**：`internal/permissions/` 和策略引擎中，用 CEL 表达式定义细粒度权限规则（如 `isOwner(ctx)`、`hasTeamRole(ctx, "admin")`），在运行时动态求值。

---

## 3. 消息总线与事件系统

goclaw 有**两套**消息/事件系统：

### 内部 MessageBus（`internal/bus/`）
- **实现**：基于 Go channel 的同步消息路由（`inbound chan InboundMessage` + `outbound chan OutboundMessage`，各 1000 缓冲）。
- **goclaw 用法**：
  - Channel（Telegram/Discord/Feishu 等）收到消息 → `PublishInbound()` → Consumer goroutine 消费 → Scheduler → Agent Loop
  - Agent 回复 → `outbound` → Channel 发送
  - WebSocket 事件广播（客户端订阅）
  - 无外部依赖，纯 Go 实现。

### DomainEventBus（`internal/eventbus/`）
- **实现**：带 worker pool 的异步事件总线，支持重试、去重、超时驱逐。
- **goclaw 用法**：
  - 驱动 V3 记忆整合流水线：episodic → semantic → KG → dreaming
  - vault enrichment worker 的事件触发
  - 配置：QueueSize=1000, WorkerCount=2, DedupTTL=5m

---

## 4. 数据库与存储

### github.com/jackc/pgx/v5
- **用途**：高性能 PostgreSQL 驱动，支持原生协议和 database/sql 接口。
- **goclaw 用法**：Standard 版本的主要数据库。`cmd/vault_graph_factory_pg.go`、所有 `*Store` 实现（sessions、agents、mcp、channels、providers、vault 等）均通过 pgx 连接 PostgreSQL。支持 pgvector 扩展用于向量相似度搜索（记忆嵌入）。

### github.com/jmoiron/sqlx
- **用途**：database/sql 的增强封装，支持结构体映射和命名参数。
- **goclaw 用法**：作为 pgx 之上的抽象层，简化 SQL 查询到 Go 结构体的映射。

### github.com/lib/pq
- **用途**：PostgreSQL 的 database/sql 驱动。
- **goclaw 用法**：部分兼容性场景使用（如某些 migration 工具依赖 pq）。

### modernc.org/sqlite
- **用途**：纯 Go 实现的 SQLite（CGO-free），可交叉编译。
- **goclaw 用法**：Lite（桌面）版本的主要数据库。`cmd/vault_graph_factory_sqlite.go` 中创建 SQLite store。无需 CGO 即可在 Windows/macOS/Linux 上运行。

### github.com/golang-migrate/migrate/v4
- **用途**：数据库 schema 迁移工具，支持版本化 migration。
- **goclaw 用法**：`goclaw migrate up/down` 命令管理 PostgreSQL schema 变更。支持 50+ 个版本化 migration 文件。

### github.com/hashicorp/golang-lru/v2
- **用途**：高性能 LRU 缓存。
- **goclaw 用法**：`internal/cache/` 中实现权限缓存（PermissionCache），缓存租户成员关系检查结果，避免每次请求都查数据库。

---

## 5. Web 框架与网络通信

### github.com/gorilla/websocket
- **用途**：Go 标准 WebSocket 实现。
- **goclaw 用法**：`internal/gateway/server.go` 中 `Server` 使用 `websocket.Upgrader` 处理 WebSocket 连接。客户端通过 WebSocket 接收实时消息、事件广播、日志流。所有 RPC 方法通过 WebSocket 双向通信。

### github.com/valyala/fasthttp
- **用途**：高性能 HTTP 客户端/服务器（比 net/http 快 10x）。
- **goclaw 用法**：`internal/webhooks/worker.go` 中回调投递使用 fasthttp 作为高性能 HTTP 客户端。

### github.com/valyala/fastjson
- **用途**：零分配 JSON 解析器。
- **goclaw 用法**：高性能 JSON 解析场景，减少 GC 压力。

### github.com/grbit/go-json
- **用途**：高性能 JSON 编解码（sonic 兼容 API）。
- **goclaw 用法**：替代 encoding/json 用于高频 JSON 操作。

### github.com/bytedance/sonic
- **用途**：字节跳动开源的高性能 JSON 库（SIMD 加速）。
- **goclaw 用法**：间接依赖，通过 go-json 或直接使用，加速 JSON 序列化/反序列化。

### golang.org/x/net
- **用途**：Go 官方扩展网络库（HTTP/2、HTML 解析等）。
- **goclaw 用法**：HTTP 客户端增强、网络工具函数。

### golang.org/x/oauth2
- **用途**：OAuth 2.0 客户端实现。
- **goclaw 用法**：`internal/http/oauth_handler.go` 处理 LLM 提供商的 OAuth 认证流程（如 GitHub OAuth、Google OAuth 等）。

---

## 6. 消息通道集成

goclaw 支持 **10+ 个消息通道**，每个通道对应一个外部 SDK：

### github.com/mymmrac/telego
- **用途**：Telegram Bot API 的 Go 实现。
- **goclaw 用法**：`internal/channels/telegram/` — 接收/发送 Telegram 消息，支持 inline keyboard、文件、语音。

### github.com/bwmarrin/discordgo
- **用途**：Discord API 的 Go 实现。
- **goclaw 用法**：`internal/channels/discord/` — Discord Bot 接入，支持 slash commands、embed、文件附件。

### github.com/slack-go/slack
- **用途**：Slack API 的 Go 实现。
- **goclaw 用法**：`internal/channels/slack/` — Slack App/Bot 接入，支持 blocks、attachments、slash commands。

### go.mau.fi/whatsmeow
- **用途**：WhatsApp Web 多设备协议的 Go 实现。
- **goclaw 用法**：`internal/channels/whatsapp/` — WhatsApp 个人号接入（基于多设备协议，非 Business API）。

### 飞书/Bitrix24/Zalo/Facebook/Pancake（内部实现）
- goclaw 对这些平台使用自研适配器（`internal/channels/feishu/`、`internal/channels/bitrix24/` 等），通过各自的 Webhook/Bot API 对接。

---

## 7. 浏览器自动化

### github.com/go-rod/rod
- **用途**：Chrome DevTools Protocol (CDP) 的 Go 封装，支持 headless Chrome 操作。
- **goclaw 用法**：
  - `pkg/browser/browser.go` — `Manager` 管理 Chrome 生命周期、页面管理、租户隔离
  - `pkg/browser/actions.go` — 页面导航、截图、点击、输入、JS 执行等操作
  - `pkg/browser/cookies.go` — Cookie 同步（浏览器 ↔ 数据库）
  - `pkg/browser/snapshot.go` — 页面 DOM 快照
  - `pkg/browser/refs.go` — 元素引用存储
  - Agent 工具：`web_fetch`、`browser_action`、`browser_snapshot` 等

---

## 8. MCP 协议

### github.com/mark3labs/mcp-go
- **用途**：Model Context Protocol (MCP) 的 Go 实现，提供 client/server SDK。
- **goclaw 用法**：
  - `internal/mcp/manager.go` — `Manager` 管理多个 MCP server 连接，自动重连、健康检查
  - `internal/mcp/pool.go` — 连接池复用
  - `internal/mcp/bridge_server.go` — 将本地工具桥接为 MCP server
  - `internal/mcp/bridge_tool.go` — 将 MCP 工具注册为本地 agent 工具
  - `internal/mcp/mcp_tool_search.go` — MCP 工具搜索（>40 个工具时切换为延迟加载模式）
  - `internal/mcp/grant_checker.go` — MCP 工具权限检查
  - `internal/mcp/validation.go` — 工具参数安全验证（防止命令注入）

---

## 9. 定时任务与调度

### github.com/adhocore/gronx
- **用途**：Cron 表达式解析和下次执行时间计算（支持标准 5/6 字段 + 扩展）。
- **goclaw 用法**：`internal/cron/service.go` — `Service` 使用 gronx 解析 cron 表达式，驱动定时任务调度（心跳、定时消息、定时 agent 触发）。

### golang.org/x/time
- **用途**：时间工具库（速率限制器 `rate.Limiter`）。
- **goclaw 用法**：`internal/gateway/ratelimit.go` — API 请求速率限制，防止 DDoS/滥用。

---

## 10. 云存储 (AWS S3)

### github.com/aws/aws-sdk-go-v2 (核心 + S3 + Config + Credentials)
- **用途**：AWS SDK v2，支持 S3、STS、SSO 等服务。
- **goclaw 用法**：
  - `cmd/gateway.go` 中的 backup API 使用 S3 存储备份文件
  - `internal/http/` 中的 `BackupS3Handler` 实现 S3 备份上传/下载
  - 支持通过环境变量或配置文件指定 S3 bucket、region、credentials

---

## 11. 分布式追踪 (OpenTelemetry)

### go.opentelemetry.io/otel (核心 + SDK + Trace)
- **用途**：OpenTelemetry Go SDK，分布式追踪和指标。
- **goclaw 用法**：Agent Loop 执行过程中创建 trace span，记录 LLM 调用、工具执行、消息路由的耗时和状态。

### go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
- **用途**：通过 gRPC 导出 trace 数据到 OTLP collector。
- **goclaw 用法**：编译时通过 `go build -tags otel` 启用，将 trace 数据发送到 Jaeger/Tempo 等后端。

### go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
- **用途**：通过 HTTP 导出 trace 数据到 OTLP collector。
- **goclaw 用法**：备选的 trace 导出方式（HTTP 替代 gRPC）。

---

## 12. 缓存 (Redis)

### github.com/redis/go-redis/v9
- **用途**：Redis 客户端，支持 Redis 7+ 特性。
- **goclaw 用法**：编译时通过 `go build -tags redis` 启用。用于：
  - 分布式锁（多实例部署时的并发控制）
  - 会话状态缓存
  - 速率限制的分布式计数器
  - MCP 工具缓存

---

## 13. 加密与安全

### github.com/zalando/go-keyring
- **用途**：跨平台系统密钥环访问（macOS Keychain、Windows Credential Manager、Linux Secret Service）。
- **goclaw 用法**：`ui/desktop/keyring.go` — 安全存储 gateway token 和 API 密钥，避免明文写入配置文件。

### github.com/skip2/go-qrcode
- **用途**：QR 码生成。
- **goclaw 用法**：`cmd/tui_onboard.go` — 生成设备配对二维码，手机扫描即可连接。

### github.com/google/uuid
- **用途**：UUID v4 生成。
- **goclaw 用法**：全局广泛使用——session ID、media ID、pairing code、webhook call ID 等。

---

## 14. 知识图谱与记忆系统

goclaw 内置了一套完整的记忆系统：

- **episodic memory**（情景记忆）：存储每次对话的摘要
- **semantic memory**（语义记忆）：长期知识存储
- **knowledge graph**（知识图谱）：实体-关系抽取和图查询
- **vault**（保险库）：文件存储 + 自动摘要 + 向量搜索

这些功能使用 **pgvector**（PostgreSQL 扩展）进行向量相似度搜索，通过 pgx 直接执行 `vector` 类型的 SQL 查询。嵌入向量通过 HTTP API 调用外部 embedding 服务生成。

---

## 15. 远程执行 (Tailscale/SSH)

### tailscale.com
- **用途**：Tailscale SDK，提供 WireGuard VPN 隧道和设备认证。
- **goclaw 用法**：
  - `internal/workstation/backends/ssh_stream.go` — 通过 Tailscale 网络 SSH 连接到远程工作站
  - `internal/workstation/backends/ssh.go` — SSH 后端实现
  - `internal/workstation/backends/ssh_pool.go` — SSH 连接池
  - Agent 通过 `workstation_exec` 工具在远程工作站执行命令

---

## 16. 桌面 GUI (Wails)

### github.com/wailsapp/wails/v2
- **用途**：Go + Web 前端的桌面应用框架（类似 Electron 但更轻量）。
- **goclaw 用法**：
  - `ui/desktop/main.go` — 桌面应用入口
  - `ui/desktop/app.go` — Go ↔ 前端桥接
  - `ui/desktop/keyring.go` — 密钥环集成
  - Lite 版本打包为桌面应用，内置 WebView2（Windows）/ WKWebView（macOS）

---

## 17. JavaScript 运行时

### github.com/dop251/goja
- **用途**：纯 Go 实现的 ECMAScript 5.1+ 运行时。
- **goclaw 用法**：
  - `internal/tools/` 中的 `exec` 工具支持执行 JavaScript 代码
  - MCP 工具的 JS 桥接层
  - 沙箱化的代码执行环境

---

## 18. 图像处理

### github.com/disintegration/imaging
- **用途**：纯 Go 图像处理库（缩放、裁剪、格式转换）。
- **goclaw 用法**：截图后处理、图像缩放、格式转换。

### golang.org/x/image
- **用途**：Go 官方扩展图像库（WebP、BMP 等格式支持）。
- **goclaw 用法**：图像格式解码支持。

---

## 19. YAML/JSON 配置解析

### gopkg.in/yaml.v3
- **用途**：YAML v3 解析和序列化。
- **goclaw 用法**：解析 SKILL.md 的 YAML frontmatter、配置文件。

### github.com/titanous/json5
- **用途**：JSON5 解析器（支持注释、尾逗号、无引号键名）。
- **goclaw 用法**：解析允许注释的配置文件。

---

## 20. 其他辅助包

### github.com/mattn/go-shellwords
- **用途**：将字符串按 shell 规则拆分为参数数组。
- **goclaw 用法**：`exec` 工具解析命令行字符串。

### github.com/mattn/go-runewidth
- **用途**：计算字符串的终端显示宽度（支持东亚字符双宽）。
- **goclaw 用法**：TUI 中正确对齐文本。

### github.com/fsnotify/fsnotify
- **用途**：跨平台文件系统事件监听。
- **goclaw 用法**：
  - Skills 目录热加载（`skills.NewWatcher`）
  - 配置文件热重载（`internal/config/hotreload.go`）

### golang.org/x/mod
- **用途**：Go 模块工具库（版本比较、模块解析）。
- **goclaw 用法**：`internal/version/semver.go` — 语义化版本比较，`upgrade` 命令检查更新。

### github.com/cespare/xxhash/v2
- **用途**：高性能非加密哈希函数。
- **goclaw 用法**：事件去重、缓存键计算。

### github.com/klauspost/compress
- **用途**：高性能压缩库（Snappy、Zstd、Gzip）。
- **goclaw 用法**：WebSocket 消息压缩、备份文件压缩。

### github.com/dustin/go-humanize
- **用途**：人类可读的数字/时间/文件大小格式化。
- **goclaw 用法**：日志输出中的文件大小、持续时间展示。

### github.com/samber/lo
- **用途**：Go 的 Lodash 工具库（泛型 map/filter/reduce）。
- **goclaw 用法**：集合操作简化代码。

---

## 21. 依赖关系总览图

```
┌─────────────────────────────────────────────────────────────────────┐
│                          goclaw 架构                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │ Telegram  │  │ Discord  │  │  Slack   │  │ WhatsApp │  ... ×10 │
│  │ (telego)  │  │(disc-go) │  │(slk-go)  │  │(whatsmeo)│          │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘          │
│       └──────────────┴──────────────┴──────────────┘               │
│                          │ channel messages                         │
│                    ┌─────▼─────┐                                    │
│                    │ MessageBus│ ← gorilla/websocket (客户端 WS)    │
│                    │  (channel)│                                    │
│                    └─────┬─────┘                                    │
│                          │                                          │
│              ┌───────────▼───────────┐                              │
│              │     Consumer          │                              │
│              └───────────┬───────────┘                              │
│                          │                                          │
│              ┌───────────▼───────────┐                              │
│              │     Scheduler         │                              │
│              │  (Lane + Queue)       │                              │
│              └───────────┬───────────┘                              │
│                          │                                          │
│              ┌───────────▼───────────┐                              │
│              │     Agent Loop        │                              │
│              │  ┌─────────────────┐  │                              │
│              │  │ Provider Registry│  │ ← tiktoken-go (token 计算) │
│              │  │ (Anthropic/OpenAI│  │ ← cel-go (权限策略)        │
│              │  │  /Custom)        │  │                              │
│              │  └────────┬────────┘  │                              │
│              │           │           │                              │
│              │  ┌────────▼────────┐  │                              │
│              │  │  Tool Registry  │  │ ← goja (JS 执行)            │
│              │  │  ┌────────────┐ │  │ ← go-rod (浏览器)          │
│              │  │  │ MCP Bridge │ │  │ ← mcp-go (MCP 协议)        │
│              │  │  └────────────┘ │  │                              │
│              │  └─────────────────┘  │                              │
│              └───────────┬───────────┘                              │
│                          │                                          │
│          ┌───────────────┼───────────────┐                          │
│          ▼               ▼               ▼                          │
│   ┌──────────┐   ┌──────────┐   ┌──────────┐                      │
│   │ PostgreSQL│   │  SQLite  │   │  Redis   │                      │
│   │  (pgx)   │   │(modernc) │   │(go-redis)│                      │
│   │ +pgvector │   │  (Lite)  │   │  (tag)   │                      │
│   └──────────┘   └──────────┘   └──────────┘                      │
│          │                                                       │
│   ┌──────▼──────────────────────────────────────┐                 │
│   │           子系统                              │                 │
│   │  ┌──────────┐ ┌──────────┐ ┌─────────────┐ │                 │
│   │  │Consolidat│ │  Vault   │ │  Knowledge  │ │                 │
│   │  │ion Pipeline│ │(文件搜索)│ │   Graph     │ │                 │
│   │  │(记忆整合) │ │          │ │ (实体抽取)  │ │                 │
│   │  └──────────┘ └──────────┘ └─────────────┘ │                 │
│   └─────────────────────────────────────────────┘                 │
│                                                                     │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│   │OpenTelemetry│  │   AWS S3    │  │  Tailscale  │              │
│   │  (追踪)     │  │  (备份)     │  │  (远程SSH)  │              │
│   └─────────────┘  └─────────────┘  └─────────────┘              │
│                                                                     │
│   ┌─────────────────────────────────────────────┐                  │
│   │  Desktop (Wails v2)  │  CLI (Cobra + Bubbletea)              │
│   │  Lite 版本桌面 GUI    │  标准版命令行交互                      │
│   └─────────────────────────────────────────────┘                  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 关键架构洞察

### 1. 双数据库策略
- **Standard 版本**：PostgreSQL（pgx）+ pgvector → 支持多租户、向量搜索、完整记忆系统
- **Lite 版本**：纯 Go SQLite（modernc）→ 零依赖桌面部署，单用户

### 2. 编译时特性开关
```
go build -tags otel    → 启用 OpenTelemetry 追踪导出
go build -tags redis   → 启用 Redis 缓存层
```

### 3. 10+ 消息通道的统一抽象
所有通道通过 `channels.Manager` + `InstanceLoader` + Factory 模式统一管理，消息进入 `MessageBus` 后处理逻辑完全一致。

### 4. MCP 协议桥接
goclaw 同时是 MCP client（连接外部 MCP server）和 MCP server（将本地工具暴露给外部），通过 `mcp-go` SDK 实现双向桥接。

### 5. 记忆系统的完整链路
```
用户消息 → episodic store → consolidation pipeline
  → semantic store → knowledge graph extraction → dreaming worker
  → 向量搜索 (pgvector) → 上下文注入 → Agent Loop
```
