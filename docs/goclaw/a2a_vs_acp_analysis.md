# A2A vs ACP 协议对比分析

> GoClaw 当前使用 ACP 编排外部 Agent 子进程，使用 MCP 扩展工具能力，未实现 A2A  
> 本文档对比 A2A 与 ACP 的定位、架构、协议细节和在 GoClaw 中的作用

---

## 1. 一句话定位

| 协议 | 一句话 | 发起方 | GoClaw 是否使用 |
|------|--------|--------|-----------------|
| **A2A** (Agent-to-Agent) | **网络上**不同组织/平台的 Agent 互相发现和协作 | Google (2025.04) | ❌ 未实现 |
| **ACP** (Agent Communication Protocol) | **同一台机器**上主进程编排和控制 Agent 子进程 | BeeAI/BASF (2024) | ✅ 核心使用 |
| **MCP** (Model Context Protocol) | 给 Agent 提供**工具能力**扩展 | Anthropic (2024) | ✅ 核心使用 |

---

## 2. 三者关系图谱

```
                     ┌─────────────────────┐
                     │       MCP           │
                     │  (工具层协议)        │
                     │  Agent → 工具       │
                     └─────────┬───────────┘
                               │ Agent 通过 MCP 获得工具能力
                               ▼
                     ┌─────────────────────┐
                     │       ACP           │
                     │  (子进程编排协议)     │
                     │  主进程 → Agent子进程 │
                     └─────────┬───────────┘
                               │ 同一机器内父子进程通信
                               ▼
                     ┌─────────────────────┐
                     │       A2A           │
                     │  (跨网络协作协议)     │
                     │  Agent ↔ Agent      │
                     └─────────────────────┘
                       跨网络/跨组织对等通信
```

**层级关系**: MCP 解决"Agent 怎么用工具"，ACP 解决"怎么控制外部 Agent"，A2A 解决"不同 Agent 怎么跨网络协作"。

---

## 3. 核心拓扑区别

```
A2A 的世界 (网络级，对等):
┌──────────┐    HTTP/SSE    ┌──────────┐
│ Agent A  │ ◄────────────► │ Agent B  │     跨网络、跨组织
│ (GoClaw) │   JSON-RPC     │ (远程)   │     对等关系
└──────────┘                └──────────┘     各自独立运行

ACP 的世界 (进程级，父子):
┌──────────────────┐  stdin/stdio  ┌──────────────┐
│ GoClaw 主进程     │ ◄──────────► │ Agent 子进程   │   同一机器、父子关系
│ (父进程)          │  JSON-RPC    │ (gemini/codex) │   主进程编排子进程
│ 控制会话生命周期   │              │ 被主进程管理    │
└──────────────────┘              └──────────────┘
```

---

## 4. 七维度详细对比

| 维度 | A2A | ACP |
|------|-----|-----|
| **发起方** | Google 2025.04 开源 | BeeAI/BASF 2024 开源 |
| **传输层** | HTTP + SSE（网络） | stdin/stdout（进程管道） |
| **协议格式** | JSON-RPC 2.0 | JSON-RPC 2.0 |
| **拓扑关系** | 对等（Peer-to-Peer） | 父子（Parent-Child） |
| **发现机制** | `/.well-known/agent.json` (AgentCard) | 二进制路径 + config 配置 |
| **典型场景** | 跨公司 Agent 协作（Salesforce Agent 调用 SAP Agent） | 本地编排外部 Agent（GoClaw 调 Gemini CLI） |
| **会话模型** | Task-based（创建 Task → 轮询/订阅） | Session-based（session/new → session/prompt） |

---

## 5. A2A 协议详解

### 5.1 核心概念

```
A2A 的 5 大核心对象:

1. AgentCard           — Agent 的"名片"
   { name, description, url, capabilities, skills[] }

2. Task                — 一次协作任务
   { id, status(submitted/working/completed/failed), messages[], artifacts[] }

3. Message             — 消息（role=user/agent）
   { role, parts[{ type: text/file/data }] }

4. Artifact            — 任务产出物
   { name, parts[{ type: text/file/... }] }

5. StreamingEvent      — SSE 流式事件
   TaskStatusUpdateEvent / TaskArtifactUpdateEvent
```

### 5.2 通信流程

```
Client Agent                          Remote Agent
    │                                      │
    │── GET /.well-known/agent.json ──────►│  (1. 发现)
    │◄── AgentCard { skills, url } ────────│
    │                                      │
    │── POST /tasks/send ─────────────────►│  (2. 创建任务)
    │◄── Task { id, status: "working" } ──│
    │                                      │
    │── GET /tasks/{id} ──────────────────►│  (3. 轮询状态)
    │◄── Task { status: "completed",       │
    │          artifacts: [...] } ─────────│
    │                                      │
    │  或: GET /tasks/{id}/events (SSE) ──►│  (3'. 流式订阅)
    │◄── { type: "TaskStatusUpdate", ... }─│
    │                                      │
    │── POST /tasks/{id}/cancel ──────────►│  (4. 取消)
```

### 5.3 A2A 关键端点

| 端点 | 方法 | 用途 |
|------|------|------|
| `/.well-known/agent.json` | GET | 发现 Agent 能力（AgentCard） |
| `/tasks/send` | POST | 创建并启动任务 |
| `/tasks/{id}` | GET | 查询任务状态 |
| `/tasks/{id}/cancel` | POST | 取消任务 |
| `/tasks/{id}/events` | GET (SSE) | 流式订阅任务更新 |

### 5.4 AgentCard 结构

```json
{
  "name": "Travel Agent",
  "description": "帮助用户规划旅行",
  "url": "https://travel-agent.example.com",
  "version": "1.0.0",
  "capabilities": {
    "streaming": true,
    "pushNotifications": true,
    "stateTransitionHistory": true
  },
  "skills": [
    {
      "id": "plan-trip",
      "name": "Plan Trip",
      "description": "根据需求规划旅行方案",
      "tags": ["travel", "planning"],
      "examples": ["Plan a 3-day trip to Tokyo"]
    }
  ],
  "authentication": {
    "schemes": ["OAuth2"]
  }
}
```

### 5.5 Task 状态机

```
submitted → working → completed
                   ↘ failed
                   ↘ canceled
                   ↘ input-required  (需要更多信息)
```

### 5.6 A2A 的优势

| 优势 | 说明 |
|------|------|
| **跨组织互操作** | 任何 HTTP 客户端都能调用任何 A2A Agent |
| **自动发现** | AgentCard 机制让 Agent 自动发现彼此能力 |
| **异步任务** | Task 模型天然支持长时间运行的任务 |
| **流式响应** | SSE 支持实时进度推送 |
| **安全认证** | 内置 OAuth2/Bearer Token 认证框架 |

---

## 6. ACP 协议详解（GoClaw 实现）

### 6.1 核心概念

```
ACP 的 4 大核心对象:

1. Process             — Agent 子进程
   binary + args → exec.Command → stdin/stdout pipe

2. Session             — 会话（多路复用）
   session/new → sessionId
   session/prompt → 阻塞等待 Agent 完成

3. ContentBlock        — 内容块
   { type: "text"|"image"|"audio", text, data, mimeType }

4. ToolBridge          — 工具桥接（Agent→Client 请求）
   fs/readTextFile, fs/writeTextFile, terminal/create, permission/request
```

### 6.2 通信流程

```
GoClaw 主进程                            ACP Agent 子进程
    │                                          │
    │── exec binary ──────────────────────────►│  (1. 启动子进程)
    │                                          │
    │── {"method":"initialize", ...} ─────────►│  (2. 握手)
    │◄── {"agentInfo":{"name":"gemini"}, ...}──│
    │                                          │
    │── {"method":"session/new", ...} ────────►│  (3. 创建会话)
    │◄── {"sessionId":"abc-123"} ─────────────│
    │                                          │
    │── {"method":"session/prompt", ...} ─────►│  (4. 发送消息)
    │                                          │
    │◄── {"method":"session/update", ...} ─────│  (5. 流式通知: thinking)
    │◄── {"method":"session/update", ...} ─────│  (6. 流式通知: tool_call)
    │                                          │
    │── {"method":"fs/readTextFile", ...} ─────│  (7. Agent请求读文件)
    │── {"result":{"content":"..."}} ──────────►│  (8. 返回文件内容)
    │                                          │
    │◄── {"method":"session/update", ...} ─────│  (9. 流式通知: reply)
    │◄── {"result":{"stopReason":"end"}} ──────│  (10. prompt完成)
```

### 6.3 完整 JSON-RPC 方法表

| 方法 | 方向 | 用途 |
|------|------|------|
| `initialize` | Client→Agent | 握手，协商协议版本和能力 |
| `session/new` | Client→Agent | 创建新会话 |
| `session/load` | Client→Agent | 恢复已有会话 |
| `session/prompt` | Client→Agent | 发送用户消息（阻塞直到 Agent 完成）|
| `session/cancel` | Client→Agent (通知) | 取消当前 prompt |
| `session/update` | Agent→Client (通知) | 流式进度更新（思考/工具调用/回复）|
| `fs/readTextFile` | Agent→Client (请求) | Agent 请求读取文件 |
| `fs/writeTextFile` | Agent→Client (请求) | Agent 请求写入文件 |
| `terminal/create` | Agent→Client (请求) | Agent 请求创建终端执行命令 |
| `terminal/output` | Agent→Client (请求) | Agent 请求读取终端输出 |
| `terminal/release` | Agent→Client (请求) | Agent 请求释放终端 |
| `terminal/waitForExit` | Agent→Client (请求) | Agent 等待命令退出（10分钟超时）|
| `terminal/kill` | Agent→Client (请求) | Agent 请求终止命令 |
| `permission/request` | Agent→Client (请求) | Agent 请求操作权限 |

### 6.4 ToolBridge 安全机制

| 机制 | 说明 |
|------|------|
| **路径逃逸防护** | 所有 fs 操作检查路径是否在 workspace 内 |
| **二进制白名单** | terminal 只允许 `sh/bash/node/python/go/git/curl/...` 等安全命令 |
| **拒绝模式** | 3 种权限级别：`approve-all`（全允许）/ `approve-reads`（只读）/ `deny-all`（全拒绝）|
| **敏感环境变量过滤** | 子进程不继承 `GOCLAW_*`, `OPENAI_*`, `DATABASE_*`, `AWS_*` 等 |
| **终端输出上限** | 单个命令输出最多 10MB |
| **进程池自动回收** | 空闲超过 TTL 的子进程自动销毁 |

### 6.5 ACP Provider 在 GoClaw 中的位置

```
用户消息 → GoClaw Agent Loop → Provider.Chat()
                                    │
                          ┌──────────┼──────────┐
                          ▼          ▼          ▼
                     OpenAI API  Anthropic   ACPProvider
                                    │          │
                                    │    resolveSession()
                                    │    → session/prompt
                                    │    → session/update (流式)
                                    │    → 返回 ChatResponse
```

配置方式：
```json
{
  "providers": {
    "acp": {
      "binary": "/usr/local/bin/gemini",
      "model": "gemini-2.5-pro",
      "work_dir": "",
      "perm_mode": "approve-all"
    }
  }
}
```

---

## 7. GoClaw 中三种协议的分工

```
                    GoClaw 工具生态
                         │
           ┌─────────────┼─────────────┐
           ▼             ▼             ▼
      内建工具        MCP 工具       ACP Provider
    (read_file)    (外部服务器)    (外部 Agent)
    (exec)         (filesystem)    (gemini CLI)
    (web_search)   (database)      (codex)
    (edit)         (api)           ...
    (memory_*)     (custom)
    ...
```

| 协议 | GoClaw 中的角色 | 关系 |
|------|-----------------|------|
| **MCP** | LLM 的**工具扩展** — 给 Agent 增加能力 | Agent 调用 MCP 工具 |
| **ACP** | **外部 Agent 编排** — 把外部 Agent 当 LLM Provider | Agent Loop 调用 ACP Provider |
| **A2A** | GoClaw **未实现** | — |

### GoClaw 的 `delegate` vs A2A

GoClaw 有自己的 Agent 间委派机制（`delegate` tool + `agent_links` 表），功能上类似 A2A，但实现方式不同：

| 对比项 | A2A | GoClaw delegate |
|--------|-----|-----------------|
| 发现 | `/.well-known/agent.json` 自动发现 | `agent_links` 表手动配置 |
| 传输 | HTTP + SSE | Go 进程内直接调用 |
| 任务模型 | Task (异步, 轮询/SSE) | RunRequest (同步, 阻塞) |
| 跨组织 | ✅ 天然支持 | ❌ 单一进程内 |
| 跨语言 | ✅ 任何 HTTP 客户端 | ❌ 仅 Go 调用 |

---

## 8. 选型指南

| 场景 | 用什么 | 为什么 |
|------|--------|--------|
| 给 Agent 增加工具（文件/DB/API） | **MCP** | 标准化的工具发现和调用 |
| 在本地编排外部 Agent（Gemini/Codex） | **ACP** | 进程级控制，会话多路复用 |
| 跨公司 Agent 协作 | **A2A** | 网络级互操作，AgentCard 发现 |
| 同一系统内多 Agent 委派 | **内部机制**（如 GoClaw delegate） | 无需协议开销 |

---

## 9. 通信格式对比

### 9.1 传输层

| 协议 | 传输方式 | 编码 | 分帧 |
|------|----------|------|------|
| A2A | HTTP POST + SSE | JSON | HTTP 请求体 / SSE 事件流 |
| ACP | stdin/stdout | JSON | 换行符分隔 (`\n`) |
| MCP | HTTP + SSE / stdio | JSON-RPC 2.0 | 取决于 transport |

### 9.2 消息格式对比

**A2A — 创建任务请求:**
```json
POST /tasks/send
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tasks/send",
  "params": {
    "id": "task-001",
    "message": {
      "role": "user",
      "parts": [
        { "type": "text", "text": "帮我规划东京3日游" }
      ]
    },
    "metadata": { "priority": "high" }
  }
}
```

**ACP — 发送消息请求:**
```json
{"jsonrpc":"2.0","id":1,"method":"session/prompt","params":{"sessionId":"abc-123","prompt":[{"type":"text","text":"帮我规划东京3日游"}]}}
```

**MCP — 工具调用请求:**
```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"web_search","arguments":{"query":"东京旅游攻略"}}}
```

---

## 10. 协议演进趋势

```
2024
  ├── MCP (Anthropic) — 工具层标准，已被广泛采用
  └── ACP (BeeAI)     — 子进程编排

2025
  ├── A2A (Google)    — 跨网络 Agent 协作标准，50+ 公司支持
  └── MCP + A2A 互补  — Google 官方声明 MCP 和 A2A 不是竞争关系

2026
  ├── A2A 生态成熟   — 主流 Agent 框架开始集成
  ├── ACP 被 A2A 部分吸收 — A2A 也支持子进程模式
  └── GoClaw: MCP + ACP + 内部 delegate 三合一
```

### 关键总结

> **ACP 和 A2A 都基于 JSON-RPC 2.0，但解决完全不同的问题：**
> - **ACP** = 进程内编排（"我启动并控制你"）—— 同一机器、父子关系
> - **A2A** = 网络间协作（"我们平等合作"）—— 跨网络、对等关系
> - **MCP** = 工具扩展（"我给你能力"）—— Agent 与工具的桥梁
> - GoClaw 用 ACP 把 Gemini CLI 当本地 Provider 调用，用 MCP 扩展工具，delegate 机制实现内部 Agent 委派
> - GoClaw 目前**未实现 A2A**，但其 `delegate` + `agent_links` 在单系统内实现了类似的 Agent 间协作能力
> - Google 官方声明 **MCP 和 A2A 是互补关系**，不是竞争关系：MCP 解决 Agent↔工具，A2A 解决 Agent↔Agent
