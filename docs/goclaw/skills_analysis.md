# Skills 系统深度分析

> 覆盖注册、管理、创建、移除、LLM 发现使用、存储形式、权限控制的完整流程。

---

## 1. 架构总览

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Skills 系统架构                                   │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                     存储层 (双存储)                                │  │
│  │                                                                   │  │
│  │  ┌──────────────────────┐    ┌──────────────────────────────┐    │  │
│  │  │  Filesystem (Loader) │    │  PostgreSQL (PGSkillStore)    │    │  │
│  │  │                      │    │                                │    │  │
│  │  │  5 级目录层级：        │    │  skills 表 (元数据)           │    │  │
│  │  │  1. workspace/skills/ │    │  skill_agent_grants (授权)    │    │  │
│  │  │  2. .agents/skills/   │    │  skill_user_grants (用户授权) │    │  │
│  │  │  3. ~/.agents/skills/ │    │  embedding 列 (pgvector)      │    │  │
│  │  │  4. ~/.goclaw/skills/ │    │                                │    │  │
│  │  │  5. builtin (二进制)   │    │  + skills-store/ 目录         │    │  │
│  │  └──────────────────────┘    └──────────────────────────────┘    │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                     工具层 (LLM 可调用)                            │  │
│  │                                                                   │  │
│  │  skill_search  → BM25 + 向量混合搜索                               │  │
│  │  use_skill     → 标记激活（可观测性）                               │  │
│  │  skill_manage  → create / patch / delete                          │  │
│  │  read_file     → 读取 SKILL.md 内容                               │  │
│  │  publish_skill → HTTP 上传发布                                     │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                     基础设施层                                      │  │
│  │                                                                   │  │
│  │  Watcher (fsnotify)  → 监控目录变化 → BumpVersion                 │  │
│  │  Seeder              → 启动时种子化系统技能到 DB                    │  │
│  │  Guard               → 安全扫描（拒绝危险内容）                    │  │
│  │  Dependency Checker  → 检查运行时依赖                              │  │
│  └──────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Skill 存储形式

### 2.1 文件系统存储

每个 Skill 是一个目录，核心文件是 `SKILL.md`：

```
~/.goclaw/skills/
├── weather/
│   └── SKILL.md              ← 核心文件
├── translate/
│   ├── SKILL.md
│   └── scripts/
│       └── helper.py         ← 伴随文件
└── github-ops/
    └── SKILL.md
```

**SKILL.md 格式**：

```markdown
---
name: Weather Assistant
description: Access real-time weather data and forecasts
---

# Weather Assistant

## When to Use
Trigger when user asks about weather, temperature, forecasts...

## Instructions
1. Use `exec` to run weather queries
2. Format response with temperature, humidity, wind...

## Examples
User: "What's the weather in Beijing?"
Action: exec weather_api --city Beijing
```

**Frontmatter 字段**（YAML 格式）：

| 字段 | 必需 | 说明 |
|------|------|------|
| `name` | ✅ | 显示名称 |
| `description` | ❌ | 简述（用于搜索和 system prompt 注入） |

### 2.2 版本化存储（Managed Skills）

DB 管理的技能使用版本化目录结构：

```
skills-store/                          ← dataDir/skills-store/
├── weather/
│   ├── 1/                             ← version 1
│   │   └── SKILL.md
│   ├── 2/                             ← version 2 (patch 后)
│   │   └── SKILL.md
│   └── .trash/                        ← 软删除
│       └── weather.1718200000
├── translate/
│   └── 1/
│       └── SKILL.md
└── .trash/                            ← 所有被删除的技能
    └── old-skill.1718200000
```

### 2.3 PostgreSQL 存储

```sql
-- skills 表：元数据 + 向量
CREATE TABLE skills (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,          -- 目录名（唯一标识）
    description TEXT,
    owner_id TEXT NOT NULL,             -- 创建者 ID
    visibility TEXT DEFAULT 'private',  -- private/internal/public
    version INT DEFAULT 1,
    is_system BOOLEAN DEFAULT false,    -- 系统技能不可删除
    status TEXT DEFAULT 'active',       -- active/archived/deleted
    enabled BOOLEAN DEFAULT true,
    file_path TEXT,                     -- 文件系统路径
    file_size INT8,
    file_hash TEXT,                     -- SHA-256
    embedding vector(1536),             -- pgvector 向量列
    missing_deps TEXT[],                -- 缺失依赖
    author TEXT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);

-- skill_agent_grants 表：Agent 级授权
CREATE TABLE skill_agent_grants (
    id UUID PRIMARY KEY,
    skill_id UUID NOT NULL REFERENCES skills(id),
    agent_id UUID NOT NULL REFERENCES agents(id),
    pinned_version INT NOT NULL,        -- 锁定版本
    granted_by TEXT NOT NULL,           -- 授权人
    can_manage BOOLEAN DEFAULT false,   -- 是否可管理
    created_at TIMESTAMPTZ,
    tenant_id UUID NOT NULL,
    UNIQUE (skill_id, agent_id)
);

-- skill_user_grants 表：用户级授权
CREATE TABLE skill_user_grants (
    id UUID PRIMARY KEY,
    skill_id UUID NOT NULL REFERENCES skills(id),
    user_id TEXT NOT NULL,
    granted_by TEXT NOT NULL,
    created_at TIMESTAMPTZ,
    tenant_id UUID NOT NULL,
    UNIQUE (skill_id, user_id)
);
```

### 2.4 Embedding 缓存

向量嵌入直接存储在 `skills.embedding` 列（pgvector），无需单独缓存表：

```sql
-- 向量搜索 SQL
SELECT name, slug, description, version, file_path,
    1 - (embedding <=> $1::vector) AS score
FROM skills
WHERE status = 'active' AND enabled = true AND embedding IS NOT NULL
    AND visibility != 'private'
ORDER BY embedding <=> $1::vector
LIMIT 5;
```

---

## 3. LLM 发现和使用 Skills 的完整流程

### 3.1 System Prompt 中的技能注入

```
Pipeline ContextStage
    │
    ├─ LoadContextFiles()          → 加载 AGENTS.md, SOUL.md, USER.md...
    ├─ BuildMessages()
    │   └─ BuildSystemPrompt()
    │       ├─ resolveSkillsSummary(ctx, skillFilter)
    │       │   └─ skillsLoader.BuildSummary(ctx, allowList)
    │       │       → 返回 XML 格式的 <available_skills> 摘要
    │       │
    │       └─ append to system prompt
    │
    └─ System Prompt 中的 Skills 部分：

## Skills (mandatory)

Before replying, scan `<available_skills>` below.
If a skill clearly applies, read its SKILL.md at the `<location>` path with `read_file`, then follow it.

<available_skills>
  <skill>
    <name>weather</name>
    <description>Access real-time weather data and forecasts</description>
    <location>/app/workspace/skills/weather/SKILL.md</location>
  </skill>
  <skill>
    <name>translate</name>
    <description>Translate text between languages</description>
    <location>~/.goclaw/skills/translate/SKILL.md</location>
  </skill>
  ...
</available_skills>
```

### 3.2 LLM 使用技能的完整调用链

```
用户: "北京今天天气怎么样？"
    │
    ▼
LLM 收到 system prompt 中的 <available_skills>
    │ LLM 内部决策: 这个问题匹配 "weather" 技能
    │
    ├─ Step 1: LLM 调用 use_skill(name="weather")
    │   │
    │   └─ UseSkillTool.Execute() → 返回 "Skill activated. Proceed to read..."
    │      （纯可观测性标记，实际是 no-op）
    │
    ├─ Step 2: LLM 调用 read_file(path="/app/workspace/skills/weather/SKILL.md")
    │   │
    │   └─ ReadFileTool.Execute() → 返回 SKILL.md 完整内容
    │      LLM 读取到：使用 weather_api 工具的指令...
    │
    ├─ Step 3: LLM 按照 SKILL.md 指令执行
    │   │
    │   └─ exec(command="curl wttr.in/Beijing?format=j1")
    │
    └─ Step 4: LLM 整合结果回复用户
        "北京今天晴，气温 28°C，湿度 45%..."
```

### 3.3 技能搜索流程（search 模式）

当技能数量过多无法全部 inline 时，使用 search 模式：

```
LLM 收到 system prompt:
## Skills (mandatory)
Before replying, check if a skill applies:
1. Run `skill_search` with English keywords describing the domain
2. If a match is found, read its SKILL.md at the returned `location`
3. If no match, proceed normally.

    │
    ▼
LLM 调用 skill_search(query="weather forecast")
    │
    ▼
SkillSearchTool.Execute()
    │
    ├─ 1. ensureIndex() → 检查 loader version 是否变化
    │   └─ rebuildIndex() if changed → BM25 索引重建
    │
    ├─ 2. BM25 搜索（本地内存索引）
    │   index.Search("weather forecast", 10)
    │   → 基于 TF-IDF 的关键词匹配
    │
    ├─ 3. 向量搜索（可选，混合模式）
    │   ├─ provider.Embed(["weather forecast"]) → OpenAI Embedding API
    │   ├─ embSearcher.SearchByEmbedding(embedding, 10)
    │   │   → PostgreSQL: 1 - (embedding <=> $1::vector)
    │   └─ hybridMerge(bm25, vector, 0.3, 0.7)
    │
    ├─ 4. 权限过滤
    │   └─ filterByAccess(ctx, results)
    │       → ListAccessible(agentID, userID) → 只返回授权技能
    │
    └─ 5. 返回结果 + 指令
        {
          "results": [{name: "weather", score: 0.95, location: "..."}],
          "count": 1
        }
        ACTION REQUIRED: Call use_skill with name "weather", then read_file...
```

---

## 4. 技能创建、修改、删除完整流程

### 4.1 Agent 自主创建（skill_manage 工具）

```
LLM 决定创建新技能: "用户经常问汇率，我应该创建一个 skill"
    │
    ▼
skill_manage(action="create", content="---
name: Currency Converter
description: Convert currencies using live exchange rates
---
# Currency Converter
...
", visibility="private")
    │
    ▼
SkillManageTool.executeCreate()
    │
    ├─ 1. 内容大小检查 (max 100KB)
    │
    ├─ 2. 安全扫描 (GuardSkillContent)
    │   violations, safe := skills.GuardSkillContent(content)
    │   // 扫描: rm -rf, fork bomb, base64 decode pipe, curl|sh,
    │   //       /etc/passwd, .ssh/id_rsa, DROP TABLE, sudo...
    │   if !safe → 拒绝创建
    │
    ├─ 3. 解析 Frontmatter
    │   name, description, slug := skills.ParseSkillFrontmatter(content)
    │   slug = skills.Slugify(name) → "currency-converter"
    │
    ├─ 4. 系统技能保护
    │   if t.skills.IsSystemSkill(slug) → 拒绝
    │
    ├─ 5. 计算版本号
    │   version = t.skills.GetNextVersion(ctx, slug) → 1
    │
    ├─ 6. 写入文件系统
    │   destDir = skills-store/{tenant}/{slug}/{version}/
    │   os.MkdirAll(destDir)
    │   os.WriteFile(destDir + "/SKILL.md", content)
    │
    ├─ 7. DB 注册
    │   t.skills.CreateSkillManaged(ctx, SkillCreateParams{
    │       Name: "Currency Converter",
    │       Slug: "currency-converter",
    │       OwnerID: "telegram:386246614",
    │       Visibility: "private",
    │       Version: 1,
    │       FilePath: destDir,
    │       FileHash: sha256(content),
    │   })
    │   → INSERT INTO skills (...) VALUES (...)
    │
    ├─ 8. 自动授权给当前 Agent
    │   t.skills.GrantToAgent(ctx, skillID, agentID, version, ownerID, true)
    │   → INSERT INTO skill_agent_grants (..., can_manage=true)
    │
    ├─ 9. 版本号递增（通知其他实例）
    │   t.loader.BumpVersion()
    │
    ├─ 10. 依赖检查（异步）
    │   manifest := skills.ScanSkillDeps(destDir)
    │   ok, missing := skills.CheckSkillDeps(manifest)
    │   if !ok → t.skills.StoreMissingDeps(ctx, id, missing)
    │
    └─ 返回: "Skill created. v1. Will appear in search on next turn."
```

### 4.2 Patch（内容修改 + 新版本）

```
skill_manage(action="patch", slug="currency-converter",
    find="old text", replace="new text")
    │
    ▼
executePatch()
    │
    ├─ 1. 所有权检查
    │   canManageSkill(ctx, skills, info)
    │   → 检查 owner_id == actor/user/sender
    │   → 或 Agent 有 can_manage grant
    │
    ├─ 2. 读取当前版本 SKILL.md
    │   current = os.ReadFile(info.Path)
    │
    ├─ 3. 应用 find/replace
    │   patched = strings.Replace(current, find, replace, 1)
    │
    ├─ 4. 安全扫描（同创建）
    │
    ├─ 5. 获取新版本号（带锁）
    │   newVer, commitLock := t.skills.GetNextVersionLocked(ctx, slug)
    │   → pg_advisory_xact_lock 防止并发竞争
    │
    ├─ 6. 写入新版本目录
    │   destDir = skills-store/{slug}/{newVer}/
    │   os.WriteFile(destDir + "/SKILL.md", patched)
    │   copyOtherFiles(oldBaseDir, destDir)  // 拷贝伴随文件
    │
    ├─ 7. DB 更新
    │   t.skills.UpdateSkill(ctx, skillID, {version: newVer, file_path: destDir, ...})
    │
    └─ 返回: "Skill patched. v1 → v2. Changes active next turn."
```

### 4.3 Delete（软删除）

```
skill_manage(action="delete", slug="currency-converter")
    │
    ▼
executeDelete()
    │
    ├─ 1. 所有权检查
    │
    ├─ 2. 系统技能保护
    │   if t.skills.IsSystemSkill(slug) → 拒绝
    │
    ├─ 3. 磁盘软删除
    │   os.Rename(skills-store/{slug}, skills-store/.trash/{slug}.{timestamp})
    │
    ├─ 4. DB 软删除
    │   t.skills.DeleteSkill(ctx, skillID)
    │   → 在事务中:
    │     DELETE FROM skill_agent_grants WHERE skill_id = $1
    │     DELETE FROM skill_user_grants WHERE skill_id = $1
    │     UPDATE skills SET status = 'deleted' WHERE id = $1
    │
    └─ 返回: "Skill deleted and removed from search."
```

---

## 5. 权限控制体系

### 5.1 三级 Visibility

```
┌─────────────┬──────────────────────────────────────────────────────┐
│  Visibility │ 含义                                                 │
├─────────────┼──────────────────────────────────────────────────────┤
│  "private"  │ 仅创建者可见，不进入 search 结果                      │
│             │ 默认值，新创建的技能                                   │
├─────────────┼──────────────────────────────────────────────────────┤
│  "internal" │ 通过 grant 授权给特定 Agent 可见                      │
│             │ GrantToAgent 时自动从 private 提升                     │
│             │ RevokeFromAgent 时自动降回 private                     │
├─────────────┼──────────────────────────────────────────────────────┤
│  "public"   │ 所有 Agent 都可见，出现在所有搜索结果中                │
│             │ 仅系统技能使用                                        │
└─────────────┴──────────────────────────────────────────────────────┘
```

### 5.2 授权矩阵

```
┌─────────────────────┬─────────┬──────────┬──────────┬──────────┐
│  操作               │ 创建者  │ 有 grant │ 无 grant │ 系统技能  │
│                     │         │ can_manage│          │ (is_system)│
├─────────────────────┼─────────┼──────────┼──────────┼──────────┤
│  查看 (ListSkills)  │   ✅    │    ✅    │   ❌*    │    ✅    │
│  搜索 (skill_search)│   ✅    │    ✅    │   ❌     │    ✅    │
│  Patch              │   ✅    │    ✅    │   ❌     │    ❌    │
│  Delete             │   ✅    │    ✅    │   ❌     │    ❌    │
│  Change Visibility  │   ✅    │    ✅    │   ❌     │    ❌    │
└─────────────────────┴─────────┴──────────┴──────────┴──────────┘
* private 技能对其他 Agent 不可见，但创建者始终可见
```

### 5.3 权限检查代码路径

```go
// tools/skill_manage.go

// 所有权检查（三种身份匹配，兼容 #915 合并前后的行）
func canManageSkill(ctx, skills, info) bool {
    // 1. 检查 owner_id == 当前 actor/user/sender
    if isOwnerOfSkill(ctx, skills, info.Slug) {
        return true
    }
    // 2. 检查 Agent 是否有 can_manage grant
    skillID := uuid.Parse(info.ID)
    agentID := store.AgentIDFromContext(ctx)
    ok, _ := skills.AgentCanManageSkill(ctx, skillID, agentID)
    return ok
}

// ListAccessible: 查询 Agent 可访问的技能
func (s *PGSkillStore) ListAccessible(ctx, agentID, userID) ([]SkillInfo, error) {
    // SQL: SELECT ... FROM skills WHERE
    //   (visibility = 'public')
    //   OR (visibility = 'internal' AND id IN (
    //       SELECT skill_id FROM skill_agent_grants WHERE agent_id = $1
    //       UNION
    //       SELECT skill_id FROM skill_user_grants WHERE user_id = $2
    //   ))
    //   AND status = 'active' AND enabled = true
    //   AND tenant_id = $3
}
```

### 5.4 Grant 自动升降级

```go
// pg/skills_grants.go

// GrantToAgent: 授予时自动提升 visibility
func (s *PGSkillStore) GrantToAgent(...) error {
    // 1. INSERT INTO skill_agent_grants (...)
    // 2. 自动提升: private → internal
    UPDATE skills SET visibility = 'internal'
    WHERE id = $1 AND visibility = 'private'
}

// RevokeFromAgent: 撤销时自动降级
func (s *PGSkillStore) RevokeFromAgent(...) error {
    // 1. DELETE FROM skill_agent_grants
    // 2. 自动降级: internal → private (仅当无剩余 grant 时)
    UPDATE skills SET visibility = 'private'
    WHERE id = $1 AND visibility = 'internal'
    AND NOT EXISTS (SELECT 1 FROM skill_agent_grants WHERE skill_id = $1)
    AND NOT EXISTS (SELECT 1 FROM skill_user_grants WHERE skill_id = $1)
}
```

---

## 6. 安全机制

### 6.1 内容安全扫描（GuardSkillContent）

创建或 patch 时强制扫描，拒绝危险模式：

```go
// skills/guard.go
var skillGuardRules = []guardRule{
    // 破坏性命令
    {`rm\s+-rf\s+[/~]`,                    "destructive rm -rf"},
    {`:\s*\|\s*:\s*&`,                     "fork bomb"},
    {`\bdd\b.+\bof=/dev/`,                "disk overwrite via dd"},

    // 代码注入
    {`base64\s+(--decode|-d)\s*\|`,        "base64 decode pipe to shell"},
    {`curl\s+\S+\s*\|\s*(ba)?sh`,          "curl pipe to shell"},
    {`python[23]?\s+-c\s+.{0,40}exec\s*\(`, "python exec injection"},

    // 凭证泄露
    {`/etc/passwd`,                         "password file access"},
    {`\.ssh/id_rsa`,                        "SSH key access"},
    {`AWS_SECRET_ACCESS_KEY`,               "AWS credential reference"},
    {`GOCLAW_DB_URL`,                       "GoClaw DB credential"},

    // 路径穿越
    {`\.\./\.\./\.\./`,                    "deep path traversal"},

    // SQL 注入
    {`\bDROP\s+TABLE\b`,                   "SQL DROP TABLE"},

    // 权限提升
    {`\bsudo\s`,                            "sudo usage"},
}
```

### 6.2 系统技能保护

```go
// 系统技能 (is_system=true) 不能被 Agent 修改或删除
if t.skills.IsSystemSkill(slug) {
    return ErrorResult("cannot manage system skill")
}

// DeleteSkill 中:
if isSystem {
    return fmt.Errorf("cannot delete system skill")
}
```

### 6.3 版本锁定（防并发竞争）

```go
// pg/skills_crud.go - CreateSkillManaged
// 使用 pg_advisory_xact_lock 防止并发上传同一 slug 的竞争
lockKey := tenantSlugAdvisoryLock(tenantID, slug)
tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey)
// 在锁保护下计算版本号 + INSERT/UPDATE
```

### 6.4 内容大小限制

```go
const maxSkillContentSize = 100 * 1024  // 100KB
const maxCopySize = 20 << 20            // 20MB (伴随文件)
```

---

## 7. 完整调用栈

```
=== 启动时 ===

runGateway()
  ├─ setupSkillsSystem()
  │   ├─ NewLoader(workspace, globalSkills, builtinSkills)  → 5 级目录发现
  │   ├─ NewPGSkillStore(db, baseDir)                       → PostgreSQL 元数据
  │   ├─ Seeder.Seed()                                      → 种子化系统技能
  │   │   └─ 遍历 bundled-skills/ → UpsertSystemSkill → 复制到 skills-store/
  │   ├─ NewSkillSearchTool(loader)                         → BM25 索引构建
  │   └─ Watcher.Start(ctx)                                 → fsnotify 监控目录变化

=== 每次对话 ===

Pipeline.ContextStage.Execute()
  ├─ buildMessages() → resolveSkillsSummary(ctx, skillFilter)
  │   └─ skillsLoader.BuildSummary(ctx, allowList)
  │       └─ 遍历 5 级目录 → parseMetadata(SKILL.md) → 构建 <available_skills> XML
  │
  └─ System Prompt 包含技能摘要（inline 或 search 模式）

LLM 决策使用技能:
  ├─ skill_search(query="weather")     → BM25 + 向量搜索
  │   ├─ ensureIndex()                 → 检查 version → rebuildIndex if stale
  │   ├─ index.Search(query)           → BM25 本地搜索
  │   ├─ hybridSearch()                → OpenAI Embedding + pgvector
  │   └─ filterByAccess()              → 权限过滤 (ListAccessible)
  │
  ├─ use_skill(name="weather")         → 可观测性标记（no-op）
  │
  ├─ read_file(path="skills/weather/SKILL.md")  → 读取完整指令
  │   └─ loader.LoadSkill(ctx, "weather")
  │       └─ 按优先级: workspace → agents → global → managed → builtin
  │
  └─ 按 SKILL.md 指令执行任务

=== Agent 自主管理 ===

skill_manage(action="create", content="...", visibility="private")
  ├─ GuardSkillContent(content)        → 安全扫描
  ├─ ParseSkillFrontmatter(content)    → 解析 name/slug
  ├─ os.WriteFile(SKILL.md)            → 写入 skills-store/{slug}/{version}/
  ├─ CreateSkillManaged(...)           → INSERT INTO skills
  ├─ GrantToAgent(...)                 → INSERT INTO skill_agent_grants + visibility 提升
  ├─ loader.BumpVersion()              → 通知其他实例
  └─ ScanSkillDeps()                   → 依赖检查（异步）
```

---

## 8. 技能是否可以被修改？

**可以，但有条件**：

| 场景 | 可否修改 | 条件 |
|------|---------|------|
| Agent 自己创建的技能 | ✅ | `skill_manage(action="patch")` |
| 有 `can_manage` grant 的技能 | ✅ | 需要 grant 授权 |
| 他人的 private 技能 | ❌ | 所有权不匹配 |
| 系统技能 (is_system) | ❌ | 硬保护，不可修改/删除 |
| 已 archived 的技能 | ❌ | 搜索不可见，不可 patch |
| 外部文件系统技能 | ⚠️ | 可直接编辑 SKILL.md 文件，watcher 自动检测变化 |

**修改方式**：
1. **LLM 工具**: `skill_manage(action="patch", slug=..., find=..., replace=...)`
2. **直接编辑文件**: 修改 `skills/{name}/SKILL.md`，Watcher 500ms 内自动检测并 BumpVersion
3. **HTTP API**: `POST /api/skills/{id}` (管理员)
4. **CLI**: `goclaw skill upload` (上传新版本)
