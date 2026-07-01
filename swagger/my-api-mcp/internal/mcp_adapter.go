package internal

import (
	"context"
	"encoding/json"
	"fmt"

	"my-api-mcp/gen"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type MCPAdapter struct {
	service *BookService
}

func NewMCPAdapter(svc *BookService) *MCPAdapter {
	return &MCPAdapter{service: svc}
}

func getArgs(args any) map[string]interface{} {
	m, ok := args.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}
	return m
}

func getString(args any, key string) string {
	m := getArgs(args)
	s, _ := m[key].(string)
	return s
}

func getFloat64(args any, key string) (float64, bool) {
	m := getArgs(args)
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}

func (a *MCPAdapter) CreateMCPServer() *server.MCPServer {
	s := server.NewMCPServer(
		"book-management-api",
		"1.0.0",
		server.WithLogging(),
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
	)

	a.registerListBooksTool(s)
	a.registerCreateBookTool(s)
	a.registerGetBookTool(s)
	a.registerSearchBooksTool(s)
	a.registerDeleteBookTool(s)
	a.registerBooksResource(s)

	return s
}

func (a *MCPAdapter) registerListBooksTool(s *server.MCPServer) {
	tool := mcp.NewTool("list_books",
		mcp.WithDescription("获取所有图书，可按作者和类型筛选"),
		mcp.WithString("author", mcp.Description("按作者筛选")),
		mcp.WithString("genre", mcp.Description("按类型筛选")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params := gen.ListBooksParams{}

		if author := getString(request.Params.Arguments, "author"); author != "" {
			params.Author = &author
		}
		if genre := getString(request.Params.Arguments, "genre"); genre != "" {
			params.Genre = &genre
		}

		books, err := a.service.ListBooks(ctx, params)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		jsonData, _ := json.MarshalIndent(books, "", "  ")
		return mcp.NewToolResultText(string(jsonData)), nil
	})
}

func (a *MCPAdapter) registerCreateBookTool(s *server.MCPServer) {
	tool := mcp.NewTool("create_book",
		mcp.WithDescription("添加新书到图书馆"),
		mcp.WithString("title", mcp.Required(), mcp.Description("书名")),
		mcp.WithString("author", mcp.Required(), mcp.Description("作者")),
		mcp.WithString("genre", mcp.Description("类型")),
		mcp.WithString("isbn", mcp.Description("ISBN 编号")),
		mcp.WithNumber("price", mcp.Description("价格")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title := getString(request.Params.Arguments, "title")
		if title == "" {
			return mcp.NewToolResultError("title 参数缺失"), nil
		}

		author := getString(request.Params.Arguments, "author")
		if author == "" {
			return mcp.NewToolResultError("author 参数缺失"), nil
		}

		req := gen.CreateBookRequest{
			Title:  title,
			Author: author,
		}

		if genre := getString(request.Params.Arguments, "genre"); genre != "" {
			req.Genre = &genre
		}
		if isbn := getString(request.Params.Arguments, "isbn"); isbn != "" {
			req.Isbn = &isbn
		}
		if price, ok := getFloat64(request.Params.Arguments, "price"); ok {
			p := float32(price)
			req.Price = &p
		}

		book, err := a.service.CreateBook(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		jsonData, _ := json.MarshalIndent(book, "", "  ")
		return mcp.NewToolResultText(string(jsonData)), nil
	})
}

func (a *MCPAdapter) registerGetBookTool(s *server.MCPServer) {
	tool := mcp.NewTool("get_book",
		mcp.WithDescription("根据 ID 获取图书详情"),
		mcp.WithNumber("book_id", mcp.Required(), mcp.Description("图书 ID")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		bookID, ok := getFloat64(request.Params.Arguments, "book_id")
		if !ok {
			return mcp.NewToolResultError("book_id 参数缺失"), nil
		}

		book, err := a.service.GetBook(ctx, int64(bookID))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		jsonData, _ := json.MarshalIndent(book, "", "  ")
		return mcp.NewToolResultText(string(jsonData)), nil
	})
}

func (a *MCPAdapter) registerSearchBooksTool(s *server.MCPServer) {
	tool := mcp.NewTool("search_books",
		mcp.WithDescription("搜索图书"),
		mcp.WithString("query", mcp.Required(), mcp.Description("搜索关键词")),
		mcp.WithNumber("limit", mcp.Description("结果数量限制")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := getString(request.Params.Arguments, "query")
		if query == "" {
			return mcp.NewToolResultError("query 参数缺失"), nil
		}

		params := gen.SearchBooksParams{Q: query}
		if limit, ok := getFloat64(request.Params.Arguments, "limit"); ok {
			l := int(limit)
			params.Limit = &l
		}

		books, total, err := a.service.SearchBooks(ctx, params)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result := map[string]interface{}{
			"total":   total,
			"results": books,
		}
		jsonData, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(jsonData)), nil
	})
}

func (a *MCPAdapter) registerDeleteBookTool(s *server.MCPServer) {
	tool := mcp.NewTool("delete_book",
		mcp.WithDescription("删除指定图书"),
		mcp.WithNumber("book_id", mcp.Required(), mcp.Description("图书 ID")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		bookID, ok := getFloat64(request.Params.Arguments, "book_id")
		if !ok {
			return mcp.NewToolResultError("book_id 参数缺失"), nil
		}

		if err := a.service.DeleteBook(ctx, int64(bookID)); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("图书 %d 已删除", int64(bookID))), nil
	})
}

func (a *MCPAdapter) registerBooksResource(s *server.MCPServer) {
	resource := mcp.NewResource(
		"books://list",
		"图书列表",
		mcp.WithMIMEType("application/json"),
	)

	s.AddResource(resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		books, err := a.service.ListBooks(ctx, gen.ListBooksParams{})
		if err != nil {
			return nil, err
		}

		jsonData, _ := json.MarshalIndent(books, "", "  ")
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "books://list",
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		}, nil
	})
}
