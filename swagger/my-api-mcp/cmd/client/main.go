package main

import (
	"context"
	"fmt"
	"log"

	"my-api-mcp/gen"
	"my-api-mcp/internal"
)

func main() {
	// 方式一：直接使用服务实现（本地调用）
	fmt.Println("=== 本地调用 ===")
	svc := internal.NewBookService()

	ctx := context.Background()

	books, err := svc.ListBooks(ctx, gen.ListBooksParams{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("找到 %d 本书\n", len(books))
	for _, book := range books {
		fmt.Printf("  - %s by %s", book.Title, book.Author)
		if book.Price != nil {
			fmt.Printf(" ($%.2f)", *book.Price)
		}
		fmt.Println()
	}

	// 方式二：HTTP 客户端调用（需要启动 HTTP 服务）
	fmt.Println("\n=== HTTP 客户端调用 ===")
	client, err := gen.NewClientWithResponses("http://localhost:8080/api/v1")
	if err != nil {
		log.Fatal(err)
	}

	// 获取所有图书
	resp, err := client.ListBooksWithResponse(ctx, &gen.ListBooksParams{})
	if err != nil {
		log.Fatal(err)
	}
	if resp.JSON200 != nil {
		fmt.Printf("状态码: %d, 图书数量: %d\n",
			resp.HTTPResponse.StatusCode, len(*resp.JSON200))
		for _, book := range *resp.JSON200 {
			fmt.Printf("  - %s\n", book.Title)
		}
	}

	// 创建新书
	price := float32(59.99)
	newBook := gen.CreateBookRequest{
		Title:  "Design Patterns",
		Author: "Gang of Four",
		Genre:  stringPtr("Software Engineering"),
		Price:  &price,
	}

	createResp, err := client.CreateBookWithResponse(ctx, newBook)
	if err != nil {
		log.Fatal(err)
	}
	if createResp.JSON201 != nil {
		fmt.Printf("创建成功: ID=%d, Title=%s\n",
			createResp.JSON201.Id, createResp.JSON201.Title)
	}

	// 搜索图书
	searchResp, err := client.SearchBooksWithResponse(ctx, &gen.SearchBooksParams{
		Q: "Go",
	})
	if err != nil {
		log.Fatal(err)
	}
	if searchResp.JSON200 != nil {
		fmt.Printf("搜索 'Go' 找到 %d 个结果\n", *searchResp.JSON200.Total)
		// 修复：解引用 *[]gen.Book
		for _, book := range *searchResp.JSON200.Results {
			fmt.Printf("  - %s\n", book.Title)
		}
	}

	// 获取单本书
	getResp, err := client.GetBookWithResponse(ctx, 1)
	if err != nil {
		log.Fatal(err)
	}
	if getResp.JSON200 != nil {
		fmt.Printf("\n图书详情: %s", getResp.JSON200.Title)
		if getResp.JSON200.Price != nil {
			fmt.Printf(" ($%.2f)", *getResp.JSON200.Price)
		}
		fmt.Println()
	}

	// 删除图书
	delResp, err := client.DeleteBookWithResponse(ctx, 2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("删除状态: %d\n", delResp.HTTPResponse.StatusCode)
}

func stringPtr(s string) *string { return &s }
