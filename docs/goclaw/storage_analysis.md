# GoClaw 存储层完整分析

> 分析基于 `internal/store/sqlitestore/schema.sql`（73 张表）+ PostgreSQL 扩展列 + 外部存储系统  
> 存储双后端：PostgreSQL（主）/ SQLite（嵌入式），Redis 为可选缓存层

---

## 1. 存储架构总览

```
┌──────────────────────────────────────────────────────────────────┐
│                        GoClaw 存储架构                           │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐    必选主存储（二选一）                         │
│  │ PostgreSQL   │─── pgvector 扩展 → 向量搜索                    │
│  │  (73 tables) │─── FTS (tsvector) → 全文搜索                   │
│  └──────────────┘                                                │
│         或                                                       │
│  ┌──────────────┐    嵌入式/开发环境                              │
│  │  SQLite      │─── FTS5 虚拟表 → 全文搜索                      │
│  │  (73 tables) │─── 无原生向量（Go 侧 cosine 计算）             │
│  └──────────────┘                                                │
│                                                                  │
│  ┌──────────────┐    可选缓存（-tags redis 编译）                │
│  │  Redis       │─── agent_ctx_cache / user_ctx_cache            │
│  │  (可选)      │    key 前缀: ctx:agent:, ctx:user:             │
│  └──────────────┘                                                │
│                                                                  │
│  ┌──────────────┐    媒体文件存储                                │
│  │  本地文件系统  │─── .media/{sessionHash}/{uuid}.{ext}         │
│  │  (.media/)   │    按 session 分目录隔离                       │
│  └──────────────┘                                                │
│                                                                  │
│  ┌──────────────┐    技能文件存储                                │
│  │  本地文件系统  │─── skills-store/{slug}/SKILL.md              │
│  │  (skills/)   │    + assets/ + references/ + templates/       │
│  └──────────────┘                                                │
│                                                                  │
│  ┌──────────────┐    沙箱状态                                    │
│  │  Docker       │─── 按 user+session+agent 隔离容器             │
│  │  容器持久化    │─── workspace volume mount                    │
│  └──────────────┘                                                │
│                                                                  │
│  ┌──────────────┐    可选备份                                    │
│  │  S3 兼容存储  │─── backup --upload-s3                         │
│  └──────────────┘                                                │
│                                                                  │
│  ┌──────────────┐    可选通道                                    │
│  │  Kafka        │─── extensions/kafka-channel/                  │
│  │  (可选扩展)   │    消息入站/出站通道                           │
│  └──────────────┘                                                │
└──────────────────────────────────────────────────────────────────┘
```

### 环境变量配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `GOCLAW_STORAGE_BACKEND` | `postgres` 或 `sqlite` | `postgres` |
| `GOCLAW_POSTGRES_DSN` | PostgreSQL 连接串 | - |
| `GOCLAW_SQLITE_PATH` | SQLite 文件路径 | `{dataDir}/goclaw.db` |
| `GOCLAW_REDIS_DSN` | Redis 连接串（可选） | 空（回退到内存缓存） |

---

## 2. PostgreSQL 独有扩展列

PostgreSQL 版本比 SQLite 多以下列（schema.sql 中注释为"omitted"）：

| 表 | 扩展列 | 类型 | 用途 |
|----|--------|------|------|
| `agents` | `tsv` | `tsvector` | agent 前端配置全文搜索 |
| `agents` | `embedding` | `vector(1536)` | agent frontmatter 语义搜索 |
| `memory_chunks` | `tsv` | `tsvector` | 记忆块全文搜索 |
| `memory_chunks` | `embedding` | `vector(1536)` | 记忆块语义搜索 |
| `embedding_cache` | `embedding` | `vector(1536)` | 嵌入向量缓存 |
| `skills` | `tsv` | `tsvector` | 技能全文搜索 |
| `skills` | `embedding` | `vector(1536)` | 技能语义搜索 |
| `episodic_summaries` | `search_vector` | `tsvector` | 情景记忆全文搜索 |
| `episodic_summaries` | `embedding` | `vector(1536)` | 情景记忆语义搜索 |
| `kg_entities` | `tsv` | `tsvector` | 知识图谱实体全文搜索 |
| `kg_entities` | `embedding` | `vector(1536)` | 知识图谱实体语义搜索 |
| `team_tasks` | `tsv` | `tsvector` | 团队任务全文搜索 |
| `team_tasks` | `embedding` | `vector(1536)` | 团队任务语义搜索 |

---

## 3. 73 张表完整清单与功能映射

### 3.1 多租户核心（2 表）

| 表名 | 功能 | 主键 | 核心字段 |
|------|------|------|----------|
| **tenants** | 租户注册表 | `id` (UUID) | name, slug, status, settings(JSON) |
| **tenant_users** | 租户-用户关联 | `id` (UUID) | tenant_id(FK), user_id, display_name, role(admin/member) |

**说明**: 所有其他表都有 `tenant_id` 外键，实现数据隔离。

---

### 3.2 LLM Provider（1 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **llm_providers** | LLM 提供商配置 | name, provider_type(openai_compat/openai/anthropic/...), api_base, api_key(AES加密), settings(JSON), tenant_id |

**唯一约束**: `(tenant_id, name)` — 同一租户下名称唯一

---

### 3.3 Agent 系统（8 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **agents** | Agent 定义 | agent_key, display_name, owner_id, provider, model, context_window, tools_config(JSON), sandbox_config(JSON), subagents_config(JSON), memory_config(JSON), compaction_config(JSON), workspace_sharing(JSON), model_fallback(JSON), budget_monthly_cents, tenant_id |
| **agent_shares** | Agent 共享权限 | agent_id(FK), user_id, role(user/admin), granted_by, tenant_id |
| **agent_context_files** | Agent 级上下文文件 | agent_id(FK), file_name, content(TEXT), tenant_id |
| **user_context_files** | 用户级上下文文件 | agent_id(FK), user_id, file_name, content(TEXT), tenant_id |
| **user_agent_profiles** | 用户-Agent 交互画像 | agent_id(FK), user_id, workspace, metadata(JSON), first_seen_at, last_seen_at |
| **user_agent_overrides** | 用户级模型覆盖 | agent_id(FK), user_id, provider, model, settings(JSON) |
| **agent_config_permissions** | 配置变更权限 | agent_id(FK), scope, config_type, user_id, permission(rw/ro) |
| **agent_links** | Agent 间委派链接 | source_agent_id(FK), target_agent_id(FK), direction, description, max_concurrent, settings(JSON), status |

**唯一约束**:
- `agents`: `(tenant_id, agent_key) WHERE deleted_at IS NULL`
- `agent_shares`: `(agent_id, user_id)`
- `agent_context_files`: `(agent_id, file_name)`
- `user_context_files`: `(agent_id, user_id, file_name)`
- `user_agent_overrides`: `(agent_id, user_id)`
- `agent_links`: `(source_agent_id, target_agent_id)`

---

### 3.4 团队系统（9 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **agent_teams** | 团队定义 | name, lead_agent_id(FK), description, settings(JSON), created_by, tenant_id |
| **agent_team_members** | 团队成员 | team_id(FK), agent_id(FK), role(member/lead), tenant_id |
| **team_tasks** | 团队任务 | team_id(FK), subject, description, status(pending/in_progress/done/...), owner_agent_id(FK), priority, result, task_type, identifier, progress_percent, followup_at, followup_count, confidence_score, tenant_id |
| **team_task_comments** | 任务评论 | task_id(FK), agent_id(FK), user_id, content, comment_type(note/comment/system), confidence_score, tenant_id |
| **team_task_events** | 任务事件日志 | task_id(FK), event_type, actor_type(agent/user), actor_id, data(JSON), tenant_id |
| **team_task_attachments** | 任务附件 | task_id(FK), team_id(FK), chat_id, path, base_name, file_size, mime_type, tenant_id |
| **team_user_grants** | 团队用户授权 | team_id(FK), user_id, role(viewer/editor/admin), granted_by, tenant_id |
| **team_task_siblings** | 任务子任务关系 | （从 schema 中可见但未详细展开） |
| **subagent_tasks** | 子 Agent 任务 | tenant_id(FK), parent_agent_key, session_key, subject, description, status, result, depth, model, provider, iterations, input_tokens, output_tokens, origin_channel, origin_chat_id |

**唯一约束**:
- `team_tasks`: `(team_id, identifier) WHERE identifier IS NOT NULL`
- `team_task_attachments`: `(task_id, path)`
- `team_user_grants`: `(team_id, user_id)`
- `agent_team_members`: `(team_id, agent_id)`

---

### 3.5 会话系统（1 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **sessions** | 对话会话 | session_key, agent_id(FK), user_id, messages(JSONB/TEXT), summary, model, provider, channel, input_tokens, output_tokens, compaction_count, memory_flush_compaction_count, label, spawned_by, spawn_depth, metadata(JSON), tenant_id, team_id(FK) |

**唯一约束**: `(tenant_id, session_key)`

**关键字段说明**:
- `messages`: JSON 数组，存储完整的对话消息历史（角色/内容/工具调用等）
- `compaction_count`: 消息压缩次数（保留最近 30%，前 70% LLM 摘要化）
- `spawned_by`: 父 session key（子 Agent 任务继承）
- `spawn_depth`: 嵌套深度（防止无限递归）

---

### 3.6 三级记忆系统（5 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **memory_documents** | L1 语义记忆 - 文档注册 | agent_id(FK), user_id, path, content, hash(SHA-256), team_id(FK), custom_scope, tenant_id |
| **memory_chunks** | L1 语义记忆 - 文档分块 | agent_id(FK), document_id(FK), user_id, path, start_line, end_line, hash, text, team_id(FK), custom_scope, tenant_id |
| **embedding_cache** | 嵌入向量缓存 | hash, provider, model, dims, embedding(vector/TEXT), tenant_id |
| **episodic_summaries** | L0 情景记忆 - 会话摘要 | agent_id(FK), user_id, session_key, summary, l0_abstract, key_topics(JSON), source_type, turn_count, token_count, recall_count, recall_score, last_recalled_at, tenant_id |
| **kg_entities** | L2 知识图谱 - 实体 | agent_id(FK), user_id, external_id, name, entity_type, description, properties(JSON), confidence, team_id(FK), valid_from, valid_until, tenant_id |
| **kg_relations** | L2 知识图谱 - 关系 | agent_id(FK), user_id, source_entity_id(FK→kg_entities), relation_type, target_entity_id(FK→kg_entities), confidence, properties(JSON), team_id(FK), valid_from, valid_until, tenant_id |
| **kg_dedup_candidates** | 知识图谱去重队列 | agent_id(FK), user_id, entity_a_id(FK), entity_b_id(FK), similarity, status(pending/merged/ignored), tenant_id |

**唯一约束**:
- `memory_documents`: `(agent_id, COALESCE(user_id,''), path)`
- `kg_entities`: `(agent_id, user_id, external_id)`
- `kg_relations`: `(agent_id, user_id, source_entity_id, relation_type, target_entity_id)`
- `kg_dedup_candidates`: `(entity_a_id, entity_b_id)`
- `episodic_summaries`: `(agent_id, user_id, source_id) WHERE source_id IS NOT NULL`

**向量搜索架构（PostgreSQL only）**:

```
                    ┌──────────────────┐
                    │     用户查询      │
                    └────────┬─────────┘
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
        ┌──────────┐  ┌──────────┐  ┌──────────┐
        │ L0 情景   │  │ L1 语义   │  │ L2 知识图谱│
        │episodic   │  │memory     │  │kg_entities│
        │summaries  │  │_chunks    │  │+relations │
        └─────┬────┘  └─────┬────┘  └─────┬────┘
              │              │              │
    ┌─────────┴─────────┐   │    ┌─────────┴─────────┐
    │ FTS(70%)+Vec(30%) │   │    │ FTS(30%)+Vec(70%) │
    │ ts_rank + cosine  │   │    │ ts_rank + cosine  │
    └─────────┬─────────┘   │    └─────────┬─────────┘
              │    FTS(50%)+Vec(50%)       │
              │    ts_rank + cosine        │
              └──────────┬─────────────────┘
                         ▼
                 hybridMerge() → 最终排序结果
```

---

### 3.7 Skills 系统（4 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **skills** | 技能定义 | name, slug, description, owner_id, visibility(private/internal/public), version, frontmatter(JSON), file_path, file_size, file_hash, tags, is_system, deps(JSON), enabled, tenant_id |
| **skill_agent_grants** | 技能→Agent 授权 | skill_id(FK), agent_id(FK), pinned_version, granted_by, can_manage, tenant_id |
| **skill_user_grants** | 技能→用户授权 | skill_id(FK), user_id, granted_by, tenant_id |
| **skill_tenant_configs** | 技能→租户配置 | skill_id(FK), tenant_id(FK), enabled, tenant_id |

**唯一约束**:
- `skills`: `(tenant_id, slug)`
- `skill_agent_grants`: `(skill_id, agent_id)`
- `skill_user_grants`: `(skill_id, user_id)`
- `skill_tenant_configs`: `(skill_id, tenant_id)`

---

### 3.8 定时任务系统（2 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **cron_jobs** | Cron 定义 | agent_id(FK), user_id, name, enabled, schedule_kind(cron/interval/once), cron_expression, interval_ms, run_at, timezone, payload(JSON), delete_after_run, stateless, deliver, deliver_channel, deliver_to, wake_heartbeat, next_run_at, last_run_at, last_status, team_id(FK), tenant_id |
| **cron_run_logs** | 执行日志 | job_id(FK), agent_id(FK), status, summary, error, duration_ms, input_tokens, output_tokens, team_id(FK), ran_at, tenant_id |

**唯一约束**:
- `cron_jobs`: `(agent_id, tenant_id, name)`

---

### 3.9 设备配对系统（2 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **pairing_requests** | 配对请求 | code(8位), sender_id, channel, chat_id, account_id, metadata(JSON), expires_at, tenant_id |
| **paired_devices** | 已配对设备 | sender_id, channel, chat_id, paired_by(operator/user), metadata(JSON), expires_at, tenant_id |

**唯一约束**:
- `pairing_requests`: `code`
- `paired_devices`: `(tenant_id, sender_id, channel)`

---

### 3.10 可观测性/追踪系统（2 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **traces** | 追踪主表 | agent_id, user_id, session_key, run_id, name, channel, input_preview, output_preview, total_input_tokens, total_output_tokens, total_cost, span_count, llm_call_count, tool_call_count, status, error, metadata(JSON), tags(JSON), parent_trace_id, team_id(FK), tenant_id |
| **spans** | Span 详情 | trace_id(FK), parent_span_id, agent_id, span_type(llm/tool/hook/sandbox/...), name, model, provider, input_tokens, output_tokens, total_cost, finish_reason, tool_name, tool_call_id, input_preview, output_preview, metadata(JSON), team_id(FK), tenant_id |

**父子追踪**: `traces` → `spans` (1:N)，`spans` → `spans` (parent_span_id 自引用)

---

### 3.11 MCP（Model Context Protocol）系统（5 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **mcp_servers** | MCP 服务器定义 | name, display_name, transport(stdio/sse/streamable-http), command, args(JSON), url, headers(JSON), env(JSON), api_key, tool_prefix, timeout_sec, settings(JSON), enabled, created_by, tenant_id |
| **mcp_agent_grants** | MCP→Agent 授权 | server_id(FK), agent_id(FK), enabled, tool_allow(JSON), tool_deny(JSON), config_overrides(JSON), granted_by, tenant_id |
| **mcp_user_grants** | MCP→用户授权 | server_id(FK), user_id, enabled, tool_allow(JSON), tool_deny(JSON), granted_by, tenant_id |
| **mcp_access_requests** | MCP 访问申请 | server_id(FK), agent_id(FK), user_id, scope(agent/user), status(pending/approved/denied), reason, tool_allow(JSON), requested_by, reviewed_by, review_note, tenant_id |
| **mcp_user_credentials** | MCP 用户凭证 | server_id(FK), user_id, api_key, headers(BLOB/AES加密), env(BLOB/AES加密), tenant_id |

**唯一约束**:
- `mcp_servers`: `(tenant_id, name)`
- `mcp_agent_grants`: `(server_id, agent_id)`
- `mcp_user_grants`: `(server_id, user_id)`
- `mcp_user_credentials`: `(server_id, user_id, tenant_id)`

---

### 3.12 Channel 通道系统（3 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **channel_instances** | 通道实例 | name, display_name, channel_type(telegram/discord/slack/whatsapp/feishu/...), agent_id(FK), credentials(BLOB/AES加密), config(JSON), enabled, created_by, tenant_id |
| **channel_pending_messages** | 群聊缓冲消息 | channel_name, history_key, sender, sender_id, body, platform_msg_id, is_summary, tenant_id |
| **channel_contacts** | 通道联系人 | channel_type, channel_instance, sender_id, user_id, display_name, username, avatar_url, peer_kind, contact_type(user/bot), thread_id, thread_type, metadata(JSON), merged_id, tenant_id |

**唯一约束**:
- `channel_instances`: `(tenant_id, name)`
- `channel_contacts`: `(tenant_id, channel_type, sender_id, COALESCE(thread_id,''))`

---

### 3.13 Vault 知识库系统（2 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **vault_documents** | 文档注册表 | agent_id(FK), team_id(FK), chat_id, scope(personal/team/shared/custom), custom_scope, path, path_basename, title, doc_type(note/context/memory/skill/episodic/media/document), content_hash(SHA-256), summary, metadata(JSON), tenant_id |
| **vault_links** | 文档间链接 | from_doc_id(FK→vault_documents), to_doc_id(FK→vault_documents), link_type(wikilink/reference/task_attachment/...), context, metadata(JSON), custom_scope, tenant_id |

**约束**: `vault_documents` 有 CHECK 约束确保 scope 一致性（personal 需要 agent_id, team 需要 team_id）

---

### 3.14 内建工具（2 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **builtin_tools** | 工具注册表 | name, display_name, description, category, enabled, settings(JSON), requires(JSON), metadata(JSON), tenant_id |
| **builtin_tool_tenant_configs** | 工具→租户配置 | tool_name(FK), tenant_id(FK), enabled, settings(JSON) |

---

### 3.15 安全 CLI 系统（3 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **secure_cli_binaries** | 安全 CLI 二进制 | binary_name, binary_path, description, encrypted_env(BLOB/AES加密), deny_args(JSON), deny_verbose(JSON), timeout_seconds, tips, is_global, enabled, created_by, adapter_name, tenant_id |
| **secure_cli_agent_grants** | CLI→Agent 授权 | binary_id(FK), agent_id(FK), deny_args(JSON), deny_verbose(JSON), timeout_seconds, tips, encrypted_env(BLOB), enabled, tenant_id |
| **secure_cli_user_credentials** | CLI 用户凭证 | binary_id(FK), user_id, encrypted_env(BLOB), metadata(JSON), credential_type, host_scope, tenant_id |

**唯一约束**:
- `secure_cli_binaries`: `(binary_name, tenant_id)`
- `secure_cli_agent_grants`: `(binary_id, agent_id, tenant_id)`
- `secure_cli_user_credentials`: `(binary_id, user_id, tenant_id)`

---

### 3.16 API 密钥（1 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **api_keys** | API 密钥 | name, prefix(前8字符), key_hash(SHA-256), scopes(JSON), expires_at, last_used_at, revoked, created_by, owner_id, tenant_id |

---

### 3.17 Heartbeat 心跳（2 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **agent_heartbeats** | 心跳配置 | agent_id(FK, UNIQUE), enabled, interval_sec, prompt, provider_id(FK), model, isolated_session, light_context, ack_max_chars, max_retries, active_hours_start/end, timezone, channel, chat_id, next_run_at, last_run_at, last_status, run_count, suppress_count, tenant_id |
| **heartbeat_run_logs** | 心跳执行日志 | heartbeat_id(FK), agent_id(FK), status, summary, error, duration_ms, input_tokens, output_tokens, skip_reason, ran_at, tenant_id |

---

### 3.18 配置与系统管理（3 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **config_secrets** | 加密配置存储 | key, value(BLOB/AES加密), tenant_id |
| **system_configs** | 系统配置键值 | key, value(TEXT), tenant_id |
| **system_configs** | 系统配置 | key, value, tenant_id |

---

### 3.19 Webhook 系统（2 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **webhooks** | Webhook 注册表 | agent_id(FK), name, kind(llm/message), secret_prefix, secret_hash(SHA-256), encrypted_secret(AES-GCM), scopes(JSON), channel_id, rate_limit_per_min, ip_allowlist(JSON), require_hmac, localhost_only, revoked, created_by, tenant_id |
| **webhook_calls** | 调用审计 | webhook_id(FK), agent_id, idempotency_key, mode(sync/async), callback_url, status(queued/running/done/failed/dead), attempts, delivery_id, lease_token, request_payload(JSON), response(JSON), last_error, tenant_id |

---

### 3.20 Workstation 远程工作站（4 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **workstations** | 工作站定义 | workstation_key, name, backend_type(ssh/docker), metadata(BLOB/AES加密), default_cwd, default_env(BLOB/AES加密), active, created_by, tenant_id |
| **agent_workstation_links** | Agent→工作站绑定 | agent_id(FK), workstation_id(FK), is_default, tenant_id |
| **workstation_permissions** | 工作站权限 | workstation_id(FK), pattern(binary 名称匹配), enabled, created_by, tenant_id |
| **workstation_activity** | 操作审计日志 | workstation_id(FK), agent_id, action(exec/deny), cmd_hash(SHA-256), cmd_preview(前200字符), exit_code, duration_ms, deny_reason, tenant_id |

---

### 3.21 审计与活动日志（1 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **activity_logs** | 活动审计 | actor_type(user/agent/system), actor_id, action, entity_type, entity_id, details(JSON), ip_address, tenant_id |

---

### 3.22 使用量与计费（3 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **usage_snapshots** | 使用量小时快照 | bucket_hour, agent_id, provider, model, channel, input_tokens, output_tokens, cache_read_tokens, cache_create_tokens, thinking_tokens, total_cost, request_count, llm_call_count, tool_call_count, error_count, unique_users, avg_duration_ms, memory_docs, memory_chunks, kg_entities, kg_relations, tenant_id |
| **agent_evolution_metrics** | Agent 自进化指标 | agent_id(FK), session_key, metric_type, metric_key, value, tenant_id |
| **agent_evolution_suggestions** | Agent 自进化建议 | agent_id(FK), suggestion_type, suggestion, rationale, parameters(JSON), status(pending/approved/rejected/...), reviewed_by, tenant_id |

**唯一约束**: `usage_snapshots` 通过 `idx_usage_snapshots_unique` 去重

---

### 3.23 Hooks 系统（3 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **hooks** | Hook 定义 | scope(global/tenant/agent), event, handler_type(command/http/prompt/script), config(JSON), matcher, if_expr, timeout_ms, on_timeout(block/allow), priority, enabled, version, source(ui/api/seed/builtin), name, tenant_id |
| **hook_agents** | Hook→Agent 绑定 | hook_id(FK), agent_id(FK) |
| **hook_executions** | 执行审计 | hook_id(FK), session_id, event, decision(allow/block/error/timeout), duration_ms, retry, dedup_key, error, metadata(JSON) |

---

### 3.24 其他（2 表）

| 表名 | 功能 | 核心字段 |
|------|------|----------|
| **bitrix_portals** | Bitrix24 OAuth | name, domain, credentials(BLOB/AES-GCM), state(BLOB/AES-GCM), tenant_id |
| **browser_cookies** | 浏览器 Cookie | user_id, agent_id, domain, name, path, encrypted_value(AES-256-GCM), secure, http_only, same_site, expires_at, source, tenant_id |
| **tenant_hook_budget** | Hook 月度配额 | tenant_id(PK), month_start, budget_total, remaining, last_warned_at |

---

## 4. 缓存层架构

### 4.1 Redis 缓存（可选，需 `-tags redis` 编译）

```
┌─────────────────────────────────────────────────────┐
│ Redis 缓存层                                         │
│                                                     │
│  Key: ctx:agent:{agentID}:{userID}                  │
│  Val: []AgentContextFileData (JSON 序列化)           │
│  TTL: 可配置                                         │
│                                                     │
│  Key: ctx:user:{agentID}:{userID}                   │
│  Val: []AgentContextFileData (JSON 序列化)           │
│  TTL: 可配置                                         │
│                                                     │
│  用途: 缓存 Agent/User 上下文文件，减少 PG 查询      │
│  回退: 无 Redis 时使用 Go 内存缓存 (sync.Map)       │
└─────────────────────────────────────────────────────┘
```

### 4.2 Go 内存缓存（默认）

```go
// Cache[V] 接口
type Cache[V any] interface {
    Get(ctx, key) (V, bool)
    Set(ctx, key, V, ttl)
    Delete(ctx, key)
    DeleteByPrefix(ctx, prefix)
    Clear(ctx)
}
```

---

## 5. 文件系统存储

### 5.1 媒体文件（`.media/`）

```
.media/
  └── {sessionHash}/           # 按 session 分目录
      ├── {uuid}.png
      ├── {uuid}.pdf
      └── ...
```

- **隔离**: 按 session key 的 SHA-256 哈希分目录
- **生命周期**: session 删除时整个目录被清理
- **来源**: IM 通道接收的图片/音频/视频/文档

### 5.2 技能文件（`skills-store/`）

```
skills-store/
  └── {slug}/
      ├── SKILL.md             # 技能主文件（YAML frontmatter + Markdown body）
      ├── assets/              # 静态资源
      ├── references/          # 参考文档
      └── templates/           # 模板文件
```

---

## 6. Docker 沙箱存储

### 6.1 容器组织

```
容器命名: goclaw-sbx-{tenantID[:8]}-{agentKey[:8]}-{userID[:8]}
           ↓              ↓              ↓             ↓
         前缀          租户隔离       Agent 隔离     用户隔离

Scope 模式:
  session  → 每个 Agent Run 一个新容器（完全隔离）
  agent    → 每个 Agent 复用一个容器（文件在 Agent 间共享）
  shared   → 全局共享一个容器（无文件隔离）
```

### 6.2 Workspace 管理

```
# 宿主机目录结构
{workspace}/.sandboxes/
  └── {tenant[:8]}/{agentKey[:8]}/{userID[:8]}/  # scope=agent 时
      └── workspace/                              # 绑定挂载到容器内
```

### 6.3 容器生命周期

| 状态 | 说明 |
|------|------|
| `sleep infinity` | 容器保持活跃 |
| Pruning (每 5 分钟) | 检查闲置 >24h 的容器自动销毁 |
| 全局 TTL (7 天) | 容器最大存活时间 |
| 重启重建 | GoClaw 重启时重建 scope=agent/shared 的容器 |
| 清理 | 服务关闭时删除所有 goclaw 容器和镜像 |

---

## 7. S3 备份存储（可选）

```
S3 路径: {prefix}/goclaw-backups/{version}/{timestamp}.tar.gz

配置来源: PostgreSQL config_secrets 表（加密存储）
  - endpoint: S3 兼容端点
  - region: 区域
  - access_key_id: AKID
  - secret_access_key: 密钥
  - bucket: 存储桶
  - prefix: 路径前缀
```

---

## 8. Kafka 通道（可选扩展）

```
extensions/kafka-channel/
  ├── consumer_group → 读取入站消息
  ├── producer       → 发送出站消息
  └── topic 配置:
      - inbound_topic:  接收用户消息的 topic
      - outbound_topic: 发送 Agent 回复的 topic
```

---

## 9. 跨表关系全景图

```
tenants (根)
  ├── tenant_users
  ├── llm_providers
  ├── agents
  │     ├── agent_shares
  │     ├── agent_context_files
  │     ├── user_context_files
  │     ├── user_agent_profiles
  │     ├── user_agent_overrides
  │     ├── agent_config_permissions
  │     ├── agent_links ←→ agents
  │     ├── agent_heartbeats
  │     │     └── heartbeat_run_logs
  │     ├── agent_workstation_links ←→ workstations
  │     ├── sessions
  │     │     ├── episodic_summaries
  │     │     ├── traces → spans
  │     │     └── subagent_tasks
  │     ├── memory_documents → memory_chunks
  │     ├── kg_entities → kg_relations
  │     ├── kg_dedup_candidates
  │     ├── skills ←→ skill_agent_grants
  │     ├── builtin_tools ←→ builtin_tool_tenant_configs
  │     ├── secure_cli_binaries ←→ secure_cli_agent_grants
  │     ├── mcp_servers ←→ mcp_agent_grants
  │     ├── channel_instances
  │     ├── webhooks ←→ webhook_calls
  │     └── vault_documents → vault_links
  ├── agent_teams
  │     ├── agent_team_members
  │     ├── team_tasks → team_task_comments
  │     │                  → team_task_events
  │     │                  → team_task_attachments
  │     └── team_user_grants
  ├── hooks ←→ hook_agents
  │     └── hook_executions
  ├── tenant_hook_budget
  ├── system_configs
  ├── config_secrets
  ├── api_keys
  ├── cron_jobs → cron_run_logs
  ├── pairing_requests
  ├── paired_devices
  ├── activity_logs
  ├── usage_snapshots
  └── agent_evolution_metrics
       agent_evolution_suggestions
```

---

## 10. 关键设计决策

### 10.1 多租户隔离

所有表都有 `tenant_id` 外键引用 `tenants(id)`，实现：
- 数据行级隔离
- 唯一约束都在 `(tenant_id, ...)` 上
- 查询都带 `WHERE tenant_id = ?` 过滤

### 10.2 软删除

`agents` 表使用 `deleted_at` 字段实现软删除，唯一约束 `WHERE deleted_at IS NULL` 保证删除后不冲突。

### 10.3 加密存储

敏感字段使用 AES-256-GCM 加密：
- `llm_providers.api_key`
- `channel_instances.credentials`
- `config_secrets.value`
- `mcp_user_credentials.headers/env`
- `secure_cli_binaries.encrypted_env`
- `bitrix_portals.credentials/state`
- `workstations.metadata/default_env`
- `browser_cookies.encrypted_value`

加密密钥来源: `GOCLAW_ENCRYPTION_KEY` 环境变量。

### 10.4 时间序列优化

`traces` 和 `spans` 表使用 `start_time` 而非 `created_at` 进行时间范围查询优化，配合 `(agent_id, created_at DESC)` 复合索引。

### 10.5 全量迁移覆盖

73 个迁移文件（`000001_init_schema` → `000073_secure_cli_credential_type`），覆盖所有表的创建和演进。
