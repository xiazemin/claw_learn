# AGENTS

## 架构
- 工作区包含两个子包：`client/`（Vite + React 19 + Tailwind 4 + shadcn）和 `server/`（Nitro 2）。
- 前端构建为静态文件（`dist-frontend.zip`），部署到 app-runtime（Nginx）。
- 后端构建为 Nitro node preset（`.output/` 打包为 `dist-server.zip`），部署到 app-runtime-be。
- 数据库访问通过 `server/middleware/db.ts` 自动注入到 `event.context.db`，连接 App Studio Runtime API。

## 开发流程

```
pnpm run dev → 编码 → pnpm run pack（每次改完代码都要执行）
```

### 1. 启动开发环境
```bash
pnpm run dev
```
同时启动前端（Vite，http://localhost:5917）和后端（Nitro，http://localhost:4917）。
前端 `/api/*` 请求自动代理到后端。支持 HMR 热更新，保存文件即刷新。

> 端口被占用时直接 kill：`lsof -ti :5917 | xargs kill` / `lsof -ti :4917 | xargs kill`

### 2. 编码

- 前端代码写在 `client/src/`，新建页面放 `pages/`，组件放 `components/`。
- 后端接口写在 `server/routes/api/`，文件名即路由，方法后缀即 HTTP Method（如 `todos.get.ts`）。
- 需要新的 UI 组件时运行 `pnpm dlx shadcn add <名称>`（在 client 目录下）。
- 数据库操作通过 `event.context.db` 在路由中使用（middleware 自动注入，无需 import）。

### 3. 提交前检查
```bash
pnpm run lint          # oxlint 代码检查
pnpm run check:types   # TypeScript 类型检查
```
两个命令都会同时检查前端和后端。确保零错误再提交。

### 4. 打包部署
```bash
pnpm run pack            # 同时打包前端和后端
```

## 开发规范
- 前端：见 [`client/AGENTS.md`](client/AGENTS.md)
- 后端：见 [`server/AGENTS.md`](server/AGENTS.md)
