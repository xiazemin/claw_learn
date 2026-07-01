package main

import (
	"context"
	"fmt"

	"github.com/pgvector/pgvector-go"
)

// Example 1: 基础操作 — 连接、创建扩展、建表、插入向量、基本查询
func example1_basics() {
	divider("Example 1: 基础操作")
	ctx := context.Background()

	// 1. 连接数据库
	conn, err := connect(ctx)
	check(err)
	defer conn.Close(ctx)
	fmt.Println("✅ 连接成功")

	// 2. 创建 pgvector 扩展
	_, err = conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	check(err)
	fmt.Println("✅ pgvector 扩展已创建/确认存在")

	// 查看扩展版本
	var version string
	err = conn.QueryRow(ctx, "SELECT extversion FROM pg_extension WHERE extname = 'vector'").Scan(&version)
	check(err)
	fmt.Printf("   pgvector 版本: %s\n", version)

	// 3. 删除旧表（幂等执行）
	_, err = conn.Exec(ctx, "DROP TABLE IF EXISTS documents")
	check(err)

	// 4. 创建带向量列的表
	// embedding 列存储 384 维向量（常见于 sentence-transformers 模型）
	_, err = conn.Exec(ctx, `
		CREATE TABLE documents (
			id         bigserial PRIMARY KEY,
			title      text NOT NULL,
			content    text,
			embedding  vector(384),
			created_at timestamptz DEFAULT now()
		)
	`)
	check(err)
	fmt.Println("✅ 表 documents 已创建 (embedding: vector(384))")

	// 5. 插入单条记录
	vec := pgvector.NewVector(randomFloat32(384))
	_, err = conn.Exec(ctx,
		`INSERT INTO documents (title, content, embedding) VALUES ($1, $2, $3)`,
		"示例文档A", "这是第一个测试文档的内容", vec,
	)
	check(err)
	fmt.Println("✅ 插入 1 条记录")

	// 6. 插入多条记录
	sampleData := []struct {
		title   string
		content string
	}{
		{"Go语言入门", "Go是Google开发的静态强类型编程语言"},
		{"PostgreSQL指南", "PostgreSQL是先进的开源关系数据库"},
		{"向量数据库介绍", "向量数据库用于高效存储和检索高维向量"},
		{"机器学习基础", "机器学习是人工智能的一个分支"},
		{"深度学习原理", "深度学习使用多层神经网络进行特征提取"},
	}

	for _, d := range sampleData {
		v := pgvector.NewVector(randomFloat32(384))
		_, err = conn.Exec(ctx,
			`INSERT INTO documents (title, content, embedding) VALUES ($1, $2, $3)`,
			d.title, d.content, v,
		)
		check(err)
	}
	fmt.Printf("✅ 插入 %d 条样本记录\n", len(sampleData))

	// 7. 查询所有记录
	rows, err := conn.Query(ctx, "SELECT id, title, content FROM documents ORDER BY id")
	check(err)
	defer rows.Close()

	fmt.Println("\n   ┌────┬──────────────────┬─────────────────────────────────────┐")
	fmt.Println("   │ ID │ 标题             │ 内容摘要                            │")
	fmt.Println("   ├────┼──────────────────┼─────────────────────────────────────┤")
	for rows.Next() {
		var id int64
		var title, content string
		err = rows.Scan(&id, &title, &content)
		check(err)
		if len(content) > 33 {
			content = content[:30] + "..."
		}
		fmt.Printf("   │ %2d │ %-16s │ %-35s │\n", id, title, content)
	}
	fmt.Println("   └────┴──────────────────┴─────────────────────────────────────┘")

	// 8. 查看向量数据（截断显示）
	fmt.Println("\n   向量数据示例 (前5维):")
	rows2, err := conn.Query(ctx, "SELECT id, embedding FROM documents ORDER BY id LIMIT 3")
	check(err)
	defer rows2.Close()
	for rows2.Next() {
		var id int64
		var emb pgvector.Vector
		err = rows2.Scan(&id, &emb)
		check(err)
		s := emb.Slice()
		if len(s) > 5 {
			s = s[:5]
		}
		fmt.Printf("   Document %d: %v... (共384维)\n", id, s)
	}

	// 9. 统计
	var count int64
	err = conn.QueryRow(ctx, "SELECT count(*) FROM documents").Scan(&count)
	check(err)
	fmt.Printf("\n✅ 表中总记录数: %d\n", count)
}

// randomFloat32 生成指定长度的随机 float32 切片（用于演示）
func randomFloat32(n int) []float32 {
	vec := make([]float32, n)
	// 使用简单确定性种子以便可重复
	seed := uint32(42)
	for i := range vec {
		seed = seed*1103515245 + 12345
		vec[i] = float32(int32(seed&0x7FFFFFFF)) / float32(0x7FFFFFFF)
	}
	return vec
}
