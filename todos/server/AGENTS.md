# 后端开发规范

## 编码原则
- 全面使用 TypeScript；非必要不用 `any`。
- 仅使用命名导出（Nitro 路由处理器除外，必须用默认导出）。
- 3 个以上参数、可选标志或含义不明确的参数使用 options 对象。

## 路由
- 基于文件的路由，位于 `routes/` 目录。文件名即路由路径。
- 方法后缀约定：`*.get.ts`、`*.post.ts`、`*.put.ts`、`*.delete.ts`。不带后缀则匹配所有方法。
- 动态参数：`[id].get.ts`。Catch-all：`[...path].ts`。
- 所有路由必须使用 `defineEventHandler` 编写。

## 自动导入
Nitro 自动注册常用函数，路由文件**无需手动 import**：
- 路由/请求工具：`defineEventHandler`、`readBody`、`getQuery`、`getRouterParam`、`getHeader`、`getCookie` 等
- 运行时配置：`useRuntimeConfig`
- 存储：`useStorage`
- `utils/` 目录下导出的函数也会自动注册

## 运行时配置
- 在 `nitro.config.ts` 的 `runtimeConfig` 中声明配置项及默认值。
- 在路由中通过 `useRuntimeConfig()` 读取。
- **禁止直接 `process.env`**：平台在同一进程中运行多个项目，`process.env` 是全局共享的。

## 数据库访问
- 数据库客户端由 `server/middleware/db.ts` 自动注入到 `event.context.db`，路由中直接使用。
- middleware 自动从请求头读取 `X-Project-Api-Key` 和 `Cookie`，自动鉴权。
- 支持 `db.query()`、`db.insert()`、`db.update()`、`db.delete()` 四个方法。

## 禁止事项
- `http.createServer()` / `server.listen()`：平台管理服务生命周期。
- `process.env.XXX`：使用 `useRuntimeConfig()` 替代。
- Express / Koa 等框架：必须用 Nitro。
- 模块顶层有状态单例：平台可能随时回收实例。

## 命令
`pnpm run dev`、`pnpm run build`、`pnpm run pack:server`、`pnpm run lint`、`pnpm run check:types`。
