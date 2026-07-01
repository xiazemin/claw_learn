# PostgreSQL pgvector 向量嵌入与搜索

基于 PostgreSQL + pgvector 扩展的完整向量搜索学习项目。包含 7 个独立示例，覆盖从基础操作到 RAG 管道的全流程。

## 快速开始

```bash
cd D:\code\backend\test\pgvector-demo

# 启动 PostgreSQL + pgvector 容器
docker compose up -d

# 运行示例 (1-7)
go run . 1    # 基础操作
go run . 2    # 相似度搜索
go run . 3    # 索引对比 (耗时较长)
go run . 4    # 批量操作
go run . 5    # 过滤搜索
go run . 6    # 本地嵌入
go run . 7    # RAG 管道
```

### 连接信息

```
Host:     localhost:5434
User:     pgvec
Password: pgvec123
Database: pgvector_demo
```

---

## 7 个示例说明

### Example 1: 基础操作 (`example1_basics.go`)

连接数据库、创建 pgvector 扩展、建表（含 `vector(N)` 列）、插入向量、基本查询。

**核心知识点：**
- `CREATE EXTENSION IF NOT EXISTS vector` — 启用 pgvector 扩展
- `CREATE TABLE t (embedding vector(384))` — 定义向量列及维度
- `pgvector.NewVector([]float32{...})` — Go 类型 → pgvector 向量

### Example 2: 相似度搜索 (`example2_similarity_search.go`)

三种距离度量的对比，附手算验证。

**核心知识点：**

| 运算符 | 度量方式 | 公式 | 范围 |
|--------|---------|------|------|
| `<->` | L2 欧氏距离 | √Σ(aᵢ-bᵢ)² | [0, +∞) |
| `<=>` | 余弦距离 | 1 - cos(θ) | [0, 2] |
| `<#>` | 负内积 | -Σ(aᵢ×bᵢ) | (-∞, 0] |

**SQL 示例：**
```sql
-- 找到与查询向量最近的 5 条
SELECT id, embedding <-> $1 AS dist
FROM items ORDER BY embedding <-> $1 LIMIT 5;
```

### Example 3: 向量索引 (`example3_indexes.go`)

HNSW vs IVFFlat 索引的构建速度、查询性能、精度对比（10000 条 128 维向量）。

**核心知识点：**

```sql
-- HNSW 索引（推荐，查询快、精度高）
CREATE INDEX ON items USING hnsw (embedding vector_l2_ops)
  WITH (m = 16, ef_construction = 64);

-- IVFFlat 索引（构建快，大数据集适用）
CREATE INDEX ON items USING ivfflat (embedding vector_l2_ops)
  WITH (lists = 100);  -- lists ≈ N/1000

-- 调节查询精度
SET LOCAL hnsw.ef_search = 100;  -- 越大越精确，越慢
```

| | HNSW | IVFFlat | 暴力搜索 |
|---|---|---|---|
| 构建速度 | 慢 | 快 | - |
| 查询速度 | 快 | 中 | 慢 |
| 精度 | 高 | 中 | 精确 |
| 动态增删 | 支持 | 需重建 | 自动 |

### Example 4: 批量操作 (`example4_batch_operations.go`)

COPY 高速批量插入、分页查询、Window Function 分组 Top-K。

**核心知识点：**
- `pgx.CopyFromSlice` — 高速批量写入（~38K 行/秒）
- `LIMIT ... OFFSET` — 向量搜索分页
- `ROW_NUMBER() OVER (PARTITION BY ...)` — 每类 Top-K

### Example 5: 过滤搜索 (`example5_filtered_search.go`)

向量搜索 + WHERE 元数据过滤的混合查询。

**核心知识点：**
```sql
-- 语言 + 分类 + 年份过滤
SELECT title, embedding <-> $1 AS dist
FROM docs
WHERE language = 'zh' AND category = 'tech' AND year = 2024
ORDER BY embedding <-> $1 LIMIT 5;

-- 距离阈值过滤
SELECT title, embedding <-> $1 AS dist
FROM docs
WHERE embedding <-> $1 < 2.0
ORDER BY embedding <-> $1;
```

### Example 6: 本地嵌入 (`example6_local_embeddings.go`)

不依赖外部 API 的嵌入实现：TF-IDF + SimHash。

**核心知识点：**
- TF-IDF 向量化 — 词频-逆文档频率，需训练词汇表
- SimHash — 哈希投影，即时计算
- 两者均无语义理解能力，仅作流程演示

> 生产环境应使用真实 embedding 模型（见下方 API 列表）

### Example 7: RAG 管道 (`example7_rag_pipeline.go`)

完整的检索增强生成（RAG）流程演示。

**流程：**
```
文档 → 分块(Chunking) → 向量化(Embedding) → 存入pgvector
                                                    ↓
用户查询 → 查询向量化 → 相似度检索(Top-K) → 上下文拼装 → Prompt
```

**包含能力：**
- 文档分块（按句子、固定长度）
- HNSW 索引加速检索
- MMR（最大边际相关性）去重，增加结果多样性
- JSONB 元数据存储
- RAG Prompt 拼装

---

## 免费 Embedding API 选型

项目中的 `randomFloat32()` 仅用于演示流程。实际项目需要真实 embedding 模型。以下为可免费使用的 API：

### 完全免费 / 大额免费

| 平台 | 模型 | 免费额度 | 维度 | 接口兼容 | 说明 |
|------|------|---------|------|---------|------|
| **Google AI Studio** | `text-embedding-004` | 1500 RPM | 768 | Gemini API | Google 账号即可，无需信用卡 |
| **Google AI Studio** | `gemini-embedding-exp-03-07` | 1500 RPM | 3072 | Gemini API | 实验模型，效果更强 |
| **阿里云百炼** | `text-embedding-v3` | 新人赠金 | 1024/2048 | OpenAI 兼容 | ¥0.7/百万 tokens |
| **硅基流动** | `BAAI/bge-m3` 等 | 注册送额度 | 多种 | OpenAI 兼容 | 国内注册方便 |
| **HuggingFace** | 任选开源模型 | 免费推理 API | 各异 | HF Inference API | 有速率限制 |

### 有免费层（需注册）

| 平台 | 模型 | 免费额度 | 维度 | 接口兼容 | 说明 |
|------|------|---------|------|---------|------|
| **Jina AI** | `jina-embeddings-v3` | 100万 tokens/月 | 1024 | OpenAI 兼容 | 多语言效果好 |
| **OpenAI** | `text-embedding-3-small` | 新账号 $5 赠金 | 1536 | 原生 | $0.02/百万 tokens |
| **OpenAI** | `text-embedding-3-large` | 同上 | 3072 | 原生 | $0.13/百万 tokens |
| **Cohere** | `embed-v4` | 1000 次/月 | 1024 | Cohere SDK | 多模态支持 |
| **Voyage AI** | `voyage-3` | 5000万 tokens | 1024 | OpenAI 兼容 | RAG 效果领先 |

### 本地部署（完全免费）

| 工具 | 代表模型 | 维度 | 说明 |
|------|---------|------|------|
| **Ollama** | `nomic-embed-text` | 768 | `ollama pull nomic-embed-text` 即用 |
| **Ollama** | `mxbai-embed-large` | 1024 | 更高精度 |
| **HuggingFace TEI** | BGE-M3、E5 等 | 各异 | Docker 部署，生产级 |
| **ONNX Runtime** | all-MiniLM-L6-v2 | 384 | 嵌入应用，极致轻量 |

### 场景推荐

```
最快上手（零成本）  → Google AI Studio text-embedding-004
国内最佳           → 硅基流动 SiliconFlow + BAAI/bge-m3
生产级 RAG         → Voyage AI voyage-3（5000万免费 tokens）
完全离线/无限制     → Ollama + nomic-embed-text
```

---

## 集成真实 Embedding API 示例

以硅基流动（OpenAI 兼容接口）为例，替换 `randomFloat32`：

```go
import (
    "bytes"
    "encoding/json"
    "net/http"
)

type EmbeddingRequest struct {
    Model string   `json:"model"`
    Input []string `json:"input"`
}

type EmbeddingResponse struct {
    Data []struct {
        Embedding []float32 `json:"embedding"`
    } `json:"data"`
}

func getEmbedding(text string) ([]float32, error) {
    reqBody, _ := json.Marshal(EmbeddingRequest{
        Model: "BAAI/bge-m3",
        Input: []string{text},
    })

    req, _ := http.NewRequest("POST",
        "https://api.siliconflow.cn/v1/embeddings",
        bytes.NewReader(reqBody))
    req.Header.Set("Authorization", "Bearer YOUR_API_KEY")
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result EmbeddingResponse
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Data[0].Embedding, nil
}

// 使用：
// vec, _ := getEmbedding("Go语言并发编程")
// _, err = conn.Exec(ctx, "INSERT INTO docs (embedding) VALUES ($1)", pgvector.NewVector(vec))
```

---

## 技术栈

| 组件 | 版本 | 说明 |
|------|------|------|
| PostgreSQL | 18.4 | 数据库 |
| pgvector | 0.8.2 | 向量扩展 |
| pgx | v5.7.4 | Go PostgreSQL 驱动 |
| pgvector-go | v0.4.0 | Go pgvector 类型绑定 |
| Docker | - | 容器化部署 |

## 文件结构

```
pgvector-demo/
├── docker-compose.yml            # PostgreSQL + pgvector 容器
├── go.work                       # Go 工作区
├── go.mod / go.sum               # Go 依赖
├── main.go                       # 入口路由
├── util.go                       # 共享工具函数
├── example1_basics.go            # 基础操作
├── example2_similarity_search.go # 相似度搜索
├── example3_indexes.go           # HNSW vs IVFFlat
├── example4_batch_operations.go  # 批量操作
├── example5_filtered_search.go   # 过滤搜索
├── example6_local_embeddings.go  # 本地嵌入
├── example7_rag_pipeline.go      # RAG 完整管道
└── README.md                     # 本文档
```
