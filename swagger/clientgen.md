go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

> oapi-codegen --version
github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen
v2.7.1

[Environment]::SetEnvironmentVariable("PATH", $env:PATH + ";" + $(go env GOPATH) + "\bin", "User")

 mkdir my-api-mcp

  cd my-api-mcp




mkdir -p api          # OpenAPI 规范文件
mkdir -p gen          # 生成的代码
mkdir -p internal     # 业务逻辑实现
mkdir -p cmd/client   # Go Client 示例
mkdir -p cmd/server   # MCP Server 入口


go get github.com/oapi-codegen/runtime
go get github.com/go-chi/chi/v5
go get github.com/mark3labs/mcp-go

# 生成客户端、服务端、类型
oapi-codegen -config oapi-codegen.yaml api/spec.yaml


# 安装依赖
go mod tidy

# 构建 Client
go build -o bin/client cmd/client/main.go

# 构建 MCP Server
go build -o bin/mcp-server cmd/server/main.go


# 方式一：使用 MCP Inspector
npx @modelcontextprotocol/inspector node -e "
const { spawn } = require('child_process');
const server = spawn('./bin/mcp-server');
// ... inspector code
"

# 方式二：直接测试 stdio
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./bin/mcp-server


# 启动 HTTP 服务（需要额外实现）
go run cmd/client/main.go

{
  "mcpServers": {
    "book-management": {
      "command": "/absolute/path/to/my-api-mcp/bin/mcp-server",
      "args": []
    }
  }
}


make generate    # 生成代码
make build       # 构建
make run-server  # 运行 MCP Server
make test        # 运行测试


// MCP 支持流式内容
mcp.NewToolResultStream(stream)

# oapi-codegen.yaml 中添加
output-options:
  skip-prune: true
  nullable-type: true

