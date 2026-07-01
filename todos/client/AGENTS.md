# 前端开发规范

## 编码原则
- 全面使用 TypeScript；非必要不用 `any`。
- 仅使用命名导出（页面组件除外，可用默认导出）。
- 使用 `@/` 绝对路径导入，同目录内可用相对路径。
- 3 个以上参数、可选标志或含义不明确的参数使用 options 对象。

## 样式
- 使用 Tailwind v4 工具类。
- shadcn 组件位于 `components/ui/`（通过 `pnpm dlx shadcn add <名称>` 添加）。

## 路由
- 必须使用 `HashRouter`，**禁止使用 `BrowserRouter`**。
  部署平台会将应用挂载到子路径（如 `/app/<id>/<slug>/`），`BrowserRouter` 无法匹配。

## API 请求
- fetch 调用必须使用**相对路径** `./api/...`，**禁止使用绝对路径** `/api/...`。
  部署后应用在子路径下，绝对路径会跳过子路径前缀导致 404。

## React
- 不使用 `useMemo`/`useCallback`（React 编译器自动处理）。尽量避免 `useEffect`。
- 组件使用单个 `props` 参数并内联类型定义；通过 `props.foo` 访问（不解构）。

## 环境变量
- Vite 通过 `import.meta.env` 暴露 `VITE_*` 前缀的环境变量。

## 服务端接口调用
- 使用裸 `fetch('./api/...')` 调用服务端路由，不需要 `credentials: "include"`。
- 禁止直连外部 API（如 `https://openapi.wps.cn`），所有数据操作通过服务端路由。

## 命令
`pnpm run dev`、`pnpm run build`、`pnpm run pack:frontend`、`pnpm run lint`、`pnpm run check:types`。
