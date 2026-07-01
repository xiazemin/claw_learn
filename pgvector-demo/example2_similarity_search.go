package main

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

type testVec struct {
	label string
	vec   []float32
}

// Example 2: 相似度搜索 — L2距离、余弦距离、内积距离三种度量方式对比
func example2_similarity_search() {
	divider("Example 2: 相似度搜索")
	ctx := context.Background()

	conn, err := connect(ctx)
	check(err)
	defer conn.Close(ctx)

	// 准备：创建专用表
	_, err = conn.Exec(ctx, "DROP TABLE IF EXISTS vectors_demo")
	check(err)
	_, err = conn.Exec(ctx, `
		CREATE TABLE vectors_demo (
			id   bigserial PRIMARY KEY,
			标签 text,
			向量 vector(4)
		)
	`)
	check(err)

	// 插入已知向量，方便理解距离计算
	testData := []testVec{
		{label: "A: [1,0,0,0]", vec: []float32{1, 0, 0, 0}},
		{label: "B: [0,1,0,0]", vec: []float32{0, 1, 0, 0}},
		{label: "C: [1,1,0,0]", vec: []float32{1, 1, 0, 0}},
		{label: "D: [1,1,1,0]", vec: []float32{1, 1, 1, 0}},
		{label: "E: [1,1,1,1]", vec: []float32{1, 1, 1, 1}},
		{label: "F: [0.5,0.5,0,0]", vec: []float32{0.5, 0.5, 0, 0}},
	}
	for _, d := range testData {
		_, err = conn.Exec(ctx, "INSERT INTO vectors_demo (标签, 向量) VALUES ($1, $2)",
			d.label, pgvector.NewVector(d.vec))
		check(err)
	}
	fmt.Printf("✅ 插入 %d 条测试向量\n", len(testData))

	queryVec := []float32{1, 1, 0, 0}

	// --- 2.1 L2 欧氏距离 ---
	divider("2.1 L2 欧氏距离 (<->)")
	fmt.Printf("查询向量: %v\n", queryVec)
	fmt.Println("L2 距离 = √Σ(a_i - b_i)², 范围 [0, +∞)\n")
	printQuery(ctx, conn, queryVec,
		`SELECT 标签, 向量 <-> $1 AS dist FROM vectors_demo ORDER BY 向量 <-> $1`)

	// --- 2.2 余弦距离 ---
	divider("2.2 余弦距离 (<=>)")
	fmt.Printf("查询向量: %v\n", queryVec)
	fmt.Println("余弦距离 = 1 - cos(θ), 范围 [0, 2]. 值越小越相近\n")
	printQuery(ctx, conn, queryVec,
		`SELECT 标签, 向量 <=> $1 AS dist FROM vectors_demo ORDER BY 向量 <=> $1`)

	// --- 2.3 内积距离 ---
	divider("2.3 内积距离 (<#>)")
	fmt.Printf("查询向量: %v\n", queryVec)
	fmt.Println("负内积: 值越小 = 内积越大 = 越相似\n")
	printQuery(ctx, conn, queryVec,
		`SELECT 标签, 向量 <#> $1 AS dist FROM vectors_demo ORDER BY 向量 <#> $1`)

	// --- 2.4 手算验证 ---
	divider("2.4 手算验证")
	fmt.Println("查询向量 Q = [1,1,0,0]\n")
	for _, h := range []testVec{
		{label: "A", vec: []float32{1, 0, 0, 0}},
		{label: "B", vec: []float32{0, 1, 0, 0}},
		{label: "C", vec: []float32{1, 1, 0, 0}},
	} {
		fmt.Printf("  %s %v → L2=%.4f, 余弦距离=%.4f, 内积=%.1f\n",
			h.label, h.vec,
			euclideanL2(queryVec, h.vec),
			1-cosineSim(queryVec, h.vec),
			innerProduct(queryVec, h.vec))
	}

	// --- 2.5 Top-K ---
	divider("2.5 Top-K 搜索")
	k := 3
	fmt.Printf("查询: %v, Top-%d:\n\n", queryVec, k)
	topK(ctx, conn, queryVec, k)
}

func printQuery(ctx context.Context, conn *pgx.Conn, query []float32, sql string) {
	rows, err := conn.Query(ctx, sql, pgvector.NewVector(query))
	check(err)
	defer rows.Close()

	i := 1
	for rows.Next() {
		var label string
		var dist float64
		err = rows.Scan(&label, &dist)
		check(err)
		fmt.Printf("  #%d %-20s %.6f\n", i, label, dist)
		i++
	}
	fmt.Println()
}

func topK(ctx context.Context, conn *pgx.Conn, query []float32, k int) {
	rows, err := conn.Query(ctx,
		`SELECT 标签, 向量 <-> $1 AS dist FROM vectors_demo ORDER BY 向量 <-> $1 LIMIT $2`,
		pgvector.NewVector(query), k)
	check(err)
	defer rows.Close()

	i := 1
	for rows.Next() {
		var label string
		var dist float64
		err = rows.Scan(&label, &dist)
		check(err)
		fmt.Printf("  #%d %s (L2=%.4f)\n", i, label, dist)
		i++
	}
	fmt.Println()
}

func euclideanL2(a, b []float32) float64 {
	var sum float64
	for i := range a {
		d := float64(a[i] - b[i])
		sum += d * d
	}
	return math.Sqrt(sum)
}

func cosineSim(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func innerProduct(a, b []float32) float64 {
	var sum float64
	for i := range a {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}
