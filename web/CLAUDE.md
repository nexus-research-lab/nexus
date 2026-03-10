# web/ - Next.js 16 + React 19 前端

Next.js 16 + React 19 + Tailwind 4 + Zustand + TypeScript

## 目录结构

```
src/
  app/         - Next.js App Router 页面（单页应用，仅根路由）
  components/  - UI 组件（按功能领域组织）
  config/      - 运行时配置常量
  hooks/       - 自定义 React Hooks
  lib/         - API 客户端、WebSocket、工具函数
  store/       - Zustand 状态管理（agent + session 独立 store）
  types/       - TypeScript 类型定义
```

## 核心约定

- 组件 `PascalCase`，hooks `useXxx`，工具函数 `camelCase`
- 类型集中在 `types/` 下统一导出，API 层通过 `types/api.ts` 共享 `ApiResponse<T>`
- Store 使用 Zustand persist middleware，数据持久化到 localStorage
- WebSocket 消息处理纯函数独立于 `hooks/agent/message-reducers.ts`

## 配置文件

- `env.example` - 环境变量模板（开发/生产/域名）
- `next.config.ts` - Next.js 配置（standalone 输出）
- `postcss.config.mjs` - PostCSS + Tailwind 4
- `tsconfig.json` - TypeScript 配置
- `Dockerfile` - 生产容器构建
