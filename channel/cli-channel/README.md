# GoClaw Channel Bridge

HTTP ↔ WebSocket 桥接服务，让外部程序通过 HTTP API 与 GoClaw AI Agent 通信。

## 架构

```
┌──────────────────┐     HTTP POST      ┌──────────────────┐     WebSocket     ┌──────────────────┐
│  External Client  │ ──────────────────▶│  Channel Bridge   │ ────────────────▶│  GoClaw Gateway   │
│  (curl/程序/Web)  │ ◀──────────────────│  (port 18800)     │ ◀────────────────│  (port 18790)     │
│                   │   JSON Response    │                   │   WS Frames      │  Agent Pipeline   │
└──────────────────┘                    └──────────────────┘                   └──────────────────┘
                                         SSE Stream ◀─────────────────────────── chunk events
```

## 功能

- **同步对话** (`POST /chat`) — 发送消息，等待完整响应
- **流式对话** (`POST /chat/stream`) — SSE 实时流式输出
- **会话管理** — 查看历史、列出会话、删除会话
- **技能查询** — 列出可用 Skills
- **模型查询** — 列出可用 AI Models
- **Agent 信息** — 获取 Agent 详情
- **健康检查** — 检查 Bridge 和 GoClaw 状态
- **自动重连** — 断线自动重试连接

## 快速开始

### 1. 一键启动

```powershell
# 自动编译 + 检查 goclaw + 启动
.\start.ps1

# 强制重新编译
.\start.ps1 -Build

# 自定义端口
.\start.ps1 -Port 18801
```

### 2. 手动编译运行

```bash
cd D:\code\backend\notebooklm\channel
go build -o channel-bridge.exe .
.\channel-bridge.exe -config config.json
```

### 3. 发送消息

```bash
# 同步对话
curl -X POST http://localhost:18800/chat ^
  -H "Content-Type: application/json" ^
  -d "{\"content\": \"你好，请介绍一下你自己\"}"

# 流式对话 (SSE)
curl -N -X POST http://localhost:18800/chat/stream ^
  -H "Content-Type: application/json" ^
  -d "{\"content\": \"帮我写一首诗\"}"
```

### 4. 运行测试

```powershell
.\test.ps1
```

## 配置

编辑 `config.json`:

```json
{
  "goclaw_url": "ws://localhost:18790/ws",
  "gateway_token": "your-gateway-token",
  "user_id": "channel-bridge",
  "agent_key": "agent:fox-spirit",
  "listen_addr": ":18800",
  "session_mode": "direct",
  "stream_mode": "sse"
}
```

### 环境变量覆盖

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `GOCLAW_GATEWAY_URL` | GoClaw WebSocket 地址 | `ws://localhost:18790/ws` |
| `GOCLAW_GATEWAY_TOKEN` | 认证 Token | - |
| `GOCLAW_USER_ID` | 用户 ID | `channel-bridge` |
| `GOCLAW_AGENT_KEY` | 目标 Agent | - |
| `GOCLAW_LISTEN_ADDR` | HTTP 监听地址 | `:18800` |

## HTTP API

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/chat` | 同步对话，返回完整响应 |
| `POST` | `/chat/stream` | 流式对话，SSE 输出 |
| `GET` | `/history?limit=50` | 获取聊天历史 |
| `POST` | `/abort` | 中止正在运行的请求 |
| `GET` | `/sessions` | 列出活跃会话 |
| `GET` | `/skills` | 列出可用技能 |
| `GET` | `/models` | 列出可用模型 |
| `GET` | `/agent?agent_key=xxx` | 获取 Agent 信息 |
| `GET` | `/health` | 健康检查 |
| `GET` | `/status` | Bridge 状态 |

### 请求体格式

```json
{
  "content": "消息内容（必填）",
  "agent_key": "agent:fox-spirit（可选，覆盖默认）",
  "chat_id": "自定义会话ID（可选）",
  "metadata": {"key": "value"}
}
```

### 同步响应

```json
{
  "ok": true,
  "content": "AI 回复内容",
  "elapsed": "5.234s"
}
```

### SSE 流式事件

```
event: chunk
data: {"content":"你"}

event: chunk
data: {"content":"好"}

event: done
data: {"done":true}
```

## 与 GoClaw 通信协议

Bridge 通过 GoClaw 的 WebSocket 协议 (v3) 与 Gateway 通信：

1. **connect** — 握手认证（token + user_id）
2. **chat.send** — 发送消息到 Agent
3. **chunk** — 接收流式 token
4. **run.completed** — Agent 完成处理

详细协议参见 `goclaw/websocket-protocol.md`。

## 文件结构

```
channel/
├── main.go          # 入口点，启动 HTTP 服务器
├── bridge.go        # WebSocket 桥接层（连接 GoClaw）
├── handler.go       # HTTP API 处理器
├── protocol.go      # GoClaw WebSocket 协议类型
├── config.go        # 配置加载
├── config.json      # 默认配置
├── start.ps1        # 一键启动脚本
├── test.ps1         # 测试脚本
├── go.mod           # Go 模块定义
└── README.md        # 本文档
```


干净了。总结一下新增的交互式 CLI：

新增：交互式命令行 (-i 模式)
启动方式
powershell

# 一键启动（交互式）
.\start.ps1 -Interactive

# 或直接运行
.\channel-bridge.exe -config config.json -i

# 跳过 HTTP 服务器（纯 CLI 模式）
.\channel-bridge.exe -config config.json -i -no-http
使用效果
text

╔══════════════════════════════════════════════════════════════╗
║           GoClaw Interactive CLI v1.0.0                      ║
║  Type /help for commands, /quit to exit                      ║
╚══════════════════════════════════════════════════════════════╝

  Agent:   fox-spirit
  Goclaw:  ws://localhost:18790/ws
  Session: (persistent)

You> 1+1=?
Assistant> 1+1=2
[437ms]

You> /history
── Chat History (2 messages) ──
[1] You       1+1=?
[2] Assistant 1+1=2

You> /quit
Bye!
支持的命令
命令	说明
<任意文本>	发送消息给 Agent
/help	显示帮助
/clear	清空历史，开新会话
/history	查看本地聊天记录
/history-server	查看服务端会话历史
/agent	查看/切换 Agent
/skills	列出可用技能
/status	查看连接状态
/abort	中止当前请求
/quit	退出
关键特性
持久会话 — 同一 sessionKey 贯穿整个 CLI 会话，Agent 保留上下文记忆
彩色输出 — 绿色用户、紫色助手、黄色提示、红色错误
命令补全 — 所有 / 命令即时响应
HTTP 并行 — CLI 运行时 HTTP API 仍可用（除非用 -no-http）