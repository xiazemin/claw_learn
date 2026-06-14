# GoClaw 桌面客户端编译指南（Windows 11）

> 从零开始在 Win11 上编译 GoClaw Lite 桌面客户端的完整指南

---

## 1. 什么是 GoClaw Lite（桌面版）

GoClaw Lite 是 GoClaw 的**单机桌面版**，不需要 Docker、不需要 PostgreSQL、不需要任何外部服务。

```
┌──────────────────────────────────────────────┐
│           GoClaw Lite (Windows .exe)         │
│                                              │
│  ┌────────────────┐  ┌────────────────────┐  │
│  │  Go Gateway     │  │  WebView2 (Chromium)│  │
│  │  (嵌入式)       │◄►│  React 前端 UI     │  │
│  │  localhost:18790│  │  WebSocket 实时通信 │  │
│  │  SQLite 存储    │  │                    │  │
│  └────────────────┘  └────────────────────┘  │
│           Wails v2 框架桥接 Go ↔ JS           │
└──────────────────────────────────────────────┘
```

**核心特点**：
- 单个 `.exe` 文件（~30MB），双击即用
- SQLite 数据库（`~/.goclaw/data/goclaw.db`），零配置
- 加密密钥存 Windows 凭据管理器（Credential Manager）
- 自动更新（从 GitHub Releases 下载）
- 最多 5 个 Agent、1 个团队（5 成员）

### Lite vs Standard

| 特性 | Lite (桌面) | Standard (服务器) |
|------|------------|-------------------|
| Agent | 最多 5 个 | 无限制 |
| 团队 | 最多 1 个 | 无限制 |
| 数据库 | SQLite (本地) | PostgreSQL |
| 记忆搜索 | FTS5 文本 | pgvector 语义 |
| 通道 | 无 IM 通道 | Telegram/Discord/Slack/... |
| 知识图谱 | 无 | 完整 |
| 多租户 | 无 | 完整 |

---

## 2. 技术原理

### 2.1 Wails v2 架构

GoClaw Lite 使用 [Wails v2](https://wails.io) 框架，将 Go 后端和 React 前端打包为一个原生桌面应用。

**Wails 的工作方式**：

```
1. Go 后端启动
   ├── 初始化 Wails Runtime（创建 WebView2 窗口）
   ├── 加载嵌入的 React 前端（dist/ 目录打包进二进制）
   ├── 注入 Go 方法到 JS 全局对象 (window.go)
   ├── 启动内嵌 Gateway（端口 18790）
   └── 前端通过 WebSocket 连接 Gateway

2. JS 前端运行
   ├── React 渲染 UI
   ├── 通过 window.go.GetGatewayURL() 获取后端地址
   ├── 通过 WebSocket 实现实时通信
   └── 通过 window.go.OpenFile() 等调用系统功能
```

**关键技术**：

| 组件 | 技术 | 说明 |
|------|------|------|
| 窗口 | WebView2 (Chromium) | Windows 10/11 内置 |
| 桥接 | Wails Runtime | Go ↔ JS 双向调用 |
| 前端 | React + TypeScript + Vite | 嵌入到 Go 二进制 |
| 后端 | Go Gateway | 嵌入式运行（端口 18790） |
| 存储 | SQLite (modernc.org/sqlite) | 纯 Go 实现，无需 CGO |
| 密钥 | Windows Credential Manager | 通过 `go-keyring` 访问 |
| 更新 | GitHub Releases | 启动后 + 每 6 小时检查 |

### 2.2 内嵌 Gateway 生命周期

```
App.startup()
  ├── EnsureSecrets()        ← 从 Credential Manager 读取/生成密钥
  ├── 设置环境变量:
  │     GOCLAW_ENCRYPTION_KEY = encKey
  │     GOCLAW_GATEWAY_TOKEN  = gwToken
  │     GOCLAW_STORAGE_BACKEND = sqlite
  │     GOCLAW_HOST = 127.0.0.1（仅本地）
  │     GOCLAW_DESKTOP = 1
  ├── startGateway()         ← goroutine 启动 Gateway
  ├── waitForGateway()       ← 轮询 /health 直到就绪
  └── updateLoop()           ← 启动自动更新检查

App.shutdown()
  └── cancelGw()             ← 优雅关闭 Gateway
```

### 2.3 密钥管理

```
首次启动:
  Credential Manager 查找 → 未找到
  ↓
  文件 ~/.goclaw/secrets/ 查找 → 未找到
  ↓
  生成随机 encKey + gwToken（各 32 字节 hex）
  ↓
  写入 Credential Manager → 成功
  ↓
  （如 Keychain 不可用）写入文件 ~/.goclaw/secrets/

后续启动:
  Credential Manager 查找 → 命中 → 使用
```

### 2.4 自动更新

```
启动后 5 秒 → 检查 GitHub Releases
之后每 6 小时 → 检查
发现新版本 → JS 事件 "update:available"
用户点击更新 → 下载 .zip → 替换 .exe → 重启
```

---

## 3. 编译环境准备

### 3.1 必需工具

| 工具 | 版本 | 安装方式 | 用途 |
|------|------|----------|------|
| **Go** | 1.26+ | `winget install GoLang.Go` 或 [go.dev](https://go.dev/dl/) | 编译后端 |
| **Node.js** | 22+ | `winget install OpenJS.NodeJS.LTS` | 构建前端 |
| **pnpm** | 10+ | `npm install -g pnpm` | 前端包管理 |
| **Wails CLI** | v2 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` | 桌面打包 |
| **Git** | 任意 | `winget install Git.Git` | 源码拉取 |

### 3.2 Windows 额外依赖

| 依赖 | 说明 | 是否需要安装 |
|------|------|-------------|
| **WebView2 Runtime** | Chromium 内核 | Win11 **已内置**，无需安装 |
| **Microsoft C++ Build Tools** | WebView2 编译依赖 | Wails 需要（通常已随 Visual Studio 安装） |
| **TDM-GCC 或 MinGW** | CGO 编译器 | GoClaw SQLite 用 `modernc.org/sqlite`（纯 Go），**不需要** |

### 3.3 一键安装脚本

将以下内容保存为 `setup-build-env.ps1`：

```powershell
# setup-build-env.ps1 — Windows 11 编译环境一键配置
# 以管理员权限运行: .\setup-build-env.ps1

$ErrorActionPreference = "Stop"

Write-Host "=== GoClaw Desktop 编译环境配置 ===" -ForegroundColor Cyan

# 1. 检查/安装 winget
if (!(Get-Command winget -ErrorAction SilentlyContinue)) {
    Write-Host "[ERROR] 需要 Windows Package Manager (winget)" -ForegroundColor Red
    Write-Host "  从 Microsoft Store 安装 '应用安装程序'" -ForegroundColor Yellow
    exit 1
}

# 2. 安装 Go
if (!(Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "[1/5] 安装 Go..." -ForegroundColor Yellow
    winget install GoLang.Go --accept-package-agreements --accept-source-agreements
    $env:PATH = "$env:LOCALAPPDATA\Programs\Go\bin;$env:PATH"
} else {
    $goVer = (go version) -replace '.*go(\d+\.\d+).*','$1'
    Write-Host "[1/5] Go 已安装: $goVer" -ForegroundColor Green
}

# 3. 安装 Node.js
if (!(Get-Command node -ErrorAction SilentlyContinue)) {
    Write-Host "[2/5] 安装 Node.js..." -ForegroundColor Yellow
    winget install OpenJS.NodeJS.LTS --accept-package-agreements --accept-source-agreements
    $env:PATH = "$env:ProgramFiles\nodejs;$env:PATH"
} else {
    $nodeVer = node --version
    Write-Host "[2/5] Node.js 已安装: $nodeVer" -ForegroundColor Green
}

# 4. 安装 pnpm
if (!(Get-Command pnpm -ErrorAction SilentlyContinue)) {
    Write-Host "[3/5] 安装 pnpm..." -ForegroundColor Yellow
    npm install -g pnpm
} else {
    Write-Host "[3/5] pnpm 已安装" -ForegroundColor Green
}

# 5. 安装 Wails CLI
$wailsPath = "$env:USERPROFILE\go\bin\wails.exe"
if (!(Test-Path $wailsPath)) {
    Write-Host "[4/5] 安装 Wails CLI..." -ForegroundColor Yellow
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
} else {
    Write-Host "[4/5] Wails CLI 已安装" -ForegroundColor Green
}

# 6. 检查 WebView2
$webview2Path = "C:\Program Files (x86)\Microsoft\EdgeWebView\Application"
if (Test-Path $webview2Path) {
    $wvVer = (Get-ChildItem $webview2Path -Directory | Sort-Object -Descending | Select-Object -First 1).Name
    Write-Host "[5/5] WebView2 Runtime 已安装: $wvVer" -ForegroundColor Green
} else {
    Write-Host "[5/5] WebView2 Runtime 未检测到（Win11 应已内置）" -ForegroundColor Yellow
    Write-Host "  下载: https://developer.microsoft.com/en-us/microsoft-edge/webview2/" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "=== 环境配置完成 ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "验证版本:" -ForegroundColor White
go version
node --version
pnpm --version
& $wailsPath version
```

### 3.4 验证环境

```powershell
# 安装完成后验证
go version          # 应输出 go1.26.x
node --version      # 应输出 v22.x.x
pnpm --version      # 应输出 10.x.x
wails version       # 应输出 v2.x.x

# 验证 WebView2（Win11 应已内置）
Get-ChildItem "C:\Program Files (x86)\Microsoft\EdgeWebView\Application" -Directory
```

---

## 4. 编译步骤

### 4.1 拉取源码

```powershell
cd D:\code\backend\notebooklm
git clone https://github.com/nextlevelbuilder/goclaw.git
cd goclaw
```

### 4.2 一键构建

```powershell
# 方式 A：使用 Makefile（推荐，需要 make）
# 安装 make: winget install GnuWin32.Make
make desktop-build

# 方式 B：手动执行（不需要 make）
cd ui\desktop
pnpm install
cd ..\..
go build -tags sqliteonly -ldflags="-s -w" -o goclaw-lite.exe .\ui\desktop\
```

### 4.3 手动分步构建（推荐初学者）

```powershell
# ── 步骤 1: 安装前端依赖 ──
cd D:\code\backend\notebooklm\goclaw\ui\desktop
pnpm install

# ── 步骤 2: 构建前端 ──
pnpm build
# 产出: ui\desktop\frontend\dist\（React 打包文件）

# ── 步骤 3: 回到项目根目录，编译 Go ──
cd D:\code\backend\notebooklm\goclaw

# 编译为 .exe（sqliteonly 标签 + 嵌入前端）
go build -tags sqliteonly -ldflags="-s -w" -o goclaw-lite.exe .\ui\desktop\

# ── 步骤 4: 验证 ──
.\goclaw-lite.exe
# 窗口弹出 → 等待 Gateway 就绪 → 显示 UI
```

### 4.4 使用 Wails CLI 构建（功能最全）

```powershell
cd D:\code\backend\notebooklm\goclaw\ui\desktop

# 开发模式（热重载，改代码自动刷新）
wails dev -tags sqliteonly

# 正式构建
wails build -tags sqliteonly -ldflags="-s -w"

# 产出位置
# ui\desktop\build\bin\goclaw-lite.exe
```

### 4.5 构建产物

```
goclaw-lite.exe            ← 主程序（~30MB）
  ├── 嵌入的 React 前端      （frontend/dist/ 打包在二进制内）
  ├── 嵌入的 Go Gateway     （启动时自动运行）
  ├── 嵌入的 SQLite 驱动     （modernc.org/sqlite，纯 Go，无 DLL）
  └── 嵌入的 migrations     （数据库迁移 SQL）

运行时生成:
  ~/.goclaw/data/goclaw.db       ← SQLite 数据库
  ~/.goclaw/data/goclaw.db-wal   ← WAL 日志
  ~/.goclaw/data/goclaw.db-shm   ← 共享内存
  ~/.goclaw/secrets/             ← 加密密钥（备选存储）
```

---

## 5. 运行说明

### 5.1 首次启动

```
双击 goclaw-lite.exe
  ↓
  系统弹出 WebView2 窗口
  ↓
  后台启动 Gateway（端口 18790）
  ↓
  生成/读取加密密钥（Windows Credential Manager）
  ↓
  创建 SQLite 数据库（~/.goclaw/data/goclaw.db）
  ↓
  显示引导页面（Setup Wizard）
  ↓
  添加 LLM Provider（OpenAI API Key 等）
  ↓
  创建第一个 Agent
  ↓
  开始对话
```

### 5.2 日常使用

| 操作 | 说明 |
|------|------|
| **对话** | 侧边栏 Chat 页面，选择 Agent 开始对话 |
| **Agent 管理** | 创建/编辑/删除 Agent，最多 5 个 |
| **Provider 配置** | 设置 LLM API Key（OpenAI/Anthropic/...）|
| **技能** | 安装/管理技能（Skills） |
| **MCP** | 连接外部 MCP 服务器 |
| **团队** | 创建团队任务看板（最多 1 个团队） |
| **定时任务** | 创建 Cron Job |
| **记忆** | 查看/搜索语义记忆 |
| **追踪** | 查看 LLM 调用追踪 |
| **使用量** | 查看 Token 消耗和成本 |
| **备份** | 导出/导入数据库 |

### 5.3 数据目录结构

```
~/.goclaw/
├── data/
│   ├── goclaw.db           ← 主数据库
│   ├── goclaw.db-wal       ← WAL 日志
│   ├── goclaw.db-shm       ← 共享内存
│   ├── .media/             ← 媒体文件
│   └── skills-store/       ← 技能文件
├── secrets/                ← 密钥（Credential Manager 不可用时的备选）
│   ├── encryption_key
│   └── gateway_token
└── workspace/              ← Agent 工作区
    └── agents/
        └── {agent-id}/
            ├── SOUL.md     ← Agent 灵魂文件
            ├── AGENTS.md   ← Agent 规则
            └── ...
```

### 5.4 配置

桌面版的配置通过 Web UI 的 Config 页面管理，也可直接编辑：

```json
// ~/.goclaw/data/goclaw.db 中的 config_secrets 表
// 或通过环境变量覆盖:

GOCLAW_PORT=18790          # Gateway 端口（默认 18790）
GOCLAW_HOST=127.0.0.1      # 绑定地址（桌面版强制 localhost）
GOCLAW_DATA_DIR=~/.goclaw/data  # 数据目录
GOCLAW_STORAGE_BACKEND=sqlite   # 存储后端（桌面版固定 sqlite）
```

---

## 6. Windows 特殊注意事项

### 6.1 WebView2

Windows 11 **已内置** WebView2 Runtime（Evergreen 模式自动更新），无需额外安装。

如果遇到 WebView2 问题：
```powershell
# 检查 WebView2 是否安装
Get-ChildItem "C:\Program Files (x86)\Microsoft\EdgeWebView\Application"

# 手动下载安装
# https://developer.microsoft.com/en-us/microsoft-edge/webview2/
```

### 6.2 Windows Defender 误报

编译出的 `.exe` 可能被 Windows Defender 标记为"未知发布者"：
- 右键 → 属性 → 解除锁定
- 或在 Windows Defender 中添加排除路径

### 6.3 数据库重置

如果数据库损坏或需要重置：
```
UI 操作: 设置 → 高级 → 重置数据库
命令行: 删除 ~/.goclaw/data/goclaw.db*
```

### 6.4 端口冲突

默认端口 `18790`，如被占用：
```powershell
# 修改端口
$env:GOCLAW_PORT = "19000"
.\goclaw-lite.exe
```

### 6.5 PowerShell 编译脚本

将以下内容保存为 `build-desktop.ps1`：

```powershell
# build-desktop.ps1 — Windows 11 一键编译 GoClaw Lite
# 用法: .\build-desktop.ps1 [-Dev] [-Version "0.1.0"]

param(
    [switch]$Dev,
    [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Desktop = Join-Path $Root "ui\desktop"

Write-Host "=== GoClaw Lite Desktop 编译 ===" -ForegroundColor Cyan
Write-Host "版本: $Version" -ForegroundColor White

# 检查依赖
$deps = @("go", "node", "pnpm", "wails")
foreach ($dep in $deps) {
    if (!(Get-Command $dep -ErrorAction SilentlyContinue)) {
        Write-Host "[ERROR] 缺少 $dep，请先运行 setup-build-env.ps1" -ForegroundColor Red
        exit 1
    }
}

if ($Dev) {
    # 开发模式
    Write-Host "[DEV] 启动热重载开发模式..." -ForegroundColor Yellow
    Push-Location $Desktop
    try {
        wails dev -tags sqliteonly
    } finally {
        Pop-Location
    }
} else {
    # 正式构建
    Write-Host "[1/3] 安装前端依赖..." -ForegroundColor Yellow
    Push-Location $Desktop
    pnpm install
    Pop-Location

    Write-Host "[2/3] 编译..." -ForegroundColor Yellow
    $ldflags = "-s -w -X github.com/nextlevelbuilder/goclaw/cmd.Version=$Version"
    Push-Location $Desktop
    wails build -tags sqliteonly -ldflags="$ldflags"
    Pop-Location

    $exePath = Join-Path $Desktop "build\bin\goclaw-lite.exe"
    if (Test-Path $exePath) {
        $size = [math]::Round((Get-Item $exePath).Length / 1MB, 1)
        Write-Host "[3/3] 构建成功!" -ForegroundColor Green
        Write-Host "  产出: $exePath ($size MB)" -ForegroundColor White
        Write-Host "  运行: .$exePath" -ForegroundColor White
    } else {
        Write-Host "[ERROR] 构建失败" -ForegroundColor Red
        exit 1
    }
}
```

---

## 7. 调试

### 7.1 开发模式日志

```powershell
# 开发模式（带日志输出）
cd ui\desktop
wails dev -tags sqliteonly -loglevel debug

# 日志输出到 stdout，包含:
# - Gateway 启动信息
# - WebSocket 连接
# - LLM 调用
# - 工具执行
```

### 7.2 打开 DevTools

开发模式下：
- 按 `F12` 或右键 → 检查元素
- WebView2 开发者工具（类似 Chrome DevTools）

### 7.3 常见问题

| 问题 | 原因 | 解决 |
|------|------|------|
| `wails: command not found` | Wails CLI 未安装 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| `WebView2 not found` | WebView2 Runtime 缺失 | 安装 [WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) |
| `port 18790 in use` | 端口被占用 | 设置 `$env:GOCLAW_PORT = "19000"` |
| 构建报错 `sqlite` | 缺少 CGO | 不需要 CGO（modernc.org/sqlite 是纯 Go）|
| 前端白屏 | dist 目录未打包 | 先运行 `cd ui\desktop && pnpm build` |
| `credential manager error` | Windows 凭据管理器不可用 | 自动回退到文件存储 `~/.goclaw/secrets/` |

---

## 8. 发布流程

### 8.1 版本号

桌面版使用独立版本号，tag 格式为 `lite-v*`：

```powershell
# 打标签
git tag lite-v0.1.0
git push origin lite-v0.1.0
```

### 8.2 GitHub Actions 自动构建

推送 `lite-v*` 标签后，GitHub Actions 自动：
1. 构建 macOS (.dmg + .tar.gz)
2. 构建 Windows (.zip)
3. 创建 GitHub Release 并上传产物

### 8.3 手动发布

```powershell
# 构建
.\build-desktop.ps1 -Version "0.1.0"

# 打包
Compress-Archive -Path "ui\desktop\build\bin\goclaw-lite.exe" `
                 -DestinationPath "goclaw-lite-0.1.0-windows-amd64.zip"
```

---

## 9. 架构原理图

```
┌─────────────────────────────────────────────────────────────┐
│                    goclaw-lite.exe                           │
│                                                             │
│  ┌─────────────────────── Go 进程 ───────────────────────┐  │
│  │                                                       │  │
│  │  Wails Runtime                                          │  │
│  │    ├── Window Manager (WebView2 窗口)                   │  │
│  │    ├── Event System (Go ↔ JS 事件)                     │  │
│  │    └── Binding (Go 方法暴露到 JS)                       │  │
│  │                                                       │  │
│  │  GoClaw Gateway (goroutine)                            │  │
│  │    ├── HTTP Server (127.0.0.1:18790)                   │  │
│  │    ├── WebSocket Server (/ws)                          │  │
│  │    ├── Agent Router (按 key 分发)                      │  │
│  │    ├── Scheduler (4 Lane 并发控制)                     │  │
│  │    ├── SQLite Store (modernc.org/sqlite)               │  │
│  │    ├── LLM Provider Registry                           │  │
│  │    ├── Tool Registry (50+ 工具)                        │  │
│  │    └── Cron + Heartbeat                                │  │
│  │                                                       │  │
│  │  Secret Manager                                        │  │
│  │    ├── Windows Credential Manager (优先)               │  │
│  │    └── File fallback (~/.goclaw/secrets/)              │  │
│  │                                                       │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌─────────────────── WebView2 (Chromium) ────────────────┐  │
│  │                                                       │  │
│  │  React 前端 (嵌入式)                                   │  │
│  │    ├── WebSocket 客户端 ←→ Gateway /ws                │  │
│  │    ├── HTTP API 客户端 ←→ Gateway /v1/*               │  │
│  │    └── Go Binding 调用 ←→ window.go.*                 │  │
│  │                                                       │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
         ↕ 读写
~/.goclaw/data/goclaw.db (SQLite)
```

**关键理解**：整个桌面应用就是**一个 Go 进程**。Gateway 不是独立进程，而是作为 goroutine 运行在 Wails 应用内部。前端不是独立的浏览器，而是嵌入在窗口中的 WebView2。
