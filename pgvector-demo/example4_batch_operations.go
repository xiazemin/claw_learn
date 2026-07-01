package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

// Example 4: 批量操作 — COPY 高速批量插入、批量查询、性能基准
func example4_batch_operations() {
	divider("Example 4: 批量操作")
	ctx := context.Background()

	conn, err := connect(ctx)
	check(err)
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, "DROP TABLE IF EXISTS batch_test")
	check(err)
	_, err = conn.Exec(ctx, `
		CREATE TABLE batch_test (
			id       bigserial PRIMARY KEY,
			category text,
			embedding vector(64)
		)
	`)
	check(err)

	// --- 4.1 COPY 批量插入 ---
	divider("4.1 COPY 批量插入")
	const N = 5000
	categories := []string{"tech", "science", "art", "business", "sports"}

	start := time.Now()
	_, err = conn.CopyFrom(ctx,
		pgx.Identifier{"batch_test"},
		[]string{"category", "embedding"},
		pgx.CopyFromSlice(N, func(i int) ([]any, error) {
			cat := categories[i%len(categories)]
			vec := pgvector.NewVector(randomFloat32Seed(64, uint32(i)))
			return []any{cat, vec.String()}, nil
		}),
	)
	check(err)
	elapsed := time.Since(start)
	fmt.Printf("✅ COPY 插入 %d 条记录, 耗时: %v\n", N, elapsed.Truncate(time.Millisecond))
	fmt.Printf("   速率: %.0f 条/秒\n\n", float64(N)/elapsed.Seconds())

	// --- 4.2 批量查询 ---
	divider("4.2 批量查询 (Fetch All)")
	queryVec := pgvector.NewVector(randomFloat32Seed(64, 0))

	start = time.Now()
	rows, err := conn.Query(ctx,
		`SELECT id, category, embedding <-> $1 AS dist
		 FROM batch_test ORDER BY embedding <-> $1 LIMIT 100`, queryVec)
	check(err)

	type row struct {
		id       int64
		category string
		dist     float64
	}
	var results []row
	for rows.Next() {
		var r row
		err = rows.Scan(&r.id, &r.category, &r.dist)
		check(err)
		results = append(results, r)
	}
	rows.Close()
	elapsed = time.Since(start)

	fmt.Printf("✅ Top-100 查询完成, 耗时: %v\n", elapsed.Truncate(time.Microsecond))
	fmt.Println("\n   Top-10:")
	for i := 0; i < 10 && i < len(results); i++ {
		fmt.Printf("   #%2d id=%-5d category=%-8s dist=%.4f\n",
			i+1, results[i].id, results[i].category, results[i].dist)
	}

	// --- 4.3 分页查询 ---
	divider("4.3 分页查询")
	pageSize := 10
	for page := 0; page < 3; page++ {
		offset := page * pageSize
		start = time.Now()
		rows, err = conn.Query(ctx,
			`SELECT id, category, embedding <-> $1 AS dist
			 FROM batch_test ORDER BY embedding <-> $1
			 LIMIT $2 OFFSET $3`, queryVec, pageSize, offset)
		check(err)

		count := 0
		for rows.Next() {
			count++
			rows.Scan(new(int64), new(string), new(float64))
		}
		rows.Close()
		fmt.Printf("  Page %d (offset=%d): %d rows, %v\n",
			page+1, offset, count, time.Since(start).Truncate(time.Microsecond))
	}

	// --- 4.4 分类聚合 ---
	divider("4.4 分类统计")
	rows, err = conn.Query(ctx,
		`SELECT category, count(*) as cnt FROM batch_test GROUP BY category ORDER BY category`)
	check(err)
	defer rows.Close()
	for rows.Next() {
		var cat string
		var cnt int64
		err = rows.Scan(&cat, &cnt)
		check(err)
		fmt.Printf("  %-10s %d 条\n", cat, cnt)
	}

	// --- 4.5 每类 Top-K ---
	divider("4.5 每类 Top-K (Window Function)")
	fmt.Println("  每个分类中与查询向量最近的 2 条:")
	rows, err = conn.Query(ctx, `
		WITH ranked AS (
			SELECT id, category, embedding <-> $1 AS dist,
			       ROW_NUMBER() OVER (PARTITION BY category ORDER BY embedding <-> $1) AS rn
			FROM batch_test
		)
		SELECT id, category, dist FROM ranked WHERE rn <= 2 ORDER BY category, dist`, queryVec)
	check(err)
	defer rows.Close()
	for rows.Next() {
		var id int64
		var cat string
		var dist float64
		err = rows.Scan(&id, &cat, &dist)
		check(err)
		fmt.Printf("  %-10s id=%-5d dist=%.4f\n", cat, id, dist)
	}

	_, _ = conn.Exec(ctx, "DROP TABLE IF EXISTS batch_test")
}
