# web/ - React 19 + Vite 7 前端

React 19 + Vite 7 + React Router 7 + Tailwind 4 + Zustand + TypeScript

## 目录结构

```
src/
  pages/       - 页面组件
  routes/      - React Router 路由定义
  components/  - UI 组件（按功能领域组织）
  features/    - 领域功能实现；`conversation/shared/composer/` 负责 DM/Room 共用输入区，`conversation/shared/subagent/` 负责子智能体任务，`conversation/shared/session-navigator/` 负责轮次导航，`operations/subscription-admin/` 负责订阅运营，`settings/provider-settings/` 负责 Provider 配置领域
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
- 子智能体 UI 只依据服务端下发的 capabilities 开放停止、发送和恢复动作；runtime kind 仅用于呈现 nxs/Claude Code 差异
- 环境变量统一使用 `VITE_*` 前缀，通过 `import.meta.env` 读取

## 配置文件

- `env.example` - 环境变量模板（开发/生产/域名）
- `vite.config.ts` - Vite 构建与别名配置
- `postcss.config.mjs` - PostCSS + Tailwind 4
- `tsconfig.json` - TypeScript 配置
- `Dockerfile` - 生产容器构建
