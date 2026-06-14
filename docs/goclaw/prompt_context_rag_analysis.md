# 提示词与上下文管理完整流程分析

> 基于 goclaw 源码的逐层分析，覆盖系统提示词构建、上下文分层、RAG 记忆检索、PG 向量化完整流程。

---

## 1. 系统提示词构建的分层逻辑

### 1.1 整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                    System Prompt 构建流程                             │
│                                                                     │
│  ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐  │
│  │ Context  │────→│ build    │────→│ Build    │────→│ AutoInject│ │
│  │ Stage    │     │ Messages │     │ System   │     │ (RAG L0)  │ │
│  │          │     │ (loop_   │     │ Prompt   │     │           │ │
│  │          │     │ history) │     │          │     │           │ │
│  └──────────┘     └──────────┘     └──────────┘     └──────────┘  │
│       │                │                │                │          │
│       ▼                ▼                ▼                ▼          │
│  injectContext    resolveContext    BuildPrompt()    episodicStore  │
│  loadSession      Files + mode     → string          .Search()     │
│  loadHistory      + persona        → append to       → append to   │
│                                     system prompt    system prompt │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.2 BuildSystemPrompt 的 16 个 Section（按顺序）

```go
// internal/agent/systemprompt.go - BuildSystemPrompt()

func BuildSystemPrompt(cfg SystemPromptConfig) string {
    // 1. Identity — 渠道感知身份
    //    "You are a personal assistant running in Telegram (a direct chat)."

    // 1.5. First-run bootstrap override
    //    首次运行：强制模型读 BOOTSTRAP.md 进行引导
    //    USER.md 未填充：持续提示收集用户信息

    // 1.7. Persona — SOUL.md + IDENTITY.md 注入
    //    persona 文件被 splitPersonaFiles 拆分到 prompt 最前面（首因效应）

    // 2. ## Tooling — 工具列表
    //    所有可用工具的名称和简述，沙箱提示

    // 2.1. ## Execution Bias — 执行偏好
    //    鼓励直接执行而非反复确认

    // 2.3. ## Tool Call Style — 调用风格
    //    工具调用格式规范

    // 2.5. Credentialed CLI context — 凭证 CLI 上下文

    // 2.6. ## Voice Response — 语音回复（TTS tagged 模式）

    // 3. ## Safety — 安全准则
    //    prompt injection 防御、身份锚定、自我进化规则

    // 3.5. ## Self-Evolution — 自我进化规则

    // 4. ## Skills — 技能系统
    //    inline / search / hybrid 三种模式

    // 4.5. ## MCP Tools — MCP 外部工具

    // 5. ## Delegation Targets — 编排目标（委派模式）

    // 6. ## Workspace — 工作目录
    //    沙箱感知路径

    // 6.3. ## Team Workspace — 团队工作区

    // 6.4. ## Team Members — 团队成员列表

    // 7. ## User Identity — 用户身份

    // 12.5. ## Memory Recall — 记忆检索指南

    // ── CACHE BOUNDARY ── （以上稳定，以下每次变化）

    // 8. Time — 当前时间
    // 9.5. Channel formatting hints — 渠道格式提示
    // 9.6. Group chat reply hint — 群聊回复提示
    // 10. Extra system prompt — 额外上下文（<extra_context>标签包裹）
    // 11b. # Project Context — 动态文件（USER.md, BOOTSTRAP.md）
    // 13. ## Sub-Agent Spawning — 子 Agent 生成
    // 15. ## Runtime — 运行时信息
    // 16. Recency reinforcements — 人格强化提醒

    // ★ AutoInject: L0 记忆上下文追加到 system prompt 末尾
    return result
}
```

### 1.3 提示词模式（Mode）分层

```
┌────────────┬─────────────────────────────────────────────────────────┐
│  Mode      │ 加载的 Section                                          │
├────────────┼─────────────────────────────────────────────────────────┤
│  "full"    │ 所有 16 个 Section（默认）                               │
│  "task"    │ Persona + Tooling + Safety(slim) + Skills + Workspace   │
│            │ + Memory(slim) + Context Files                          │
│  "minimal" │ AGENTS_CORE.md + CAPABILITIES.md + Pinned Skills       │
│            │ + Memory(minimal)                                       │
│  "none"    │ TOOLS.md only + Pinned Skills（身份隔离 prompt）         │
└────────────┴─────────────────────────────────────────────────────────┘
```

### 1.4 Mode 解析逻辑

```go
// 3 层优先级: runtime > auto-detect > config
mode := resolvePromptMode("", sessionKey, l.promptMode)

// auto-detect 规则:
// - subagent session → "minimal"
// - cron/heartbeat session → "minimal"  
// - team dispatch session → "task"
// - 否则 → config 中的默认值（通常是 "full"）
```

---

## 2. ContextStage 完整执行流程

```go
// pipeline/context_stage.go - ContextStage.Execute()

func (s *ContextStage) Execute(ctx, state) error {
    // === Phase 1: Hook 触发 ===
    // Hook: SessionStart (async) — 通知外部系统
    // Hook: UserPromptSubmit (sync) — 可拦截/改写用户消息

    // === Phase 2: Context 注入 ===
    injectContext(ctx, input)  // agent/tenant/user/workspace/tool 隔离

    // === Phase 3: Context Window 解析 ===
    ResolveContextWindow(provider, model)  // 一次性确定模型上下文窗口大小

    // === Phase 4: Workspace 解析 ===
    ResolveWorkspace(ctx, input)  // 分层路径: tenant → team → user/chat

    // === Phase 5: Context Files 加载 ===
    LoadContextFiles(ctx, userID)
    // → 加载 AGENTS.md, SOUL.md, USER.md, TOOLS.md, IDENTITY.md,
    //   BOOTSTRAP.md, CAPABILITIES.md 等
    // → 模式过滤: ModeAllowlist(mode) → 只保留允许的文件

    // === Phase 6: Session History 加载 ===
    LoadSessionHistory(ctx, sessionKey)
    // → PostgreSQL sessions.messages (JSONB) → []providers.Message
    // → sessions.summary (compaction 摘要)

    // === Phase 7: Messages 构建 ===
    BuildMessages(ctx, input, history, summary)
    // → 构建 system prompt (BuildSystemPrompt)
    // → 合并 history
    // → 返回 [system_msg, history...]

    // === Phase 8: Tool Schema 计算 ===
    BuildFilteredTools(state)
    // → 按模式过滤工具 + 渠道感知过滤

    // === Phase 9: Overhead Tokens 计算 ===
    TokenCounter.CountMessages(model, [system_msg]) + CountToolSchemas(model, tools)
    // → system prompt + tool schema 的 token 数量
    // → PruneStage 用此值计算剩余预算

    // === Phase 10: Media 富化 ===
    EnrichMedia(ctx, state)

    // === Phase 11: Team Reminders 注入 ===
    InjectReminders(ctx, input, history)

    // === Phase 12: ★ AutoInject (RAG L0 记忆) ===
    AutoInject(ctx, userMessage, userID, recentContext)
    // → 搜索 episodic_summaries 表
    // → 返回 L0 摘要文本
    // → 追加到 system prompt 末尾
}
```

---

## 3. RAG 记忆检索完整流程

### 3.1 三级记忆架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                        三级记忆架构                                  │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  L0: Episodic Summaries (episodic_summaries 表)             │   │
│  │  - 预计算的 ~50 token 摘要                                   │   │
│  │  - 每次对话自动生成（session.completed 事件触发）              │   │
│  │  - 自动注入 system prompt（无需工具调用）                      │   │
│  │  - 向量 + FTS 混合搜索                                       │   │
│  └───────────────────────┬─────────────────────────────────────┘   │
│                          │ memory_expand 工具触发深层检索            │
│                          ▼                                          │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  L1: Semantic Memory (memory_chunks 表)                      │   │
│  │  - 语义化的文档片段（由 dreaming worker 从 episodic 合成）     │   │
│  │  - 向量 + FTS 混合搜索                                       │   │
│  │  - memory_search / memory_get 工具调用                       │   │
│  └───────────────────────┬─────────────────────────────────────┘   │
│                          │ knowledge_graph_search 工具触发           │
│                          ▼                                          │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  L2: Knowledge Graph (knowledge_graph 表)                    │   │
│  │  - 实体-关系图谱（由 KG Extractor 从对话中提取）              │   │
│  │  - 实体去重 + 关系推理                                       │   │
│  │  - knowledge_graph_search 工具调用                           │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

### 3.2 AutoInject RAG 流程（L0 自动注入）

```
用户发送消息 "what's my favorite color?"
    │
    ▼
ContextStage.AutoInject()
    │
    ├─ 1. 构建召回查询（buildRecallQuery）
    │     recentContext = "User: I like blue tones | User: what's my favorite color?"
    │     searchQuery = "Context: User: I like blue tones\nQuery: what's my favorite color?"
    │     → 帮助向量搜索理解代词 "my" 指代什么
    │
    ├─ 2. 搜索 episodic_summaries（Hybrid: FTS 70% + Vector 30%）
    │     │
    │     ├─ FTS 搜索（postgresql full-text search）
    │     │   SELECT id, l0_abstract,
    │     │     ts_rank(search_vector, plainto_tsquery('english', $1)) AS score
    │     │   FROM episodic_summaries
    │     │   WHERE agent_id = $2 AND search_vector @@ plainto_tsquery('english', $1)
    │     │     AND user_id = $3 AND tenant_id = $4
    │     │   ORDER BY score DESC LIMIT 10
    │     │
    │     ├─ Vector 搜索（pgvector cosine similarity）
    │     │   SELECT id, l0_abstract,
    │     │     1 - (embedding <=> $1) AS score
    │     │   FROM episodic_summaries
    │     │   WHERE agent_id = $2 AND embedding IS NOT NULL
    │     │     AND user_id = $3 AND tenant_id = $4
    │     │   ORDER BY embedding <=> $1 LIMIT 10
    │     │
    │     └─ 合并（FTS × 0.7 + Vector × 0.3）
    │         mergeEpisodicScores(fts, vec, 0.7, 0.3)
    │         → 按 combined score 降序排列
    │
    ├─ 3. 过滤 + 格式化
    │     threshold = 0.3（最低分数）
    │     maxEntries = 5（最多注入 5 条）
    │
    └─ 4. 追加到 system prompt
          sys.Content += "\n\n## Memory Context\n\n"
          sys.Content += "- User prefers blue tones and dark themes\n"
          sys.Content += "- User is a Python developer\n"
          sys.Content += "..."
```

### 3.3 Memory Search 工具流程（L1 语义检索）

```
Agent 调用 memory_search("favorite color")
    │
    ▼
PGMemoryStore.Search()
    │
    ├─ 1. FTS 搜索（postgresql tsvector）
    │     SELECT path, start_line, end_line, text, user_id,
    │       ts_rank(tsv, plainto_tsquery('simple', $1)) AS score
    │     FROM memory_chunks
    │     WHERE agent_id = $2 AND tsv @@ plainto_tsquery('simple', $3)
    │       AND (user_id IS NULL OR user_id = $4)
    │     ORDER BY score DESC LIMIT 12
    │
    ├─ 2. Vector 搜索（pgvector）
    │     embeddings := provider.Embed(["favorite color"])
    │     → 调用 OpenAI text-embedding-3-small API
    │     → 返回 [0.012, -0.034, ...] 1536 维向量
    │
    │     SELECT path, start_line, end_line, text, user_id,
    │       1 - (embedding <=> $1::vector) AS score
    │     FROM memory_chunks
    │     WHERE agent_id = $2 AND embedding IS NOT NULL
    │       AND (user_id IS NULL OR user_id = $3)
    │     ORDER BY embedding <=> $1::vector LIMIT 12
    │
    ├─ 3. 混合合并（hybridMerge）
    │     - 个人文档 1.2x boost
    │     - 文本权重 0.3 + 向量权重 0.7（默认）
    │     - 去重：同一文档路径+行号取最高分
    │
    └─ 4. 返回 MemorySearchResult[]
          → Agent 看到匹配的文档片段
```

---

## 4. PG 向量化完整流程

### 4.1 数据库 Schema

```sql
-- 记忆文档表
CREATE TABLE memory_documents (
    id UUID PRIMARY KEY,
    agent_id UUID NOT NULL,
    user_id TEXT,           -- NULL = 全局文档
    path TEXT NOT NULL,
    content TEXT NOT NULL,
    hash TEXT NOT NULL,      -- SHA-256 内容哈希
    tenant_id UUID NOT NULL,
    updated_at TIMESTAMPTZ,
    UNIQUE (agent_id, COALESCE(user_id, ''), path)
);

-- 记忆分块表（带向量）
CREATE TABLE memory_chunks (
    id UUID PRIMARY KEY,
    agent_id UUID NOT NULL,
    document_id UUID NOT NULL REFERENCES memory_documents(id),
    user_id TEXT,
    path TEXT NOT NULL,
    start_line INT,
    end_line INT,
    hash TEXT NOT NULL,       -- 分块内容哈希
    text TEXT NOT NULL,
    embedding vector(1536),  -- pgvector 列（OpenAI text-embedding-3-small）
    tsv tsvector,            -- 全文搜索向量
    tenant_id UUID NOT NULL,
    updated_at TIMESTAMPTZ
);

-- 向量缓存表（避免重复 API 调用）
CREATE TABLE embedding_cache (
    hash TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    embedding vector(1536) NOT NULL,
    dims INT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    tenant_id UUID
);

-- Episodic 记忆表（带向量）
CREATE TABLE episodic_summaries (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    user_id TEXT,
    session_key TEXT,
    summary TEXT NOT NULL,
    key_topics TEXT[],
    l0_abstract TEXT,          -- ~50 token 预计算摘要
    embedding vector(1536),   -- pgvector 列
    search_vector tsvector,   -- FTS 列
    source_id TEXT,
    source_type TEXT,
    turn_count INT,
    token_count INT,
    recall_count INT DEFAULT 0,
    recall_score FLOAT DEFAULT 0,
    last_recalled_at TIMESTAMPTZ,
    promoted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    UNIQUE (agent_id, user_id, source_id) WHERE source_id IS NOT NULL
);
```

### 4.2 向量化写入流程（IndexDocument）

```
用户调用 memory_put(path="notes/design.md", content="...")
    │
    ▼
PGMemoryStore.PutDocument()
    │
    ├─ 1. 存储文档
    │   INSERT INTO memory_documents (id, agent_id, user_id, path, content, hash, ...)
    │   VALUES ($1, $2, $3, $4, $5, $6, ...)
    │   ON CONFLICT (agent_id, COALESCE(user_id,''), path)
    │   DO UPDATE SET content = EXCLUDED.content, hash = EXCLUDED.hash
    │
    ├─ 2. 触发 IndexDocument
    │   │
    │   ├─ 2a. 获取文档内容
    │   │   SELECT content FROM memory_documents
    │   │   WHERE agent_id = $1 AND path = $2 AND (user_id = $3 OR user_id IS NULL)
    │   │
    │   ├─ 2b. 删除旧分块
    │   │   DELETE FROM memory_chunks WHERE document_id = $1
    │   │
    │   ├─ 2c. 文本分块（ChunkText）
    │   │   chunks = memory.ChunkText(content, maxChunkLen=1000, overlap=200)
    │   │   → 按段落边界分割，每块 ≤1000 字符
    │   │   → 块间重叠 200 字符保持上下文连续性
    │   │
    │   ├─ 2d. 生成内容哈希
    │   │   hashes[i] = ContentHash(chunk.Text)  // SHA-256 前 16 字节
    │   │
    │   ├─ 2e. 查询向量缓存
    │   │   SELECT hash, embedding FROM embedding_cache
    │   │   WHERE hash IN ($1, $2, ...) AND provider = 'openai' AND model = 'text-embedding-3-small'
    │   │
    │   ├─ 2f. 调用 Embedding API（仅未缓存的）
    │   │   │
    │   │   │  POST https://api.openai.com/v1/embeddings
    │   │   │  Authorization: Bearer sk-xxx
    │   │   │  Content-Type: application/json
    │   │   │  {
    │   │   │    "input": ["chunk_text_1", "chunk_text_2", ...],
    │   │   │    "model": "text-embedding-3-small"
    │   │   │  }
    │   │   │
    │   │   │  Response: {
    │   │   │    "data": [{"embedding": [0.012, -0.034, ...]}]
    │   │   │  }
    │   │   │
    │   │   └→ embeddings[][]float32 (1536 维)
    │   │
    │   ├─ 2g. 写入向量缓存
    │   │   INSERT INTO embedding_cache (hash, provider, model, embedding, dims, ...)
    │   │   VALUES ($1, $2, $3, $4::vector, $5, ...)
    │   │   ON CONFLICT (hash, provider, model) DO UPDATE ...
    │   │
    │   └─ 2h. 插入分块（带向量）
    │       INSERT INTO memory_chunks (id, agent_id, document_id, user_id,
    │         path, start_line, end_line, hash, text, embedding, tenant_id, ...)
    │       VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::vector, $11, ...)
    │       ON CONFLICT DO NOTHING
    │
    └─ 完成
```

### 4.3 向量化写入流程（Episodic Summaries）

```
Session 完成 → Consolidation Pipeline
    │
    ▼
episodicWorker.Handle()
    │
    ├─ 1. 调用 LLM 生成 summary + l0_abstract
    │   LLM Prompt: "Summarize this conversation session concisely..."
    │   → summary: "User discussed Python project architecture..."
    │   → l0_abstract: "Python project planning" (~50 tokens)
    │
    ├─ 2. 生成向量嵌入
    │   vectors := embProvider.Embed([summary])
    │   → POST /v1/embeddings { input: [summary], model: "text-embedding-3-small" }
    │   → embedding = [0.012, -0.034, ...] (1536 维)
    │
    ├─ 3. 存储到 PostgreSQL
    │   INSERT INTO episodic_summaries
    │     (id, tenant_id, agent_id, user_id, session_key,
    │      summary, key_topics, turn_count, token_count,
    │      embedding, l0_abstract, source_id, source_type, ...)
    │   VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::vector, $11, $12, $13, ...)
    │   ON CONFLICT (agent_id, user_id, source_id) DO NOTHING
    │
    ├─ 4. FTS 索引自动更新（PostgreSQL trigger）
    │   search_vector = to_tsvector('english', summary)
    │
    └─ 5. Publish(episodic.created) → 触发 semanticWorker + dreamingWorker
```

### 4.4 混合搜索算法（Hybrid Search）

```
搜索查询 "favorite color"
    │
    ├─ FTS 搜索（PostgreSQL tsvector）
    │   权重: TextWeight = 0.7
    │   算法: ts_rank(search_vector, plainto_tsquery('english', query))
    │   优点: 关键词精确匹配，速度快
    │   缺点: 无法理解语义
    │
    ├─ Vector 搜索（pgvector）
    │   权重: VectorWeight = 0.3
    │   算法: 1 - (embedding <=> query_embedding)  [cosine distance]
    │   优点: 语义理解，支持同义词
    │   缺点: 需要 embedding API 调用
    │
    └─ 合并算法（mergeEpisodicScores / hybridMerge）
        │
        ├─ 1. FTS 结果: score *= TextWeight
        ├─ 2. Vector 结果: score *= VectorWeight
        ├─ 3. 合并相同 ID 的分数（加权求和）
        ├─ 4. 个人文档 1.2x boost
        └─ 5. 按 combined score 降序排列
```

---

## 5. Messages 构建与 LLM 请求结构

### 5.1 MessageBuffer 三层结构

```
MessageBuffer {
    system: providers.Message {
        role: "system",
        content: "You are a personal assistant running in Telegram...[16 sections]..."
                 + "\n\n## Memory Context\n\n- User prefers blue tones..."
    }

    history: []providers.Message {   // 从 PostgreSQL 加载
        {role: "user", content: "hello"},
        {role: "assistant", content: "Hi! How can I help?"},
        {role: "user", content: "what's 2+2?"},
        {role: "assistant", tool_calls: [{id:"call_1", name:"exec", args:{command:"echo $((2+2))"}}]},
        {role: "tool", tool_call_id: "call_1", content: "4"},
        {role: "assistant", content: "2+2 = 4"},
        ... 更多历史消息 ...
    }

    pending: []providers.Message {   // 本轮新增
        {role: "user", content: "what's my favorite color?"},
    }
}

// 发送给 LLM 的完整请求
messages = Buffer.All()
// = [system] + [history] + [pending]
```

### 5.2 发送给 LLM 的完整请求结构

```json
{
  "model": "claude-sonnet-4-20250514",
  "messages": [
    {
      "role": "system",
      "content": "You are a personal assistant running in Telegram (a direct chat)...\n\n## Tooling\n\n- exec: Execute a shell command...\n- read_file: Read file contents...\n...\n\n## Safety\n\nNo independent goals...\n\n## Skills (mandatory)\n\n<available_skills>...</available_skills>\n\n## Workspace\n\nYour working directory is: /app/workspace/agent-default/user-alice\n\n## Memory Context\n\n- User prefers blue tones and dark themes\n- User is a Python developer\n\n---CACHE BOUNDARY---\n\nCurrent time: 2026-06-12 18:30:00 UTC\n\n<extra_context>\nHeartbeat checklist: ...\n</extra_context>\n\n## Project Context\n\n### SOUL.md\n[persona content]\n\n### AGENTS.md\n[instructions]\n\n### USER.md\n[user profile]\n\n## Runtime\nAgent: agent-default\nModel: claude-sonnet-4-20250514\nChannel: Telegram\n..."
    },
    {"role": "user", "content": "hello"},
    {"role": "assistant", "content": "Hi! How can I help?"},
    {"role": "user", "content": "what's 2+2?"},
    {"role": "assistant", "tool_calls": [{"id": "call_1", "name": "exec", "arguments": {"command": "echo $((2+2))"}}]},
    {"role": "tool", "tool_call_id": "call_1", "content": "4"},
    {"role": "assistant", "content": "2+2 = 4"},
    {"role": "user", "content": "what's my favorite color?"}
  ],
  "tools": [
    {"type": "function", "function": {"name": "exec", "description": "Execute a shell command", "parameters": {...}}},
    {"type": "function", "function": {"name": "read_file", "description": "Read file contents", "parameters": {...}}},
    {"type": "function", "function": {"name": "memory_search", "description": "Search memory documents", "parameters": {...}}},
    ...
  ]
}
```

---

## 6. 上下文存储完整流程

### 6.1 Session 存储

```
PostgreSQL sessions 表:
┌──────────────┬────────────────────────────────────────┬─────────┐
│ session_key  │ messages (JSONB)                       │ summary │
├──────────────┼────────────────────────────────────────┼─────────┤
│ agent:default│ [{role:"system", content:"..."},       │ "User   │
│ :telegram    │  {role:"user", content:"hello"},       │ discuss │
│ :direct:     │  {role:"assistant", content:"Hi!..."}, │  ed     │
│ 1718200000   │  {role:"tool", content:"4"},           │ Python  │
│              │  {role:"assistant", content:"2+2=4"},  │ ..."]   │
│              │  {role:"user", content:"favorite?"}]   │         │
└──────────────┴────────────────────────────────────────┴─────────┘

// 持久化时机:
// 1. CheckpointStage: 每 5 次迭代 flush pending → history
// 2. FinalizeStage: 所有 pending → history → PostgreSQL
// 3. Compaction: 前 70% 消息摘要化 → 更新 summary 字段
```

### 6.2 Compaction（上下文压缩）

```
当消息数超过 context window 限制时:
    │
    ├─ 1. 分割点: splitIdx = len(messages) - keepCount (最后 30% 保留)
    │     → 找到 tool_use/tool_result 对的边界
    │
    ├─ 2. LLM 摘要化: callLLM(compactionSummaryPrompt + messages[:splitIdx])
    │     → "Summarize this conversation concisely for the AI agent to resume work.
    │        MUST PRESERVE: Active tasks, Pending subagent tasks, Decisions..."
    │
    ├─ 3. 替换: messages = [{summary_msg}, messages[splitIdx:]...]
    │
    └─ 4. 持久化: session.summary = summary
```

### 6.3 Embedding Cache 策略

```
写入时:
    1. 计算 content_hash (SHA-256 前 16 字节)
    2. 查询 embedding_cache 表
    3. 命中 → 直接使用缓存向量
    4. 未命中 → 调用 OpenAI Embedding API → 写入缓存

优势:
    - 相同内容不重复调用 API（节省费用）
    - 增量更新：修改文档只重新 embedding 变化的分块
    - BackfillEmbeddings(): 批量回填缺失的向量
```

---

## 7. 完整调用栈

```
Pipeline.Run()
  │
  ├─ ContextStage.Execute()
  │   ├─ injectContext()           → ctx 注入 agent/tenant/user/workspace
  │   ├─ ResolveWorkspace()        → 解析分层工作区路径
  │   ├─ LoadContextFiles()        → 加载 bootstrap 文件 (AGENTS.md, SOUL.md...)
  │   ├─ LoadSessionHistory()      → PostgreSQL sessions → []Message
  │   ├─ BuildMessages()           → BuildSystemPrompt() + 合并 history
  │   │   └─ BuildSystemPrompt()   → 16 个 Section 组装
  │   ├─ TokenCounter.CountMessages → 计算 overhead tokens
  │   └─ AutoInject()              → RAG L0 检索 → 追加到 system prompt
  │       └─ episodicStore.Search()
  │           ├─ ftsSearch()       → PostgreSQL tsvector + ts_rank
  │           ├─ embProvider.Embed → OpenAI Embedding API
  │           ├─ vectorSearch()    → pgvector cosine similarity
  │           └─ mergeScores()     → 加权合并 FTS + Vector
  │
  ├─ ThinkStage.Execute()          → callLLM(messages) → LLM 响应
  │   └─ callLLM()
  │       ├─ provider.Chat(req)    → OpenAI/Anthropic API
  │       └─ response.ToolCalls    → 工具调用列表
  │
  ├─ PruneStage.Execute()          → 上下文裁剪（如果超预算）
  │   └─ pruneMessages()
  │       ├─ soft-trim: 裁剪工具结果
  │       ├─ hard-clear: 清空旧工具结果
  │       └─ compact: LLM 摘要化旧消息
  │
  ├─ ToolStage.Execute()           → 工具执行
  │   └─ executeToolCall()
  │       ├─ registry.Execute("memory_search", args)
  │       │   └─ PGMemoryStore.Search()
  │       │       ├─ ftsSearch()   → PostgreSQL tsvector
  │       │       ├─ provider.Embed → OpenAI Embedding API
  │       │       ├─ vectorSearch() → pgvector cosine similarity
  │       │       └─ hybridMerge()  → 加权合并
  │       └─ Result{ForLLM: "..."} → 追加到 messages
  │
  ├─ ObserveStage.Execute()        → 检查是否完成
  │
  ├─ CheckpointStage.Execute()     → FlushPending → history
  │
  └─ FinalizeStage.Execute()       → 持久化 + 后处理
      ├─ FlushMessages()           → PostgreSQL sessions.messages
      ├─ MaybeSummarize()          → 异步 compaction
      └─ EmitSessionCompleted()    → → consolidation pipeline
          └─ episodicWorker.Handle()
              ├─ LLM Summarize     → summary + l0_abstract
              ├─ embProvider.Embed  → 生成向量
              └─ INSERT episodic_summaries → PostgreSQL
```

---

## 8. 关键 SQL 语句汇总

### 8.1 向量搜索

```sql
-- Episodic Vector Search
SELECT id, l0_abstract, 1 - (embedding <=> $1) AS score
FROM episodic_summaries
WHERE agent_id = $2 AND embedding IS NOT NULL
  AND user_id = $3 AND tenant_id = $4
ORDER BY embedding <=> $1 LIMIT 10;

-- Memory Vector Search  
SELECT path, start_line, end_line, text, user_id,
  1 - (embedding <=> $1::vector) AS score
FROM memory_chunks
WHERE agent_id = $2 AND embedding IS NOT NULL
  AND (user_id IS NULL OR user_id = $3)
ORDER BY embedding <=> $1::vector LIMIT 12;
```

### 8.2 全文搜索

```sql
-- Episodic FTS Search
SELECT id, l0_abstract,
  ts_rank(search_vector, plainto_tsquery('english', $1)) AS score
FROM episodic_summaries
WHERE agent_id = $2
  AND search_vector @@ plainto_tsquery('english', $1)
  AND user_id = $3 AND tenant_id = $4
ORDER BY score DESC LIMIT 10;

-- Memory FTS Search
SELECT path, start_line, end_line, text, user_id,
  ts_rank(tsv, plainto_tsquery('simple', $1)) AS score
FROM memory_chunks
WHERE agent_id = $2 AND tsv @@ plainto_tsquery('simple', $3)
  AND (user_id IS NULL OR user_id = $4)
ORDER BY score DESC LIMIT 12;
```

### 8.3 向量写入

```sql
-- Memory Chunk 写入（带向量）
INSERT INTO memory_chunks (id, agent_id, document_id, user_id,
  path, start_line, end_line, hash, text, embedding, tenant_id, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::vector, $11, $12);

-- Episodic Summary 写入（带向量）
INSERT INTO episodic_summaries
  (id, tenant_id, agent_id, user_id, session_key, summary, key_topics,
   turn_count, token_count, embedding, l0_abstract, source_id, source_type, ...)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::vector, $11, $12, $13, ...)
ON CONFLICT (agent_id, user_id, source_id) DO NOTHING;

-- Embedding Cache 写入
INSERT INTO embedding_cache (hash, provider, model, embedding, dims, created_at, ...)
VALUES ($1, $2, $3, $4::vector, $5, $6, ...)
ON CONFLICT (hash, provider, model) DO UPDATE SET embedding = EXCLUDED.embedding;
```

### 8.4 Recall 信号更新

```sql
-- 记录记忆被检索的次数和平均分
UPDATE episodic_summaries
SET recall_count = recall_count + 1,
    recall_score = (recall_score * recall_count + $1) / (recall_count + 1),
    last_recalled_at = NOW()
WHERE id = $2 AND tenant_id = $3;
```
