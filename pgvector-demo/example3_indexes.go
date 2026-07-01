package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

// Example 3: 向量索引 — HNSW vs IVFFlat 的创建、性能对比和使用场景
func example3_indexes() {
	divider("Example 3: 向量索引 (HNSW vs IVFFlat)")
	ctx := context.Background()

	conn, err := connect(ctx)
	check(err)
	defer conn.Close(ctx)

	// --- 3.1 创建测试数据 ---
	divider("3.1 准备数据 (10000 条 128 维向量)")
	_, err = conn.Exec(ctx, "DROP TABLE IF EXISTS idx_test")
	check(err)
	_, err = conn.Exec(ctx, `
		CREATE TABLE idx_test (
			id   bigserial PRIMARY KEY,
			data text,
			向量 vector(128)
		)
	`)
	check(err)

	const N = 10000
	// 逐条插入（简化演示，生产中应使用 COPY + pgx.CopyFromSlice）
	start := time.Now()
	for i := 0; i < N; i++ {
		vec := pgvector.NewVector(randomFloat32Seed(128, uint32(i)))
		_, err = conn.Exec(ctx,
			"INSERT INTO idx_test (data, 向量) VALUES ($1, $2)",
			fmt.Sprintf("document_%d", i), vec)
		check(err)
	}
	fmt.Printf("✅ 插入 %d 条记录, 耗时: %v\n\n", N, time.Since(start).Truncate(time.Millisecond))

	queryVec := pgvector.NewVector(randomFloat32Seed(128, 9999))

	// --- 3.2 无索引 (暴力搜索) ---
	divider("3.2 无索引 (暴力扫描)")
	start = time.Now()
	for i := 0; i < 10; i++ {
		_, err = conn.Exec(ctx,
			"SELECT id FROM idx_test ORDER BY 向量 <-> $1 LIMIT 5", queryVec)
		check(err)
	}
	elapsed := time.Since(start) / 10
	fmt.Printf("  10 次查询平均耗时: %v\n\n", elapsed)

	// --- 3.3 HNSW 索引 ---
	divider("3.3 创建 HNSW 索引")
	fmt.Println("  HNSW: 分层可导航小世界图, 查询快但构建慢")
	fmt.Println("  参数: m=16 (每层连接数), ef_construction=64 (构建搜索宽度)\n")

	start = time.Now()
	_, err = conn.Exec(ctx,
		`CREATE INDEX idx_hnsw ON idx_test USING hnsw (向量 vector_l2_ops) WITH (m = 16, ef_construction = 64)`)
	check(err)
	fmt.Printf("  HNSW 索引构建耗时: %v\n", time.Since(start).Truncate(time.Millisecond))

	// 查询性能
	start = time.Now()
	for i := 0; i < 10; i++ {
		_, err = conn.Exec(ctx,
			"SELECT id FROM idx_test ORDER BY 向量 <-> $1 LIMIT 5", queryVec)
		check(err)
	}
	elapsed = time.Since(start) / 10
	fmt.Printf("  10 次查询平均耗时: %v (加速 %.1fx)\n\n",
		elapsed, float64(time.Since(start))/float64(elapsed))

	// ef_search 参数
	fmt.Println("  调节 ef_search 控制查询精度/速度平衡:")
	for _, ef := range []int{10, 50, 100} {
		start = time.Now()
		for i := 0; i < 5; i++ {
			_, err = conn.Exec(ctx,
				fmt.Sprintf("SET LOCAL hnsw.ef_search = %d", ef))
			check(err)
			_, err = conn.Exec(ctx,
				"SELECT id FROM idx_test ORDER BY 向量 <-> $1 LIMIT 5", queryVec)
			check(err)
		}
		elapsed = time.Since(start) / 5
		fmt.Printf("    ef_search=%-3d → %v\n", ef, elapsed.Truncate(time.Microsecond))
	}
	fmt.Println()

	// 保存 HNSW 结果用于对比
	hnswResults := searchTopK(ctx, conn, queryVec, 5)

	// --- 3.4 删除 HNSW, 创建 IVFFlat ---
	divider("3.4 创建 IVFFlat 索引")
	fmt.Println("  IVFFlat: 倒排文件索引, 构建快但查询略慢")
	fmt.Println("  参数: lists=100 (聚类中心数, 通常 N/1000)\n")

	_, err = conn.Exec(ctx, "DROP INDEX IF EXISTS idx_hnsw")
	check(err)
	start = time.Now()
	_, err = conn.Exec(ctx,
		`CREATE INDEX idx_ivfflat ON idx_test USING ivfflat (向量 vector_l2_ops) WITH (lists = 100)`)
	check(err)
	fmt.Printf("  IVFFlat 索引构建耗时: %v\n", time.Since(start).Truncate(time.Millisecond))

	// 查询性能
	start = time.Now()
	for i := 0; i < 10; i++ {
		_, err = conn.Exec(ctx,
			"SELECT id FROM idx_test ORDER BY 向量 <-> $1 LIMIT 5", queryVec)
		check(err)
	}
	elapsed = time.Since(start) / 10
	fmt.Printf("  10 次查询平均耗时: %v\n\n", elapsed)

	ivfflatResults := searchTopK(ctx, conn, queryVec, 5)

	// --- 3.5 对比 ---
	divider("3.5 结果对比")
	fmt.Println("  HNSW 结果:")
	for i, r := range hnswResults {
		fmt.Printf("    #%d id=%d dist=%.4f\n", i+1, r.id, r.dist)
	}
	fmt.Println("  IVFFlat 结果:")
	for i, r := range ivfflatResults {
		fmt.Printf("    #%d id=%d dist=%.4f\n", i+1, r.id, r.dist)
	}

	// --- 3.6 三种距离类型的索引 ---
	divider("3.6 不同距离类型的索引")
	fmt.Println("  HNSW + 余弦距离:")
	_, err = conn.Exec(ctx, "DROP INDEX IF EXISTS idx_ivfflat")
	check(err)
	_, err = conn.Exec(ctx,
		`CREATE INDEX idx_hnsw_cos ON idx_test USING hnsw (向量 vector_cosine_ops)`)
	check(err)
	fmt.Println("    CREATE INDEX idx_hnsw_cos ON ... USING hnsw (向量 vector_cosine_ops)")

	fmt.Println("\n  HNSW + 内积:")
	_, err = conn.Exec(ctx, "DROP INDEX IF EXISTS idx_hnsw_cos")
	check(err)
	_, err = conn.Exec(ctx,
		`CREATE INDEX idx_hnsw_ip ON idx_test USING hnsw (向量 vector_ip_ops)`)
	check(err)
	fmt.Println("    CREATE INDEX idx_hnsw_ip ON ... USING hnsw (向量 vector_ip_ops)")

	// --- 3.7 选择指南 ---
	divider("3.7 选择指南")
	fmt.Println("  ┌──────────┬──────────────────┬──────────────────┬──────────────────────┐")
	fmt.Println("  │          │ HNSW             │ IVFFlat          │ 暴力搜索             │")
	fmt.Println("  ├──────────┼──────────────────┼──────────────────┼──────────────────────┤")
	fmt.Println("  │ 构建速度 │ 慢               │ 快               │ -                    │")
	fmt.Println("  │ 查询速度 │ 快               │ 中               │ 慢                   │")
	fmt.Println("  │ 精度     │ 高               │ 中               │ 精确                 │")
	fmt.Println("  │ 内存     │ 高               │ 低               │ 低                   │")
	fmt.Println("  │ 数据量   │ >1000条推荐      │ >10000条推荐     │ <1000条              │")
	fmt.Println("  │ 动态增删 │ 支持             │ 需重建           │ 自动                 │")
	fmt.Println("  └──────────┴──────────────────┴──────────────────┴──────────────────────┘")

	_, _ = conn.Exec(ctx, "DROP TABLE IF EXISTS idx_test")
}

type idxResult struct {
	id   int64
	dist float64
}

func searchTopK(ctx context.Context, conn *pgx.Conn, query pgvector.Vector, k int) []idxResult {
	rows, err := conn.Query(ctx,
		"SELECT id, 向量 <-> $1 AS dist FROM idx_test ORDER BY 向量 <-> $1 LIMIT $2",
		query, k)
	check(err)
	defer rows.Close()

	var results []idxResult
	for rows.Next() {
		var r idxResult
		err = rows.Scan(&r.id, &r.dist)
		check(err)
		results = append(results, r)
	}
	return results
}

func randomFloat32Seed(n int, seed uint32) []float32 {
	vec := make([]float32, n)
	for i := range vec {
		seed = seed*1103515245 + 12345
		vec[i] = float32(int32(seed&0x7FFFFFFF)) / float32(0x7FFFFFFF)
	}
	return vec
}
