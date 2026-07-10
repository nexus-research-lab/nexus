# web/ - React 19 + Vite 7 前端

React 19 + Vite 7 + React Router 7 + Tailwind 4 + Zustand + TypeScript

## 目录结构

```
src/
  pages/       - 页面组件
  routes/      - React Router 路由定义
  components/  - UI 组件（按功能领域组织）
  features/    - 领域功能实现；`capability/skills/` 负责技能市场及其状态域，`conversation/shared/composer/` 负责 DM/Room 共用输入区，`conversation/room/group/chat/panel/` 负责 Room 会话编排，`conversation/room/group/chat/feed/` 负责 Room 轮次渲染，`conversation/room/members/` 负责 Room 成员与设置表单，`conversation/shared/subagent/` 负责子智能体任务，`conversation/shared/session-navigator/` 负责轮次导航，`operations/subscription-admin/` 负责订阅运营，`settings/provider-settings/` 负责 Provider 配置领域
  config/      - 运行时配置常量；`desktop-runtime/` 按宿主配置、鉴权、OAuth 和生命周期协议分层
  hooks/       - 自定义 React Hooks；`agent/` 按动作、会话、运行态和传输协议分层
  lib/         - API 客户端、WebSocket、工具函数
  store/       - Zustand 状态管理（agent + session 独立 store）
  types/       - TypeScript 类型定义
```

## 核心约定

- 组件 `PascalCase`，hooks `useXxx`，工具函数 `camelCase`
- 类型集中在 `types/` 下统一导出，API 层通过 `types/api.ts` 共享 `ApiResponse<T>`
- Store 使用 Zustand persist middleware，数据持久化到 localStorage
- Agent WebSocket 信封校验与事件路由位于 `hooks/agent/transport/`，业务处理器不得回流到组件层
- Room 群聊面板只在 `panel/` 组合会话、Goal 与输入区模型；普通和虚拟消息流必须共用 `feed/group-conversation-round.tsx`，不得复制轮次分支
- Room 创建与管理弹窗只通过 `members/use-create-room-form.ts` 管理不变量，并以 `RoomDialogSubmission` 对象提交；视图组件不得在渲染期修正表单状态
- 宽侧栏由 `shared/ui/sidebar/wide-panel/` 管理；折叠栏与展开面板共用主 Tab、Nexus 入口和系统操作，路由/Store 同步只留在入口控制器
- 技能市场由 `features/capability/skills/controller/` 按目录、外部搜索、来源和操作拆分状态；子视图只消费窄 Props，不得依赖完整控制器
- 桌面运行时只通过 `config/desktop-runtime/index.ts` 暴露稳定门面，消费者不得读取宿主原始全局对象或复制 URL 协议判断
- 子智能体 UI 只依据服务端下发的 capabilities 开放停止、发送和恢复动作；runtime kind 仅用于呈现 nxs/Claude Code 差异
- 环境变量统一使用 `VITE_*` 前缀，通过 `import.meta.env` 读取

## 配置文件

- `env.example` - 环境变量模板（开发/生产/域名）
- `vite.config.ts` - Vite 构建与别名配置
- `postcss.config.mjs` - PostCSS + Tailwind 4
- `tsconfig.json` - TypeScript 配置
- `Dockerfile` - 生产容器构建
