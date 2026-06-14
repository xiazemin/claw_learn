# 沙箱（Sandbox）执行系统深度解析

> 基于 `internal/sandbox/`、`internal/tools/shell.go`、`internal/agent/` 等源码的逐层分析，覆盖完整调用路径、Docker 通信、隔离机制、状态保存与恢复、结果回传。

---

## 1. 沙箱架构总览

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Agent Loop (goroutine)                       │
│                                                                     │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐     │
│  │ LLM Call │───→│Tool Stage│───→│ ExecTool │───→│ Sandbox  │     │
│  │(ThinkStage)│  │          │    │ .Execute │    │ .Exec()  │     │
│  └──────────┘    └──────────┘    └──────────┘    └────┬─────┘     │
│       ▲                                              │            │
│       │            tool result                       │            │
│       └──────────────────────────────────────────────┘            │
│                   (providers.Message{role:"tool"})                 │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     MessageBuffer (in-memory)                       │
│  pending: [user_msg, assistant_msg, tool_result, tool_result, ...]  │
│       ↓                                                             │
│  FlushPending → history[] + pending[] → session store               │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    PostgreSQL (sessions table)                       │
│  session_key │ messages (JSONB) │ summary │ compaction_count │ ...  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. 完整调用路径（代码级）

### 2.1 从 LLM 到 Docker 的完整链路

```
LLM 返回 tool_call: {name: "exec", arguments: {command: "python3 -c 'print(42)'"}}
    │
    ▼
Pipeline.Run() → iteration loop
    │
    ├─ ThinkStage: LLM returns tool_call
    │
    ├─ ToolStage.Execute()
    │   │
    │   ├─ ExecuteToolCall(ctx, state, toolCall)  // loop_pipeline_tool_callbacks.go
    │   │   │
    │   │   ├─ l.normalizeToolCall(tc)            // 重写 MCP 伪调用
    │   │   ├─ l.resolveToolCallName(tc.Name)     // 前缀剥离
    │   │   │
    │   │   ├─ l.executeToolForActor(ctx,         // per-user MCP 工具路由
    │   │   │     "exec", args, channel, chatID,
    │   │   │     peerKind, sessionKey, actorUserID)
    │   │   │   │
    │   │   │   └─ registry.ExecuteWithContext()   // tools/registry.go
    │   │   │       │
    │   │   │       └─ execTool.Execute(ctx, args) // tools/shell.go ★
    │   │   │
    │   │   ├─ l.processToolResult()               // 错误处理、loop detection
    │   │   └─ return providers.Message{role:"tool", content: result.ForLLM}
    │   │
    │   └─ state.Messages.AppendPending(toolMsg)  // 工具结果进入 pending buffer
    │
    ├─ ObserveStage: 检查是否需要继续迭代
    │
    ├─ CheckpointStage: 周期性 flush pending → history
    │
    └─ FinalizeStage.Execute()
        │
        ├─ state.Messages.FlushPending()          // pending → history
        ├─ FlushMessages(ctx, sessionKey, msgs)   // → PostgreSQL sessions.messages
        ├─ MaybeSummarize(ctx, sessionKey)         // 异步摘要（compaction）
        └─ EmitSessionCompleted(...)              // → consolidation pipeline
```

### 2.2 ExecTool.Execute() 内部路由

```go
// tools/shell.go - ExecTool.Execute()

func (t *ExecTool) Execute(ctx context.Context, args map[string] any) *Result {
    command := args["command"].(string)

    // 1. Credentialed exec 检查（SecureCLI 注册的二进制 → 直接 exec，不经过 shell）
    if cred, binary, cmdArgs := t.lookupCredentialedBinary(ctx, command); cred != nil {
        return t.executeCredentialed(ctx, cred, binary, cmdArgs, cwd, sandboxKey, command)
        //   ↓ sandbox 路径               ↓ host 路径
        //   executeCredentialedSandbox()  executeCredentialedHost()
    }

    // 2. 安全策略：deny patterns + approval 检查
    //    ...

    // 3. 沙箱路由判断
    sandboxKey := ToolSandboxKeyFromCtx(ctx)
    if t.sandboxMgr != nil && sandboxKey != "" {
        return t.executeInSandbox(ctx, command, cwd, sandboxKey)  // ★ 沙箱路径
    }

    // 4. 宿主机直接执行
    return t.executeOnHost(ctx, command, cwd)
}
```

---

## 3. Docker 通信详解

### 3.1 容器创建（`docker run`）

当 `DockerManager.Get()` 首次被调用时：

```go
// sandbox/docker.go - newDockerSandbox()

func newDockerSandbox(ctx, name, cfg, workspace) (*DockerSandbox, error) {
    args := []string{
        "run", "-d",                        // 后台运行
        "--name", name,                     // 容器名: goclaw-sbx-{sanitized-key}
        "--label", "goclaw.sandbox=true",   // 标签
    }

    // === 安全加固 ===
    args = append(args, "--read-only")                    // 只读根文件系统
    args = append(args, "--tmpfs", "/tmp:noexec,nosuid,nodev,size=64m")  // tmpfs
    args = append(args, "--tmpfs", "/var/tmp:noexec,nosuid,nodev")
    args = append(args, "--tmpfs", "/run:noexec,nosuid,nodev")
    args = append(args, "--cap-drop", "ALL")              // 丢弃所有 capabilities
    args = append(args, "--security-opt", "no-new-privileges")  // 禁止提权
    args = append(args, "--user", "1000:1000")            // 非 root 用户

    // === 资源限制 ===
    args = append(args, "--memory", "512m")               // 内存限制
    args = append(args, "--cpus", "1.0")                  // CPU 限制
    args = append(args, "--pids-limit", "256")            // 进程数限制

    // === 网络隔离 ===
    args = append(args, "--network", "none")              // 禁用网络

    // === Workspace 挂载 ===
    hostPath := resolveHostWorkspacePath(ctx, workspace)  // DooD 路径解析
    args = append(args, "-v", fmt.Sprintf("%s:%s:rw", hostPath, "/workspace"))
    args = append(args, "-w", "/workspace")               // 工作目录

    // === 环境变量 ===
    for k, v := range cfg.Env {
        args = append(args, "-e", k+"="+v)
    }

    // === 启动命令：sleep infinity（保持容器存活）===
    args = append(args, cfg.Image, "sleep", "infinity")

    cmd := exec.CommandContext(ctx, "docker", args...)
    cmd.Run()  // docker run -d ...

    // 可选：执行 setup_command
    if cfg.SetupCommand != "" {
        exec.Command("docker", "exec", "-i", containerID, "sh", "-lc", cfg.SetupCommand).Run()
    }
}
```

**执行的 Docker 命令**：
```bash
docker run -d \
    --name goclaw-sbx-telegram-123-alice \
    --label goclaw.sandbox=true \
    --read-only \
    --tmpfs /tmp:noexec,nosuid,nodev,size=64m \
    --tmpfs /var/tmp:noexec,nosuid,nodev \
    --tmpfs /run:noexec,nosuid,nodev \
    --cap-drop ALL \
    --security-opt no-new-privileges \
    --user 1000:1000 \
    --memory 512m \
    --cpus 1.0 \
    --pids-limit 256 \
    --network none \
    -v /host/path/to/workspace:/workspace:rw \
    -w /workspace \
    goclaw-sandbox:bookworm-slim sleep infinity
```

### 3.2 命令执行（`docker exec`）

```go
// sandbox/docker.go - DockerSandbox.Exec()

func (s *DockerSandbox) Exec(ctx, command, workDir, opts) (*ExecResult, error) {
    timeout := 5 * time.Minute  // 沙箱默认超时

    args := []string{"exec"}

    // 注入环境变量（credentialed exec 的凭证）
    for k, v := range o.Env {
        args = append(args, "-e", k+"="+v)  // docker exec -e KEY=VALUE
    }

    if workDir != "" {
        args = append(args, "-w", workDir)  // docker exec -w /workspace/user-123
    }

    args = append(args, s.containerID)       // docker exec -i <container_id>
    args = append(args, command...)           // docker exec -i <id> sh -c "python3 -c 'print(42)'"

    cmd := exec.CommandContext(execCtx, "docker", args...)

    // 输出限制：1MB 防止 OOM
    stdout := &limitedBuffer{max: 1 << 20}
    stderr := &limitedBuffer{max: 1 << 20}
    cmd.Stdout = stdout
    cmd.Stderr = stderr

    cmd.Run()

    return &ExecResult{
        ExitCode: exitCode,
        Stdout:   stdout.String(),
        Stderr:   stderr.String(),
    }
}
```

**执行的 Docker 命令**：
```bash
# 普通 exec
docker exec -w /workspace goclaw-sbx-telegram-123-alice sh -c "python3 -c 'print(42)'"

# Credentialed exec（注入凭证）
docker exec -e GITHUB_TOKEN=ghp_xxx -e GITHUB_HOST=github.com \
    -w /workspace goclaw-sbx-xxx /usr/bin/gh repo list
```

### 3.3 文件操作（FsBridge）

沙箱内的文件操作通过 `docker exec` 实现，而非 host 文件系统直接访问：

```go
// sandbox/fsbridge.go - FsBridge

func (b *FsBridge) ReadFile(ctx, path) (string, error) {
    // docker exec <id> cat -- /workspace/file.txt
    stdout, stderr, exitCode, _ := b.dockerExec(ctx, nil, "cat", "--", realPath)
    return stdout, nil
}

func (b *FsBridge) WriteFile(ctx, path, content, appendMode) error {
    // docker exec -i <id> tee [--append] -- /workspace/file.txt
    // stdin: content bytes
    b.dockerExec(ctx, []byte(content), "tee", "--", resolved)
}

func (b *FsBridge) ListDir(ctx, path) (string, error) {
    // docker exec <id> ls -la -- /workspace/
    stdout, _, _, _ := b.dockerExec(ctx, nil, "ls", "-la", "--", realPath)
    return stdout, nil
}

func (b *FsBridge) dockerExec(ctx, stdin, args) (stdout, stderr, exitCode, err) {
    dockerArgs := []string{"exec"}
    if stdin != nil {
        dockerArgs = append(dockerArgs, "-i")  // 交互模式（写文件用）
    }
    dockerArgs = append(dockerArgs, b.containerID)
    dockerArgs = append(dockerArgs, args...)

    cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
    if stdin != nil {
        cmd.Stdin = bytes.NewReader(stdin)  // 通过 stdin 传递文件内容
    }
    cmd.Run()
}
```

**FsBridge 安全机制**：
- `resolvePath()`: 所有路径相对 `/workspace` 解析，防止路径逃逸
- `resolveExistingPath()`: 用 `realpath -e` 验证最终路径在 workdir 内
- `validateParentBeforeCreate()`: 逐级验证父目录不会逃逸

---

## 4. 隔离机制

### 4.1 三个维度的隔离

```
┌─────────────────────────────────────────────────────────┐
│                   隔离维度矩阵                            │
├─────────────┬───────────────────────────────────────────┤
│  Mode       │ 控制哪些 Agent 使用沙箱                      │
│  "off"      │ 不使用沙箱                                  │
│  "non-main" │ 除 "main" Agent 外全部沙箱化                │
│  "all"      │ 所有 Agent 都沙箱化                         │
├─────────────┼───────────────────────────────────────────┤
│  Scope      │ 控制容器复用粒度（隔离级别）                   │
│  "session"  │ 每个 session 一个容器（最大隔离）             │
│  "agent"    │ 每个 agent 一个容器（中等隔离）               │
│  "shared"   │ 所有 agent 共享一个容器（最小隔离）            │
├─────────────┼───────────────────────────────────────────┤
│  Workspace  │ 控制文件系统访问权限                         │
│  "none"     │ 无文件系统访问                               │
│  "ro"       │ 只读挂载 workspace                          │
│  "rw"       │ 读写挂载 workspace（默认）                   │
└─────────────┴───────────────────────────────────────────┘
```

### 4.2 Scope Key 解析（容器名唯一性）

```go
// sandbox/sandbox.go - ResolveScopeKey()

func (c Config) ResolveScopeKey(sessionKey string) string {
    switch c.Scope {
    case ScopeShared:
        return "shared"                           // 所有人共享同一个容器
    case ScopeAgent:
        parts := strings.SplitN(sessionKey, ":", 3)
        if len(parts) >= 2 {
            return "agent:" + parts[1]            // 同一个 agent 共享容器
        }
        return "agent:default"
    default: // ScopeSession
        if sessionKey == "" {
            return "default"
        }
        return sessionKey                         // 每个 session 独立容器
    }
}

// sandboxKey 通过 context 传递（线程安全）
ctx = tools.WithSandboxKey(ctx, sandboxKey)
// ExecTool 从 ctx 读取
sandboxKey := ToolSandboxKeyFromCtx(ctx)
```

**示例**：
- Session scope: 容器名 `goclaw-sbx-telegram-agent-default-alice-direct-1718200000`
- Agent scope: 容器名 `goclaw-sbx-agent-default`
- Shared scope: 容器名 `goclaw-sbx-shared`

### 4.3 Docker 层面的安全加固

| 安全措施 | Docker 参数 | 作用 |
|---------|------------|------|
| 只读根文件系统 | `--read-only` | 容器内无法修改系统文件 |
| 丢弃所有 capabilities | `--cap-drop ALL` | 禁用 root 权限操作 |
| 禁止提权 | `--security-opt no-new-privileges` | 禁止 setuid/setgid 提权 |
| 非 root 用户 | `--user 1000:1000` | 以普通用户运行 |
| 内存限制 | `--memory 512m` | 防止 OOM 影响宿主 |
| CPU 限制 | `--cpus 1.0` | 防止 CPU 耗尽 |
| 进程数限制 | `--pids-limit 256` | 防止 fork bomb |
| 网络隔离 | `--network none` | 完全禁用网络访问 |
| tmpfs | `--tmpfs /tmp:noexec,nosuid,nodev` | 临时文件系统，禁止执行 |

### 4.4 Workspace 路径映射（DooD）

当 GoClaw 自身运行在 Docker 容器内时（Docker-out-of-Docker），需要将容器内路径映射到宿主机路径：

```go
// sandbox/docker_resolve.go - resolveHostWorkspacePath()

// 1. 检测是否在容器内（/.dockerenv）
// 2. 检测当前容器 ID（/proc/self/mountinfo → docker container ID）
// 3. docker inspect --format '{{json .Mounts}}' <containerID>
// 4. 解析 mount 列表，找到匹配的 Source → 返回宿主机路径

// 例如：GoClaw 容器挂载了 -v /data:/app/data
// workspace=/app/workspace → 宿主机路径=/data/workspace
```

---

## 5. 状态保存与恢复

### 5.1 容器生命周期管理

```
┌─────────────────────────────────────────────────────────────┐
│                  容器生命周期                                  │
│                                                              │
│  Get() 首次调用                                               │
│    → newDockerSandbox()                                      │
│      → docker run -d --name goclaw-sbx-xxx ...               │
│      → sleep infinity（保持存活）                              │
│    → manager.sandboxes[key] = sandbox                        │
│                                                              │
│  Get() 后续调用                                               │
│    → 检查 sandboxes[key] 是否存在                             │
│    → 存在 → 直接返回复用                                      │
│    → 不存在 → 创建新容器                                      │
│                                                              │
│  Release(key)                                                │
│    → docker rm -f <containerID>                              │
│    → delete(sandboxes, key)                                  │
│                                                              │
│  Pruning（后台 goroutine，每 5 分钟）                          │
│    → 检查 lastUsed > 24h → destroy                           │
│    → 检查 createdAt > 7d → destroy                           │
│                                                              │
│  ReleaseAll()（优雅关闭）                                      │
│    → 遍历所有 sandboxes → 逐个 docker rm -f                   │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 容器内状态保持

**关键点：沙箱容器在 Agent Run 之间保持存活（`sleep infinity`），因此：**

1. **容器内文件系统持久**：Agent A 写入的文件，下次 Agent A 调用时仍可读取（同一 scope key）
2. **安装的包持久**：`setup_command` 安装的依赖在容器生命周期内可用
3. **进程状态不持久**：容器重启后，所有进程丢失（仅 `sleep infinity` 保活）

```
Session Scope:
  Run 1: [创建容器] → [exec: pip install pandas] → [exec: python3 ...] → [容器保持存活]
  Run 2: [复用容器] → [exec: python3 -c "import pandas"] ← 可以！（pandas 仍在）
  Run 3: [复用容器] → [exec: python3 new_script.py] ← 可以！（文件系统还在）
  ...
  Pruning: 闲置 > 24h → [docker rm -f] → 容器销毁

Agent Scope:
  Agent A 的所有 session 共享同一个容器 → 文件和包可跨 session 共享
  Agent B 有独立容器 → 完全隔离

Shared Scope:
  所有 Agent 共享一个容器 → 文件共享（最大隔离最小）
```

### 5.3 容器恢复（崩溃后）

```go
// cmd/gateway.go - 启动时清理卡住的 summoning 状态
if n, err := pgStores.Agents.ResetStuckSummoning(context.Background()); err != nil {
    slog.Warn("agents.reset_stuck_summoning_failed", "err", err)
} else if n > 0 {
    slog.Info("agents.reset_stuck_summoning", "count", n)
}

// 关闭时释放所有容器
func (d *gatewayDeps) runLifecycle(...) {
    // ...
    go func() {
        sig := <-deps.sigCh  // SIGINT/SIGTERM
        // ...
        deps.sandboxMgr.Stop()                    // 停止 pruning goroutine
        deps.sandboxMgr.ReleaseAll(context.Background())  // docker rm -f 所有容器
        // ...
    }()
}
```

---

## 6. 结果回传到 LLM 上下文

### 6.1 工具结果如何变成 LLM 的输入

```
Step 1: ExecTool.Execute() 返回 *Result
    │
    │  Result{
    │    ForLLM: "42",           ← 给 LLM 看的文本
    │    ForUser: "",            ← 给用户看的文本（非空则通知用户）
    │    IsError: false,
    │    MediaResults: [...],    ← 媒体文件
    │  }
    │
    ▼
Step 2: processToolResult() 包装为 providers.Message
    │
    │  providers.Message{
    │    Role:       "tool",
    │    Content:    result.ForLLM,     // "42"
    │    ToolCallID: tc.ID,            // 关联到对应的 tool_call
    │    IsError:    result.IsError,
    │  }
    │
    ▼
Step 3: state.Messages.AppendPending(toolMsg)
    │
    │  MessageBuffer.pending = [
    │    user_msg,
    │    assistant_msg(tool_calls),
    │    tool_result_msg,        ← 新增
    │  ]
    │
    ▼
Step 4: 下次 LLM 调用时，messages = All()
    │
    │  messages = [
    │    {system: "You are..."},
    │    {history...},
    │    {user: "what is 6*7?"},
    │    {assistant, tool_calls: [{id:"call_1", name:"exec", args:{command:"echo $((6*7))"}}]},
    │    {role:"tool", tool_call_id:"call_1", content:"42"},  ← LLM 看到工具结果
    │  ]
    │
    ▼
Step 5: LLM 基于工具结果生成回复
    │
    │  {assistant: "6*7 = 42"}
    │
    ▼
Step 6: FinalizeStage → FlushPending → PostgreSQL
    │
    │  sessions.messages = [user, assistant(tool_calls), tool_result, assistant(reply)]
    └─→ 持久化到数据库
```

### 6.2 MessageBuffer 机制

```go
// pipeline/message_buffer.go

type MessageBuffer struct {
    system  providers.Message   // 系统提示词（每次迭代重建）
    history []providers.Message // 持久化的历史消息
    pending []providers.Message // 本轮新增消息（checkpoints 时 flush）
}

// 所有 LLM 调用的消息来源
func (mb *MessageBuffer) All() []providers.Message {
    out := make([]providers.Message, 0, 1+len(mb.history)+len(mb.pending))
    out = append(out, mb.system)       // system prompt
    out = append(out, mb.history...)   // 历史消息
    out = append(out, mb.pending...)   // 新消息（含工具结果）
    return out
}

// FlushPending: pending → history（checkpoint 或 finalize 时调用）
func (mb *MessageBuffer) FlushPending() []providers.Message {
    flushed := mb.pending
    mb.history = append(mb.history, mb.pending...)
    mb.pending = nil
    return flushed
}
```

---

## 7. 上下文存储

### 7.1 Session 存储结构

```
PostgreSQL sessions 表:
┌──────────────┬────────────────────────────────────────┬─────────┬───────────┐
│ session_key  │ messages (JSONB)                       │ summary │ updated_at│
├──────────────┼────────────────────────────────────────┼─────────┼───────────┤
│ agent:default│ [{role:"system", content:"..."},       │ ""      │ timestamp │
│ :telegram    │  {role:"user", content:"hello"},       │         │           │
│ :direct:     │  {role:"assistant", tool_calls:[...]}, │         │           │
│ 1718200000   │  {role:"tool", content:"42"},          │         │           │
│              │  {role:"assistant", content:"..."}]    │         │           │
└──────────────┴────────────────────────────────────────┴─────────┴───────────┘
```

### 7.2 消息持久化流程

```go
// PGSessionStore - 内存缓存 + 数据库

// 1. AddMessage: 追加到内存缓存
func (s *PGSessionStore) AddMessage(ctx, key, msg) {
    data := s.getOrInit(ctx, key)   // 不存在则创建
    data.Messages = append(data.Messages, msg)
    data.Updated = time.Now()
}

// 2. FlushMessages（FinalizeStage 调用）
//    → 批量写入 PostgreSQL
//    → UPDATE sessions SET messages = $1, updated_at = $2 WHERE session_key = $3

// 3. 下次加载
func (s *PGSessionStore) GetHistory(ctx, key) []providers.Message {
    // 优先从内存缓存读取
    if data, ok := s.cache[key]; ok {
        return data.Messages
    }
    // 缓存未命中 → loadFromDB → 缓存
}
```

### 7.3 Compaction（上下文压缩）

当消息数量超过 context window 限制时，触发 compaction：

```go
// agent/loop_compact.go - compactMessagesInPlace()

// 将前 70% 的消息摘要化，保留最后 30%
func (l *Loop) compactMessagesInPlace(ctx, messages) []providers.Message {
    splitIdx := len(messages) - keepCount  // 保留最后几条

    // 1. 将 splitIdx 之前的全部消息交给 LLM 摘要
    toSummarize := messages[:splitIdx]
    summary := callLLM(compactionSummaryPrompt + toSummarize)

    // 2. 用摘要替换原始消息
    messages = [{role:"system", content: summary}, messages[splitIdx:]...]

    // 3. 更新 session.summary
    session.SetSummary(summary)

    return messages
}
```

### 7.4 上下文注入链（Context Setup）

```go
// agent/loop_context.go - injectContext()

func (l *Loop) injectContext(ctx, req) (contextSetupResult, error) {
    // 1. 注入 Agent 身份
    ctx = store.WithAgentID(ctx, l.agentUUID)
    ctx = store.WithAgentKey(ctx, l.id)
    ctx = store.WithTenantID(ctx, l.tenantID)
    ctx = store.WithUserID(ctx, req.UserID)

    // 2. 注入 Workspace（分层解析）
    //    tenant → team → project → user/chat
    effectiveWorkspace := tools.ResolveWorkspace(ws,
        tools.UserChatLayer(tools.SanitizePathSegment(req.UserID), shared),
    )
    ctx = tools.WithToolWorkspace(ctx, effectiveWorkspace)

    // 3. 注入 Sandbox 配置
    ctx = tools.WithSandboxConfig(ctx, l.sandboxCfg)

    // 4. 注入 Shell deny groups（安全策略）
    ctx = store.WithShellDenyGroups(ctx, l.shellDenyGroups)

    // 5. 构建 RunContext 快照
    rc := &store.RunContext{
        AgentID:      l.agentUUID,
        TenantID:     l.tenantID,
        UserID:       req.UserID,
        Workspace:    tools.ToolWorkspaceFromCtx(ctx),
        SandboxCfg:   l.sandboxCfg,
        // ...
    }
    ctx = store.WithRunContext(ctx, rc)

    return contextSetupResult{ctx: ctx}, nil
}
```

---

## 8. 完整数据流图

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         完整消息生命周期                                  │
│                                                                         │
│  ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐     │
│  │ Telegram │────→│ Channel  │────→│ Message  │────→│ Consumer │     │
│  │ Webhook  │     │ .Start() │     │ Bus      │     │ goroutine│     │
│  └──────────┘     │ goroutine│     │ .inbound │     │          │     │
│                   └──────────┘     │ (1000)   │     │          │     │
│                                    └──────────┘     └────┬─────┘     │
│                                                          │            │
│                                                          ▼            │
│                                           ┌──────────────────────┐    │
│                                           │   Scheduler          │    │
│                                           │   SessionQueue       │    │
│                                           │   Lane "main" (30)   │    │
│                                           └──────────┬───────────┘    │
│                                                      │                │
│                                                      ▼                │
│  ┌────────────────────────────────────────────────────────────────┐   │
│  │                    Agent Loop (goroutine)                       │   │
│  │                                                                 │   │
│  │  ┌─────────────────────────────────────────────────────────┐   │   │
│  │  │ Pipeline.Run()                                          │   │   │
│  │  │                                                         │   │   │
│  │  │ ContextStage:                                           │   │   │
│  │  │   injectContext() → LoadSessionHistory() → BuildMessages│   │   │
│  │  │   → [system, history, user_msg]                        │   │   │
│  │  │                                                         │   │   │
│  │  │ ┌─── Iteration Loop ─────────────────────────────────┐ │   │   │
│  │  │ │ ThinkStage: callLLM(messages) → tool_calls         │ │   │   │
│  │  │ │ PruneStage: context pruning (if over budget)       │ │   │   │
│  │  │ │ ToolStage:                                         │ │   │   │
│  │  │ │   ┌─────────────────────────────────────────────┐  │ │   │   │
│  │  │ │   │ executeToolForActor("exec", {command:...})  │  │ │   │   │
│  │  │ │   │   │                                         │  │ │   │   │
│  │  │ │   │   ├─ sandbox_key from ctx?                  │  │ │   │   │
│  │  │ │   │   │  YES: executeInSandbox()                │  │ │   │   │
│  │  │ │   │   │     │                                   │  │ │   │   │
│  │  │ │   │   │     ├─ DockerManager.Get(key)           │  │ │   │   │
│  │  │ │   │     │   │  → create/reuse container         │  │ │   │   │
│  │  │ │   │   │     │                                   │  │ │   │   │
│  │  │ │   │   │     ├─ SandboxCwd() → /workspace/user   │  │ │   │   │
│  │  │ │   │     │                                       │  │ │   │   │
│  │  │ │   │     │   ├─ docker exec -w /workspace/user   │  │ │   │   │
│  │  │ │   │     │   │   <container> sh -c "command"     │  │ │   │   │
│  │  │ │   │     │   │                                   │  │ │   │   │
│  │  │ │   │     │   └─ Result{ExitCode, Stdout, Stderr} │  │ │   │   │
│  │  │ │   │     │                                       │  │ │   │   │
│  │  │ │   │   NO: executeOnHost()                       │  │ │   │   │
│  │  │ │   │     → exec.Command("/bin/sh", "-c", cmd)    │  │ │   │   │
│  │  │ │   │                                             │  │ │   │   │
│  │  │ │   │   └─ → Result{ForLLM: "42", IsError: false} │  │ │   │   │
│  │  │ │   └─────────────────────────────────────────────┘  │ │   │   │
│  │  │ │                                                     │ │   │   │
│  │  │ │   → providers.Message{role:"tool", content:"42"}    │ │   │   │
│  │  │ │   → state.Messages.AppendPending(toolMsg)           │ │   │   │
│  │  │ │                                                     │ │   │   │
│  │  │ │ ObserveStage: check if done                         │ │   │   │
│  │  │ │ CheckpointStage: FlushPending → history (periodic)  │ │   │   │
│  │  │ └─────────────────────────────────────────────────────┘ │   │   │
│  │  │                                                         │   │   │
│  │  │ FinalizeStage:                                          │   │   │
│  │  │   1. FlushPending → history                             │   │   │
│  │  │   2. FlushMessages → PostgreSQL sessions.messages        │   │   │
│  │  │   3. MaybeSummarize → async compaction                   │   │   │
│  │  │   4. EmitSessionCompleted → consolidation pipeline       │   │   │
│  │  │   → domainBus → episodicWorker → semanticWorker         │   │   │
│  │  │                                                         │   │   │
│  │  └─────────────────────────────────────────────────────────┘   │   │
│  │                                                                 │   │
│  │  emit(AgentEventRunCompleted, content: "6*7 = 42")             │   │
│  └──────────────────────────┬──────────────────────────────────────┘   │
│                             │                                          │
│                             ▼                                          │
│  ┌────────────────────────────────────────────────────────────────┐   │
│  │  eventPub.Broadcast({Name:"chat", Payload:OutboundMessage})    │   │
│  │     │                                                          │   │
│  │     ├─→ msgBus.subscribers → WebSocket clients (实时推送)       │   │
│  │     └─→ msgBus.outbound → dispatchOutbound goroutine           │   │
│  │                    │                                            │   │
│  │                    ▼                                            │   │
│  │          channel.Send(ctx, OutboundMessage)                    │   │
│  │          → Telegram API: sendMessage                           │   │
│  └────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 9. Credentialed Exec（安全凭证执行）

### 9.1 两条执行路径

```
execTool.Execute(command)
    │
    ├─ lookupCredentialedBinary(command)
    │   → 发现 "gh" 已注册 SecureCLI 凭证
    │
    ├─ 在沙箱中？
    │   ├─ YES: executeCredentialedSandbox()
    │   │   → DockerManager.Get(sandboxKey)
    │   │   → sandbox.Exec(ctx, ["/usr/bin/gh", "repo", "list"], cwd,
    │   │         WithEnv{"GITHUB_TOKEN": "ghp_xxx"})
    │   │   → docker exec -e GITHUB_TOKEN=ghp_xxx <id> /usr/bin/gh repo list
    │   │
    │   └─ NO: executeCredentialedHost()
    │       → cmd = exec.Command("/usr/bin/gh", "repo", "list")
    │       → cmd.Env = ["PATH=...", "HOME=...", "GITHUB_TOKEN=ghp_xxx"]
    │       → cmd.Run()
    │
    └─ 结果 → ScrubCredentialsCtx(output)  // 净化凭证泄露
            → capExecOutput(output)        // 截断到最大长度
            → SilentResult(output)
```

### 9.2 凭证注入方式

```go
// 沙箱模式：通过 docker exec -e 注入（无 shell 解释）
func (t *ExecTool) executeCredentialedSandbox(ctx, absPath, args, cwd, sandboxKey, envMap, timeout) {
    command := append([]string{absPath}, args...)
    result, err := sb.Exec(ctx, command, cwd, sandbox.WithEnv(envMap))
    // → docker exec -e GITHUB_TOKEN=ghp_xxx -e GITHUB_HOST=github.com <id> /usr/bin/gh repo list
}

// 宿主机模式：通过 exec.Cmd.Env 注入（无 shell 解释）
func (t *ExecTool) executeCredentialedHost(ctx, absPath, args, cwd, envMap, timeout) {
    cmd := exec.Command(absPath, args...)  // 直接调用二进制
    cmd.Env = buildCredentialedEnv(envMap) // PATH + HOME + 凭证环境变量
    cmd.Run()
}
```

---

## 10. 关键设计决策总结

| 设计决策 | 实现方式 | 原因 |
|---------|---------|------|
| 容器保持存活 | `sleep infinity` | 避免每次 exec 都重建容器（启动开销） |
| 通过 Docker CLI 通信 | `exec.Command("docker", ...)` | 不依赖 Docker SDK，简化部署 |
| 只读根 + tmpfs | `--read-only` + `--tmpfs` | 最小化攻击面 |
| 输出限制 | `limitedBuffer{max:1MB}` | 防止恶意命令产生巨量输出导致 OOM |
| 路径逃逸防护 | `realpath -e` + 前缀检查 | 防止 symlink/相对路径攻击 |
| 净化凭证泄露 | `ScrubCredentialsCtx()` | 防止工具输出中包含密钥 |
| Scope 复用容器 | `DockerManager.Get(key)` | 同一 session/agent 的文件和包可复用 |
| 异步 Pruning | 后台 goroutine 定期清理 | 防止容器泄漏 |
