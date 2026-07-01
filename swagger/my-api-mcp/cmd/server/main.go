package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"my-api-mcp/internal"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// 创建业务服务
	svc := internal.NewBookService()

	// 创建 MCP 适配器
	adapter := internal.NewMCPAdapter(svc)
	mcpServer := adapter.CreateMCPServer()

	// 信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动 MCP Server（stdio 模式）
	go func() {
		log.Println("MCP Server 启动（stdio 模式）")
		if err := server.ServeStdio(mcpServer); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// 等待退出信号
	<-sigChan
	log.Println("Shutting down...")
}
