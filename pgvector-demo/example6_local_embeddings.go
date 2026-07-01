package main

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/pgvector/pgvector-go"
)

// Example 6: 本地嵌入 — 不依赖外部API的简单嵌入函数 (TF-IDF + 哈希投影)
// 目的: 完整演示 embedding 的流程, 不需要 OpenAI 等 API key

// --- 6.1 简易词袋模型 ---
func tokenize(text string) []string {
	// 简单分词：按空格和标点分割，转小写
	text = strings.ToLower(text)
	for _, p := range []string{",", ".", "!", "?", "，", "。", "！", "？", "、", "：", "；", "(", ")", "（", "）"} {
		text = strings.ReplaceAll(text, p, " ")
	}
	return strings.Fields(text)
}

// --- 6.2 TF-IDF 向量化 ---
type TFIDFVectorizer struct {
	dim     int
	vocab   map[string]int // word -> index
	idf     map[string]float64
	fitted  bool
}

func NewTFIDFVectorizer(dim int) *TFIDFVectorizer {
	return &TFIDFVectorizer{
		dim:   dim,
		vocab: make(map[string]int),
		idf:   make(map[string]float64),
	}
}

// Fit 从文档集合中学习词汇表和 IDF 权重
func (v *TFIDFVectorizer) Fit(documents []string) {
	// 收集词汇表
	docCount := len(documents)
	wordDocFreq := make(map[string]int)
	for _, doc := range documents {
		tokens := tokenize(doc)
		seen := make(map[string]bool)
		for _, t := range tokens {
			if !seen[t] {
				wordDocFreq[t]++
				seen[t] = true
			}
		}
	}

	// 按频率排序取 top-dim 词汇
	type wordFreq struct {
		word string
		freq int
	}
	var sorted []wordFreq
	for w, f := range wordDocFreq {
		sorted = append(sorted, wordFreq{w, f})
	}
	// 简单排序
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].freq > sorted[i].freq {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
		if i >= v.dim-1 {
			break
		}
	}

	for i, wf := range sorted {
		v.vocab[wf.word] = i
		v.idf[wf.word] = math.Log(float64(docCount) / float64(wf.freq))
	}
	v.fitted = true
}

// Transform 将文档转换为 TF-IDF 向量
func (v *TFIDFVectorizer) Transform(doc string) []float32 {
	tokens := tokenize(doc)
	tf := make(map[string]int)
	for _, t := range tokens {
		tf[t]++
	}

	vec := make([]float32, v.dim)
	for word, idx := range v.vocab {
		if idx >= v.dim {
			continue
		}
		tfVal := float64(tf[word]) / float64(len(tokens))
		vec[idx] = float32(tfVal * v.idf[word])
	}

	// L2 归一化
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec
}

// FitTransform Fit + Transform
func (v *TFIDFVectorizer) FitTransform(documents []string) [][]float32 {
	v.Fit(documents)
	result := make([][]float32, len(documents))
	for i, doc := range documents {
		result[i] = v.Transform(doc)
	}
	return result
}

// --- 6.3 哈希投影 (SimHash) ---
func SimHash(text string, dim int) []float32 {
	tokens := tokenize(text)
	vec := make([]float32, dim)
	for _, token := range tokens {
		// 对每个 token 生成确定性伪随机向量
		h := fnv32(token)
		for i := 0; i < dim; i++ {
			h = (h*1103515245 + 12345 + uint32(i)*7919)
			if h&1 == 0 {
				vec[i] += 1.0
			} else {
				vec[i] -= 1.0
			}
		}
	}
	// L2 归一化
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec
}

func fnv32(s string) uint32 {
	h := uint32(2166136261)
	for _, c := range s {
		h ^= uint32(c)
		h *= 16777619
	}
	return h
}

// --- 主函数 ---
func example6_local_embeddings() {
	divider("Example 6: 本地嵌入 (无外部API)")
	ctx := context.Background()

	conn, err := connect(ctx)
	check(err)
	defer conn.Close(ctx)

	// 准备语料库
	corpus := []struct {
		title   string
		content string
	}{
		{"Go语言并发", "Go语言提供了goroutine和channel用于并发编程"},
		{"Python数据分析", "Python的pandas和numpy是数据分析的核心库"},
		{"PostgreSQL数据库", "PostgreSQL是功能强大的开源关系数据库"},
		{"机器学习入门", "机器学习通过数据训练模型来进行预测"},
		{"深度学习框架", "TensorFlow和PyTorch是主流的深度学习框架"},
		{"分布式系统", "分布式系统通过多节点协作处理大规模任务"},
		{"容器化部署", "Docker和Kubernetes是容器化部署的标准工具"},
		{"向量数据库", "向量数据库用于存储和检索高维向量数据"},
		{"RAG检索增强", "RAG通过检索外部知识来增强大模型的回答"},
		{"大语言模型", "GPT和LLaMA是当前最流行的大语言模型"},
	}

	fmt.Println("语料库:")
	for i, c := range corpus {
		fmt.Printf("  %2d. %s\n", i+1, c.title)
	}

	// --- 6.1 TF-IDF 向量化 ---
	divider("6.1 TF-IDF 向量化")
	docs := make([]string, len(corpus))
	for i, c := range corpus {
		docs[i] = c.title + " " + c.content
	}

	vecSize := 16
	tfidf := NewTFIDFVectorizer(vecSize)
	vectors := tfidf.FitTransform(docs)
	fmt.Printf("✅ TF-IDF 词汇表大小: %d, 向量维度: %d\n", len(tfidf.vocab), vecSize)
	fmt.Printf("   词汇表: %v\n\n", wordsOfVocab(tfidf.vocab))

	// 将向量存入 pgvector
	_, err = conn.Exec(ctx, "DROP TABLE IF EXISTS tfidf_docs")
	check(err)
	_, err = conn.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE tfidf_docs (
			id bigserial PRIMARY KEY,
			title text,
			content text,
			embedding vector(%d)
		)
	`, vecSize))
	check(err)

	for i, c := range corpus {
		vec := pgvector.NewVector(vectors[i])
		_, err = conn.Exec(ctx,
			"INSERT INTO tfidf_docs (title, content, embedding) VALUES ($1, $2, $3)",
			c.title, c.content, vec)
		check(err)
	}
	fmt.Printf("✅ %d 篇文档向量已存入 PostgreSQL\n", len(corpus))

	// 搜索
	divider("6.2 TF-IDF 语义搜索")
	queries := []string{
		"Go并发编程入门",
		"数据库优化",
		"AI模型训练",
		"容器技术",
	}
	for _, q := range queries {
		qvec := tfidf.Transform(q)
		fmt.Printf("  查询: %q\n", q)

		rows, err := conn.Query(ctx,
			`SELECT title, embedding <-> $1 AS dist
			 FROM tfidf_docs ORDER BY embedding <-> $1 LIMIT 3`,
			pgvector.NewVector(qvec))
		check(err)
		rank := 1
		for rows.Next() {
			var title string
			var dist float64
			rows.Scan(&title, &dist)
			fmt.Printf("    #%d %-15s (dist=%.4f)\n", rank, title, dist)
			rank++
		}
		rows.Close()
		fmt.Println()
	}

	// --- 6.3 SimHash ---
	divider("6.3 SimHash 向量化")
	_, err = conn.Exec(ctx, "DROP TABLE IF EXISTS simhash_docs")
	check(err)
	_, err = conn.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE simhash_docs (
			id bigserial PRIMARY KEY,
			title text,
			embedding vector(%d)
		)
	`, vecSize))
	check(err)

	for _, c := range corpus {
		text := c.title + " " + c.content
		vec := pgvector.NewVector(SimHash(text, vecSize))
		_, err = conn.Exec(ctx,
			"INSERT INTO simhash_docs (title, embedding) VALUES ($1, $2)",
			c.title, vec)
		check(err)
	}
	fmt.Printf("✅ SimHash %d 篇文档已存储\n", len(corpus))

	// SimHash 搜索
	divider("6.4 SimHash 搜索")
	for _, q := range []string{"编程语言特性", "AI人工智能"} {
		qvec := pgvector.NewVector(SimHash(q, vecSize))
		fmt.Printf("  查询: %q\n", q)
		rows, err := conn.Query(ctx,
			`SELECT title, embedding <=> $1 AS dist
			 FROM simhash_docs ORDER BY embedding <=> $1 LIMIT 3`,
			qvec)
		check(err)
		rank := 1
		for rows.Next() {
			var title string
			var dist float64
			rows.Scan(&title, &dist)
			fmt.Printf("    #%d %-15s (cos_dist=%.4f)\n", rank, title, dist)
			rank++
		}
		rows.Close()
		fmt.Println()
	}

	// --- 6.5 对比 TF-IDF vs SimHash ---
	divider("6.5 TF-IDF vs SimHash 对比")
	fmt.Println("  ┌────────────┬──────────────────────┬──────────────────────┐")
	fmt.Println("  │            │ TF-IDF               │ SimHash              │")
	fmt.Println("  ├────────────┼──────────────────────┼──────────────────────┤")
	fmt.Println("  │ 原理       │ 词频-逆文档频率      │ 哈希投影             │")
	fmt.Println("  │ 词序       │ 忽略                 │ 忽略                 │")
	fmt.Println("  │ 语义       │ 无(纯统计)           │ 弱(哈希碰撞近似)     │")
	fmt.Println("  │ 速度       │ 需训练               │ 即时                 │")
	fmt.Println("  │ 适用场景   │ 文档聚类/分类        │ 近似去重/相似检测     │")
	fmt.Println("  │ 建议       │ 小规模快速原型       │ 大规模近似搜索       │")
	fmt.Println("  └────────────┴──────────────────────┴──────────────────────┘")
	fmt.Println("\n  💡 生产环境建议使用 sentence-transformers 或 OpenAI Embeddings API")

	_, _ = conn.Exec(ctx, "DROP TABLE IF EXISTS tfidf_docs")
	_, _ = conn.Exec(ctx, "DROP TABLE IF EXISTS simhash_docs")
}

func wordsOfVocab(vocab map[string]int) []string {
	words := make([]string, 0, len(vocab))
	for w := range vocab {
		words = append(words, w)
	}
	if len(words) > 10 {
		words = words[:10]
		words = append(words, "...")
	}
	return words
}

