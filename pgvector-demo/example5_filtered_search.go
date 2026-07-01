package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

// Example 5: 过滤搜索 — 向量搜索 + WHERE 元数据过滤、混合搜索
func example5_filtered_search() {
	divider("Example 5: 过滤搜索")
	ctx := context.Background()

	conn, err := connect(ctx)
	check(err)
	defer conn.Close(ctx)

	// --- 5.1 创建带元数据的表 ---
	divider("5.1 创建文档表 (含元数据)")
	_, err = conn.Exec(ctx, "DROP TABLE IF EXISTS docs")
	check(err)
	_, err = conn.Exec(ctx, `
		CREATE TABLE docs (
			id         bigserial PRIMARY KEY,
			title      text NOT NULL,
			content    text,
			language   text DEFAULT 'zh',
			category   text DEFAULT 'general',
			year       int DEFAULT 2024,
			embedding  vector(32)
		)
	`)
	check(err)

	// 插入多语言、多年份、多分类的文档
	docs := []struct {
		title, lang, cat string
		year             int
		vec              []float32
	}{
		{"Go并发编程", "zh", "tech", 2024, randomFloat32Seed(32, 1)},
		{"Rust入门指南", "zh", "tech", 2024, randomFloat32Seed(32, 2)},
		{"Python机器学习", "zh", "tech", 2023, randomFloat32Seed(32, 3)},
		{"GoConcurrency", "en", "tech", 2024, randomFloat32Seed(32, 4)},
		{"PostgreSQL优化", "zh", "tech", 2023, randomFloat32Seed(32, 5)},
		{"机器学习实战", "zh", "science", 2024, randomFloat32Seed(32, 6)},
		{"数据可视化", "zh", "art", 2024, randomFloat32Seed(32, 7)},
		{"2024年技术趋势", "zh", "general", 2024, randomFloat32Seed(32, 8)},
		{"GoWeb开发", "zh", "tech", 2023, randomFloat32Seed(32, 9)},
		{"DeepLearning", "en", "science", 2024, randomFloat32Seed(32, 10)},
		{"量子计算入门", "zh", "science", 2023, randomFloat32Seed(32, 11)},
		{"AI艺术创作", "zh", "art", 2024, randomFloat32Seed(32, 12)},
		{"GraphQL实战", "en", "tech", 2023, randomFloat32Seed(32, 13)},
		{"微服务架构", "zh", "tech", 2024, randomFloat32Seed(32, 14)},
		{"区块链原理", "zh", "tech", 2023, randomFloat32Seed(32, 15)},
	}
	for _, d := range docs {
		_, err = conn.Exec(ctx,
			`INSERT INTO docs (title, language, category, year, embedding) VALUES ($1,$2,$3,$4,$5)`,
			d.title, d.lang, d.cat, d.year, pgvector.NewVector(d.vec))
		check(err)
	}
	fmt.Printf("✅ 插入 %d 条多语言、多分类文档\n", len(docs))

	// 建 HNSW 索引
	_, err = conn.Exec(ctx, `CREATE INDEX ON docs USING hnsw (embedding vector_l2_ops)`)
	check(err)
	fmt.Println("✅ HNSW 索引已创建\n")

	queryVec := pgvector.NewVector(randomFloat32Seed(32, 2))

	// --- 5.2 无过滤 (基线) ---
	divider("5.2 无过滤 (基线 Top-5)")
	printDocResults(ctx, conn, queryVec,
		`SELECT title, language, category, year, embedding <-> $1 AS dist
		 FROM docs ORDER BY embedding <-> $1 LIMIT 5`)

	// --- 5.3 语言过滤 ---
	divider("5.3 仅中文文档 (language='zh')")
	printDocResults(ctx, conn, queryVec,
		`SELECT title, language, category, year, embedding <-> $1 AS dist
		 FROM docs WHERE language = 'zh'
		 ORDER BY embedding <-> $1 LIMIT 5`)

	// --- 5.4 分类过滤 ---
	divider("5.4 仅科技分类 (category='tech')")
	printDocResults(ctx, conn, queryVec,
		`SELECT title, language, category, year, embedding <-> $1 AS dist
		 FROM docs WHERE category = 'tech'
		 ORDER BY embedding <-> $1 LIMIT 5`)

	// --- 5.5 多条件过滤 ---
	divider("5.5 中文 + 科技 + 2024年")
	printDocResults(ctx, conn, queryVec,
		`SELECT title, language, category, year, embedding <-> $1 AS dist
		 FROM docs WHERE language = 'zh' AND category = 'tech' AND year = 2024
		 ORDER BY embedding <-> $1 LIMIT 5`)

	// --- 5.6 范围过滤 ---
	divider("5.6 距离阈值过滤 (dist < 2.0)")
	start := time.Now()
	rows, err := conn.Query(ctx,
		`SELECT title, embedding <-> $1 AS dist
		 FROM docs WHERE embedding <-> $1 < 2.0
		 ORDER BY embedding <-> $1`, queryVec)
	check(err)
	count := 0
	for rows.Next() {
		var title string
		var dist float64
		rows.Scan(&title, &dist)
		fmt.Printf("  %-20s dist=%.4f\n", title, dist)
		count++
	}
	rows.Close()
	fmt.Printf("\n  在阈值 2.0 内找到 %d 条, 耗时: %v\n", count, time.Since(start).Truncate(time.Microsecond))

	// --- 5.7 带重排序的混合查询 ---
	divider("5.7 分类聚合计数")
	rows, err = conn.Query(ctx,
		`SELECT category, count(*), min(embedding <-> $1) AS min_dist
		 FROM docs WHERE language = 'zh'
		 GROUP BY category ORDER BY min_dist`, queryVec)
	check(err)
	defer rows.Close()
	for rows.Next() {
		var cat string
		var cnt int
		var minDist float64
		rows.Scan(&cat, &cnt, &minDist)
		fmt.Printf("  %-12s %d篇 最近距离=%.4f\n", cat, cnt, minDist)
	}

	_, _ = conn.Exec(ctx, "DROP TABLE IF EXISTS docs")
}

func printDocResults(ctx context.Context, conn *pgx.Conn, query pgvector.Vector, sql string) {
	start := time.Now()
	rows, err := conn.Query(ctx, sql, query)
	check(err)
	defer rows.Close()
	for rows.Next() {
		var title, lang, cat string
		var year int
		var dist float64
		err = rows.Scan(&title, &lang, &cat, &year, &dist)
		check(err)
		fmt.Printf("  %-20s [%s/%s/%d] dist=%.4f\n", title, lang, cat, year, dist)
	}
	fmt.Printf("  ⏱ %v\n\n", time.Since(start).Truncate(time.Microsecond))
}
