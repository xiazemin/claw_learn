package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

// Example 7: RAG 管道 — 完整的检索增强生成流程
// 文档 → 分块 → 嵌入 → 存储 → 查询 → 检索上下文 → 拼装 prompt
func example7_rag_pipeline() {
	divider("Example 7: RAG 管道 (检索增强生成)")
	ctx := context.Background()

	conn, err := connect(ctx)
	check(err)
	defer conn.Close(ctx)

	// --- 7.1 创建 RAG 知识库表 ---
	divider("7.1 创建知识库表")
	_, err = conn.Exec(ctx, "DROP TABLE IF EXISTS knowledge_base")
	check(err)
	_, err = conn.Exec(ctx, `
		CREATE TABLE knowledge_base (
			id            bigserial PRIMARY KEY,
			source        text NOT NULL,          -- 来源文档名
			chunk_index   int NOT NULL,           -- 分块序号
			content       text NOT NULL,          -- 分块文本内容
			token_count   int,                    -- 估算 token 数
			metadata      jsonb DEFAULT '{}'::jsonb,     -- 额外元数据
			embedding     vector(32)              -- 嵌入向量
		)
	`)
	check(err)
	_, err = conn.Exec(ctx, "CREATE INDEX ON knowledge_base USING hnsw (embedding vector_cosine_ops)")
	check(err)
	fmt.Println("✅ 知识库表已创建 (含 HNSW 余弦索引)")

	// --- 7.2 文档分块 ---
	divider("7.2 文档分块 (Chunking)")
	// 模拟几篇技术文档
	documents := map[string]string{
		"Go并发编程.md": `Go 语言的并发编程是其核心特性之一。goroutine 是 Go 中轻量级的执行线程，
由 Go 运行时调度。channel 是 goroutine 之间通信的桥梁，支持有缓冲和无缓冲两种模式。
select 语句允许同时监听多个 channel 操作。sync 包提供了互斥锁、读写锁、WaitGroup 等同步原语。
context 包用于控制 goroutine 的生命周期和超时。`, 

		"PostgreSQL向量搜索.md": `PostgreSQL 通过 pgvector 扩展支持向量存储和检索。pgvector 添加了 vector、
halfvec、sparsevec 等数据类型。支持 L2 距离 (<->)、余弦距离 (<=>)、内积 (<#>) 三种度量。
HNSW 索引适用于高召回率场景，IVFFlat 索引适用于大数据集。向量维度最高支持 16000 维。
生产环境建议维度 >= 128 以获得较好的区分度。`,

		"RAG架构设计.md": `RAG (Retrieval-Augmented Generation) 是一种结合检索和生成的 AI 架构。
核心流程: 文档分块 → 向量化 → 索引存储 → 用户查询向量化 → 相似度检索 → 上下文拼装 → LLM 生成。
分块策略: 固定长度分块、语义分块、递归分块。推荐 chunk_size=512, overlap=50。
向量模型选择: OpenAI text-embedding-3-small (1536维)、BGE-small (512维)。
检索优化: MMR (最大边际相关性) 去重、混合检索 (向量+关键词)、重排序 (reranker)。`,

		"Docker容器化.md": `Docker 是最流行的容器化平台。容器是镜像的运行实例，镜像采用分层存储。
Dockerfile 定义镜像构建步骤。多阶段构建可以减小最终镜像体积。
Docker Compose 用于定义和运行多容器应用。网络模式包括 bridge、host、overlay。
Kubernetes 是容器编排平台，管理大规模容器集群。核心概念: Pod、Service、Deployment。`,
	}

	type Chunk struct {
		Source string
		Index  int
		Content string
	}
	var chunks []Chunk

	for source, content := range documents {
		sentences := splitSentences(content)
		// 简单分块: 每 3 句一块
		chunkSize := 3
		for i := 0; i < len(sentences); i += chunkSize {
			end := i + chunkSize
			if end > len(sentences) {
				end = len(sentences)
			}
			chunk := strings.TrimSpace(strings.Join(sentences[i:end], ""))
			if len(chunk) > 0 {
				chunks = append(chunks, Chunk{
					Source:  source,
					Index:  len(chunks),
					Content: chunk,
				})
			}
		}
	}

	fmt.Printf("✅ %d 篇文档 → %d 个分块\n", len(documents), len(chunks))
	for i, c := range chunks {
		preview := c.Content
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}
		fmt.Printf("  [%d] %s (chunk_%d): %s\n", i+1, c.Source, c.Index, preview)
	}

	// --- 7.3 向量化并存储 ---
	divider("7.3 向量化并存入 pgvector")
	// 使用 TF-IDF 作为本地嵌入（生产环境用真实 embedding 模型）
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Content
	}
	tfidf := NewTFIDFVectorizer(32)
	vectors := tfidf.FitTransform(texts)

	for i, c := range chunks {
		vec := pgvector.NewVector(vectors[i])
		_, err = conn.Exec(ctx,
			`INSERT INTO knowledge_base (source, chunk_index, content, token_count, metadata, embedding)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			c.Source, c.Index, c.Content, len(tokenize(c.Content)),
			fmt.Sprintf(`{"chunk_size": %d}`, len(c.Content)), vec)
		check(err)
	}
	fmt.Printf("✅ %d 个分块已向量化并存储\n\n", len(chunks))

	// --- 7.4 检索 ---
	divider("7.4 检索 (Retrieval)")

	type SearchResult struct {
		ID        int64
		Source    string
		ChunkIdx  int
		Content   string
		Score     float64
	}

	retrieve := func(query string, topK int) []SearchResult {
		qvec := pgvector.NewVector(tfidf.Transform(query))
		rows, err := conn.Query(ctx,
			`SELECT id, source, chunk_index, content, 1 - (embedding <=> $1) AS score
			 FROM knowledge_base
			 ORDER BY embedding <=> $1
			 LIMIT $2`,
			qvec, topK)
		check(err)
		defer rows.Close()

		var results []SearchResult
		for rows.Next() {
			var r SearchResult
			err = rows.Scan(&r.ID, &r.Source, &r.ChunkIdx, &r.Content, &r.Score)
			check(err)
			results = append(results, r)
		}
		return results
	}

	// 测试查询
	queries := []string{
		"Go怎么实现并发?",
		"什么是向量索引?",
		"如何分块文档?",
		"容器怎么编排?",
	}

	for _, q := range queries {
		fmt.Printf("🔍 查询: %q\n", q)
		results := retrieve(q, 3)
		for i, r := range results {
			content := r.Content
			if len(content) > 80 {
				content = content[:80] + "..."
			}
			fmt.Printf("  #%d [%s] (score=%.4f)\n", i+1, r.Source, r.Score)
			fmt.Printf("      %s\n\n", content)
		}
	}

	// --- 7.5 拼装 Prompt ---
	divider("7.5 拼装 RAG Prompt")
	userQuery := "Go 中 channel 和 goroutine 配合使用的最佳实践是什么?"
	fmt.Printf("用户问题: %q\n\n", userQuery)

	results := retrieve(userQuery, 3)

	var contextParts []string
	for i, r := range results {
		contextParts = append(contextParts,
			fmt.Sprintf("[来源 %d: %s]\n%s", i+1, r.Source, r.Content))
	}

	prompt := fmt.Sprintf(`你是一个技术文档问答助手。请基于以下参考资料回答用户问题。
如果参考资料中没有相关信息，请如实说明。

=== 参考资料 ===
%s

=== 用户问题 ===
%s

=== 回答要求 ===
1. 仅基于参考资料回答，不要编造
2. 引用来源编号
3. 保持简洁准确`, strings.Join(contextParts, "\n\n"), userQuery)

	fmt.Println("┌─ Generated Prompt ─────────────────────────────────┐")
	for _, line := range strings.Split(prompt, "\n") {
		fmt.Printf("│ %s\n", line)
	}
	fmt.Println("└────────────────────────────────────────────────────┘")

	// --- 7.6 MMR 去重 ---
	divider("7.6 MMR 去重检索 (多样性)")
	fmt.Println("  标准检索 vs MMR 检索:\n")

	// 标准检索
	fmt.Println("  标准 Top-5:")
	standardResults := retrieve(userQuery, 5)
	for i, r := range standardResults {
		fmt.Printf("    #%d [%s] score=%.4f\n", i+1, r.Source, r.Score)
	}

	// MMR: lambda=0.5 平衡相关性和多样性
	fmt.Println("\n  MMR (λ=0.5) Top-5:")
	mmrResults := mmrRetrieve(conn, tfidf, userQuery, 5, 0.5)
	for i, r := range mmrResults {
		fmt.Printf("    #%d [%s] mmr_score=%.4f\n", i+1, r.Source, r.Score)
	}

	// --- 7.7 知识库统计 ---
	divider("7.7 知识库统计")
	stats := []struct {
		sql  string
		desc string
	}{
		{"SELECT count(*) FROM knowledge_base", "总分块数"},
		{"SELECT count(DISTINCT source) FROM knowledge_base", "文档数"},
		{"SELECT avg(token_count) FROM knowledge_base", "平均 token 数"},
		{"SELECT max(token_count) - min(token_count) FROM knowledge_base", "token 数范围"},
	}
	for _, s := range stats {
		var val float64
		err = conn.QueryRow(ctx, s.sql).Scan(&val)
		check(err)
		if s.desc == "平均 token 数" {
			fmt.Printf("  %-12s: %.1f\n", s.desc, val)
		} else {
			fmt.Printf("  %-12s: %.0f\n", s.desc, val)
		}
	}

	_, _ = conn.Exec(ctx, "DROP TABLE IF EXISTS knowledge_base")
}

// splitSentences 按句号/换行分割
func splitSentences(text string) []string {
	seps := []string{"。", "\n"}
	var sentences []string
	parts := []string{text}
	for _, sep := range seps {
		var newParts []string
		for _, p := range parts {
			newParts = append(newParts, strings.Split(p, sep)...)
		}
		parts = newParts
	}
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if len(s) > 0 {
			sentences = append(sentences, s)
		}
	}
	return sentences
}

type mmrResult struct {
	Source string
	Score  float64
}

// mmrRetrieve 最大边际相关性检索 (简化实现)
func mmrRetrieve(conn *pgx.Conn, tfidf *TFIDFVectorizer, query string, topK int, lambda float64) []mmrResult {
	ctx := context.Background()
	// 1. 获取候选集 (3x topK)
	candidatePool := topK * 3
	qvec := pgvector.NewVector(tfidf.Transform(query))

	rows, err := conn.Query(ctx,
		`SELECT id, source, content, 1 - (embedding <=> $1) AS score
		 FROM knowledge_base ORDER BY embedding <=> $1 LIMIT $2`,
		qvec, candidatePool)
	check(err)

	type candidate struct {
		id      int64
		source  string
		content string
		score   float64
		vec     []float32
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		err = rows.Scan(&c.id, &c.source, &c.content, &c.score)
		check(err)
		c.vec = tfidf.Transform(c.content)
		candidates = append(candidates, c)
	}
	rows.Close()

	// 2. 贪心 MMR 选择
	selected := make(map[int]bool)
	var results []mmrResult

	for len(results) < topK && len(results) < len(candidates) {
		bestIdx := -1
		bestScore := -1.0

		for i, c := range candidates {
			if selected[i] {
				continue
			}
			// 与已选文档的最大相似度
			maxSim := 0.0
			for j := range selected {
				sim := cosineSim(c.vec, candidates[j].vec)
				if sim > maxSim {
					maxSim = sim
				}
			}
			// MMR 分数
			mmrScore := lambda*c.score - (1-lambda)*maxSim
			if mmrScore > bestScore {
				bestScore = mmrScore
				bestIdx = i
			}
		}

		if bestIdx >= 0 {
			selected[bestIdx] = true
			results = append(results, mmrResult{
				Source: candidates[bestIdx].source,
				Score:  bestScore,
			})
		}
	}

	return results
}
