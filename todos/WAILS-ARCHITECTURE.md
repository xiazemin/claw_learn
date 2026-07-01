# Wails 桌面应用架构详解

## 一、Wails 是什么

Wails 是一个用 Go 语言构建桌面应用的框架。它的核心思想：

```
┌─────────────────────────────────────────────────┐
│              Wails 桌面应用                      │
│  ┌───────────────────┐  ┌─────────────────────┐ │
│  │   Go 后端进程      │  │  WebView2 渲染引擎   │ │
│  │                   │  │  (Chromium 内核)     │ │
│  │  - 业务逻辑       │◄─┤  - React/Vue/HTML   │ │
│  │  - 文件读写       │──►│  - CSS/Tailwind     │ │
│  │  - 系统调用       │IPC│  - TypeScript        │ │
│  │  - 数据持久化     │  │                     │ │
│  └───────────────────┘  └─────────────────────┘ │
│         ▲                        ▲               │
│         │                        │               │
│         └────── IPC 桥接 ────────┘               │
└─────────────────────────────────────────────────┘
```

**一句话总结**：Wails = Go 后端 + Web 前端，通过 IPC 通信，打包成单个 `.exe`。

---

## 二、Wails 和 TypeScript 的配合方式

### 2.1 通信模型

```
TypeScript 前端                          Go 后端
     │                                      │
     │  window.go.main.App.GetAll()         │
     │──────────────────────────────────────►│
     │                                      │  调用 Go 函数
     │                                      │  读取 JSON 文件
     │                                      │  返回 []Todo
     │◄──────────────────────────────────────│
     │  Promise<Todo[]> resolve             │
     │                                      │
```

Wails 通过 WebView2 的 `postMessage` 机制实现了 Go ↔ JavaScript 的双向调用：
- **JS → Go**：前端调用 `window.go.main.App.GetAll()`，Wails 通过 IPC 转发给 Go
- **Go → JS**：Go 可以通过 `runtime.EventsEmit` 向前端推送事件

### 2.2 自动生成的绑定代码

当你在 Go 中定义了一个导出方法：

```go
func (a *App) GetAll() []Todo { ... }
```

Wails 会自动生成对应的 TypeScript/JavaScript 绑定文件：

```typescript
// wailsjs/go/main/App.d.ts (自动生成)
export function GetAll(): Promise<Todo[]>;
```

```javascript
// wailsjs/go/main/App.js (自动生成)
export function GetAll() {
  return window['go']['main']['App']['GetAll']();
}
```

前端代码直接调用，就像调用本地函数一样：

```typescript
// 前端代码
import { GetAll } from "../../wailsjs/go/main/App";

const todos = await GetAll();  // 实际是 IPC 调用 Go
```

### 2.3 类型安全

Go 结构体 ↔ TypeScript 接口的映射：

```go
// Go
type Todo struct {
    ID        string `json:"id"`
    Title     string `json:"title"`
    Category  string `json:"category"`
    Completed bool   `json:"completed"`
    CreatedAt int64  `json:"createdAt"`
}
```

```typescript
// TypeScript (自动生成)
interface Todo {
  id: string;
  title: string;
  category: string;
  completed: boolean;
  createdAt: number;
}
```

Go 的 `json:"id"` tag 直接映射为 TypeScript 的属性名。

---

## 三、逐行解析 Go 代码

### 3.1 main.go — 应用入口

```go
package main
```
Go 程序的入口包。每个 Go 可执行文件必须有一个 `main` 包。

```go
import (
    "embed"
    "github.com/wailsapp/wails/v2"
    "github.com/wailsapp/wails/v2/pkg/options"
    "github.com/wailsapp/wails/v2/pkg/options/assetserver"
)
```
- `embed`：Go 1.16+ 内置包，用于将静态文件嵌入到二进制文件中
- `wails/v2`：Wails 框架核心
- `options`：应用配置选项
- `assetserver`：前端静态资源服务配置

```go
//go:embed all:frontend/dist
var assets embed.FS
```
**关键指令**：`//go:embed` 是 Go 编译器指令（不是注释），它将 `frontend/dist` 目录下的所有文件在编译时嵌入到 `assets` 变量中。`all:` 前缀表示包含隐藏文件。这样前端代码就和 Go 一起编译成单个 `.exe`，不需要额外的文件。

```go
func main() {
    app := NewApp()
```
创建 `App` 实例。`NewApp()` 在 `app.go` 中定义，返回一个持有应用状态的结构体指针。

```go
    err := wails.Run(&options.App{
        Title:  "个人待办",
        Width:  800,
        Height: 600,
```
启动 Wails 应用，配置窗口标题和尺寸。

```go
        AssetServer: &assetserver.Options{
            Assets: assets,
        },
```
告诉 Wails 使用嵌入的 `assets` 作为前端静态资源。Wails 内置了一个 HTTP 服务器，从这些嵌入的文件中提供 `index.html`、JS、CSS。

```go
        OnStartup: app.startup,
```
注册启动回调。应用窗口创建后、加载前端之前，会调用 `app.startup(ctx)`。这是初始化后端状态（如读取本地数据文件）的时机。

```go
        Bind: []interface{}{
            app,
        },
```
**核心配置**：将 `app` 对象的所有导出方法注册为前端可调用的绑定。Wails 会扫描 `app` 的所有 `func (a *App) Xxx() Yyy` 方法，为每个方法生成 JS 绑定。前端通过 `window.go.main.App.Xxx()` 调用。

```go
    })
    if err != nil {
        println("Error:", err.Error())
    }
}
```
如果 `wails.Run` 返回错误（如 WebView2 未安装），打印错误信息。

---

### 3.2 app.go — 业务逻辑

#### 数据模型

```go
type Todo struct {
    ID        string `json:"id"`
    Title     string `json:"title"`
    Category  string `json:"category"`
    Completed bool   `json:"completed"`
    CreatedAt int64  `json:"createdAt"`
}
```
**JSON 映射**：`json:"id"` tag 告诉 Go 的 `encoding/json` 包在序列化时使用 `"id"` 作为键名。这和 TypeScript 接口一一对应：

| Go 字段 | JSON key | TypeScript 属性 |
|---------|----------|----------------|
| `ID` | `"id"` | `id` |
| `Title` | `"title"` | `title` |
| `Category` | `"category"` | `category` |
| `Completed` | `"completed"` | `completed` |
| `CreatedAt` | `"createdAt"` | `createdAt` |

#### 应用结构体

```go
type App struct {
    ctx      context.Context
    mu       sync.Mutex
    todos    []Todo
    filePath string
}
```

| 字段 | 类型 | 作用 |
|------|------|------|
| `ctx` | `context.Context` | Wails 提供的上下文，用于访问运行时 API（日志、事件、对话框等） |
| `mu` | `sync.Mutex` | 互斥锁，保护 `todos` 的并发访问 |
| `todos` | `[]Todo` | 内存中的待办列表 |
| `filePath` | `string` | 本地 JSON 数据文件的完整路径 |

#### 构造函数

```go
func NewApp() *App {
    return &App{}
}
```
创建并返回一个空的 `App` 结构体指针。`&App{}` 是 Go 的复合字面量语法，`&` 取地址。返回指针是因为 `App` 的方法需要修改内部状态（`todos`），值类型会导致修改丢失。

#### 启动回调

```go
func (a *App) startup(ctx context.Context) {
    a.ctx = ctx
```
`(a *App)` 是 Go 的**方法接收者**语法。`startup` 是 `App` 的一个方法，`a` 是接收者变量（类似 `this`）。当 Wails 启动时调用 `app.startup(ctx)`，将 Wails 上下文保存到结构体中。

```go
    var dir string
    u, err := user.Current()
    if err != nil {
        dir = "."
    } else {
        dir = filepath.Join(u.HomeDir, ",todos-desktop")
    }
```
获取当前用户的主目录，拼接应用数据目录路径。例如 Windows 上是 `C:\Users\wps\.todos-desktop`。

- `user.Current()`：获取当前操作系统用户信息
- `filepath.Join()`：跨平台路径拼接（Windows 用 `\`，Linux 用 `/`）
- `u.HomeDir`：用户主目录（如 `C:\Users\wps`）

```go
    os.MkdirAll(dir, 0755)
    a.filePath = filepath.Join(dir, "todos.json")
```
- `os.MkdirAll(dir, 0755)`：递归创建目录。`0755` 是 Unix 权限位：所有者读写执行，其他人读执行。Windows 上此参数被忽略
- `a.filePath`：设置数据文件完整路径为 `~/.todos-desktop/todos.json`

```go
    a.load()
}
```
启动时从文件加载已有的待办数据到内存。

#### 数据加载

```go
func (a *App) load() {
    data, err := os.ReadFile(a.filePath)
    if err != nil {
        a.todos = []Todo{}
        return
    }
    json.Unmarshal(data, &a.todos)
}
```

逐行解析：

1. `os.ReadFile(a.filePath)`：读取整个文件为 `[]byte`（字节切片）。如果文件不存在，返回错误
2. `if err != nil`：文件不存在是正常情况（首次运行），初始化为空列表
3. `a.todos = []Todo{}`：创建空的 Todo 切片
4. `json.Unmarshal(data, &a.todos)`：将 JSON 字节反序列化为 `[]Todo` 结构体切片

#### 数据保存

```go
func (a *App) save() {
    data, _ := json.MarshalIndent(a.todos, "", "  ")
    os.WriteFile(a.filePath, data, 0644)
}
```

1. `json.MarshalIndent(a.todos, "", "  ")`：将 `[]Todo` 序列化为带缩进的 JSON（方便人类阅读）。第二个参数是前缀，第三个是缩进字符串
2. `_`：Go 的空白标识符，表示忽略 `MarshalIndent` 可能返回的错误
3. `os.WriteFile(a.filePath, data, 0644)`：将 JSON 字节写入文件。`0644` 权限：所有者读写，其他人只读

#### CRUD 方法

```go
func (a *App) GetAll() []Todo {
    a.mu.Lock()
    defer a.mu.Unlock()
    return a.todos
}
```

- `a.mu.Lock()`：获取互斥锁。如果有其他 goroutine 正在操作 `todos`，这里会阻塞等待
- `defer a.mu.Unlock()`：`defer` 关键字表示这个函数返回**之前**执行。确保无论函数如何退出（正常返回/panic），锁都会被释放。这是 Go 的惯用模式
- `return a.todos`：返回内存中的完整待办列表

```go
func (a *App) Add(title string, category string) Todo {
    a.mu.Lock()
    defer a.mu.Unlock()

    todo := Todo{
        ID:        uuid.New().String(),
        Title:     title,
        Category:  category,
        Completed: false,
        CreatedAt: int64(0),
    }
    a.todos = append([]Todo{todo}, a.todos...)
    a.save()
    return todo
}
```

- `uuid.New().String()`：生成 RFC 4122 v4 随机 UUID（如 `"550e8400-e29b-41d4-a716-446655440000"`）
- `append([]Todo{todo}, a.todos...)`：将新待办插入到列表**开头**。`...` 是展开运算符，将切片展开为独立参数
- `a.save()`：每次修改后立即持久化到文件

```go
func (a *App) Toggle(id string) {
    a.mu.Lock()
    defer a.mu.Unlock()

    for i := range a.todos {
        if a.todos[i].ID == id {
            a.todos[i].Completed = !a.todos[i].Completed
            break
        }
    }
    a.save()
}
```

- `for i := range a.todos`：遍历切片，`i` 是索引
- `a.todos[i].ID == id`：按 UUID 查找目标待办
- `!a.todos[i].Completed`：取反，切换完成/未完成状态
- `break`：找到后立即退出循环

```go
func (a *App) Remove(id string) {
    a.mu.Lock()
    defer a.mu.Unlock()

    filtered := make([]Todo, 0, len(a.todos))
    for _, t := range a.todos {
        if t.ID != id {
            filtered = append(filtered, t)
        }
    }
    a.todos = filtered
    a.save()
}
```

- `make([]Todo, 0, len(a.todos))`：预分配切片，容量为当前长度，避免频繁扩容
- `for _, t := range a.todos`：`_` 表示忽略索引，`t` 是当前元素的副本
- 过滤掉 ID 匹配的项，保留其余

---

## 四、编译流程

```
┌─────────────────────────────────────────────────────────┐
│  wails build -skipbindings                              │
│                                                         │
│  Step 1: pnpm install (前端依赖)                        │
│  Step 2: vite build (React → dist/)                     │
│  Step 3: //go:embed (dist/ → 嵌入 Go 二进制)            │
│  Step 4: go build (Go → todos-desktop.exe)              │
│                                                         │
│  最终产物:                                              │
│  ┌──────────────────────────────────────┐               │
│  │  todos-desktop.exe (13MB)            │               │
│  │  ┌────────────────────────────────┐  │               │
│  │  │  Go runtime                    │  │               │
│  │  │  + Wails framework             │  │               │
│  │  │  + App 业务逻辑                │  │               │
│  │  │  + 嵌入的 React dist/          │  │               │
│  │  │  + WebView2 loader             │  │               │
│  │  └────────────────────────────────┘  │               │
│  └──────────────────────────────────────┘               │
└─────────────────────────────────────────────────────────┘
```

---

## 五、运行时数据流

```
用户点击"添加"
    │
    ▼
React: handleAdd("买菜", "生活")
    │
    ▼
JavaScript: await Add("买菜", "生活")
    │
    ▼ (IPC postMessage)
Wails Runtime Bridge
    │
    ▼
Go: func (a *App) Add(title, category) Todo
    │
    ├─► uuid.New().String()  → 生成 ID
    ├─► a.todos = prepend    → 内存更新
    ├─► json.MarshalIndent   → 序列化
    ├─► os.WriteFile         → 写入 ~/.todos-desktop/todos.json
    │
    ▼
Go 返回 Todo 结构体 → JSON 序列化 → IPC → JavaScript Promise resolve
    │
    ▼
React: setTodos(await todoStorage.getAll())
    │
    ▼
UI 更新，显示新待办
```

---

## 六、关键概念对照表

| 概念 | Wails 中的位置 | 作用 |
|------|--------------|------|
| `//go:embed` | `main.go` | 编译时嵌入前端文件 |
| `Bind` | `main.go` | 注册 Go 方法为前端可调用 |
| `OnStartup` | `main.go` | 应用启动时的初始化钩子 |
| `AssetServer` | `main.go` | 前端静态资源服务器 |
| `sync.Mutex` | `app.go` | 并发安全保护 |
| `defer` | `app.go` | 函数退出前执行（用于解锁） |
| `json.Marshal` | `app.go` | Go 结构体 → JSON |
| `json.Unmarshal` | `app.go` | JSON → Go 结构体 |
| `uuid.New()` | `app.go` | 生成唯一 ID |
| `context.Context` | `app.go` | Wails 运行时上下文 |
