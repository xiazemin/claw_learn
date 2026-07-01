package main

import (
	"fmt"
	"os"
)

func main() {
	examples := map[string]func(){
		"1": example1_basics,
		"2": example2_similarity_search,
		"3": example3_indexes,
		"4": example4_batch_operations,
		"5": example5_filtered_search,
		"6": example6_local_embeddings,
		"7": example7_rag_pipeline,
	}

	if len(os.Args) < 2 {
		fmt.Println("PostgreSQL pgvector 向量嵌入与搜索 - 完整示例集")
		fmt.Println("用法: go run . <example-number>")
		fmt.Println()
		fmt.Println("可用示例:")
		fmt.Println("  1 - 基础操作: 连接、建表、插入、查询")
		fmt.Println("  2 - 相似度搜索: L2距离、余弦距离、内积")
		fmt.Println("  3 - 向量索引: HNSW vs IVFFlat")
		fmt.Println("  4 - 批量操作: COPY批量插入、元数据列")
		fmt.Println("  5 - 过滤搜索: 向量搜索 + WHERE元数据过滤")
		fmt.Println("  6 - 本地嵌入: 无外部API的简单嵌入函数")
		fmt.Println("  7 - RAG管道: 文档→嵌入→存储→搜索→检索上下文")
		fmt.Println()
		fmt.Println("前置条件:")
		fmt.Println("  docker compose up -d          # 启动 pgvector")
		fmt.Println("  go run . 1                    # 运行指定示例")
		os.Exit(0)
	}

	fn, ok := examples[os.Args[1]]
	if !ok {
		fmt.Fprintf(os.Stderr, "未知示例: %s\n", os.Args[1])
		os.Exit(1)
	}
	fn()
}
