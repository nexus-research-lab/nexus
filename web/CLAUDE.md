# web/ - React 19 + Vite 7 前端

React 19 + Vite 7 + React Router 7 + Tailwind 4 + Zustand + TypeScript

## 目录结构

```
src/
  pages/       - 页面组件
  routes/      - React Router 路由定义
  components/  - UI 组件（按功能领域组织）
  features/    - 领域功能实现；`home/home-directory-resource.ts` 负责侧栏与通知共用的聊天目录，`home/notifications/` 分离通知投影、浏览器边界和 Room 协议，`agents/options/editor/` 负责单一 Agent 配置草稿、校验与保存事务，`memory/` 负责 SDK 记忆投影、Agent 快照与文档资源，`conversation/room/workspace/controller/` 分离 Workspace Agent 作用域、文件资源、路径模型和命令，`conversation/room/workspace/view/` 分离文件列表布局、浏览器和弹窗，`capability/skills/` 负责技能市场及其状态域，`capability/connectors/` 按 catalog/detail/auth/controller 分离目录、详情、认证和命令，`capability/channels/` 按 catalog/connection/pairings 分离频道目录、连接状态机与 IM 配对，`capability/scheduled/dialog/` 按 form/schedule/resources 分离任务表单、调度规则与依赖资源，`conversation/shared/goal/` 负责 Goal 资源快照、命令和视图，`conversation/shared/session/` 统一 DM/Room 会话基础设施，`conversation/shared/timeline/` 负责时间线投影与窗口加载，`conversation/shared/timeline/scroll/` 负责跟随、锚定、动画和轮次 DOM 协议，`conversation/shared/todos/` 负责按轮次索引并合并 TodoWrite 与运行时任务，`conversation/shared/composer/controller/` 负责 DM/Room 输入状态与动作协议，`conversation/shared/feed/` 负责 DM 轮次渲染及共享虚拟列表协议，`conversation/shared/message/item/` 按控制与视图分层，`conversation/shared/subagent/` 负责子智能体列表、线程资源和命令，`conversation/room/dm/panel/` 负责 DM 页面模型与视图，`conversation/room/surface/layout/` 负责桌面分栏与右栏编排，`conversation/room/group/chat/panel/` 负责 Room 会话编排，`conversation/room/group/chat/feed/` 负责 Room 轮次渲染，`conversation/room/group/round/` 负责 Room Agent 轮次与 Thread 纯投影，`conversation/room/members/` 负责 Room 成员与设置表单，`conversation/shared/session-navigator/` 负责轮次导航，`operations/subscription-admin/` 负责订阅运营，`settings/general/` 负责通用偏好与默认模型，`settings/provider-settings/` 按 `model/` 与 `actions/` 分离 Provider 纯模型和异步命令
  config/      - 运行时配置常量；`desktop-runtime/` 按宿主配置、鉴权、OAuth 和生命周期协议分层
  hooks/       - 自定义 React Hooks；`agent/` 按动作、消息模型、会话、运行态和传输协议分层
  lib/         - API 客户端、工具函数；`websocket/` 按策略、心跳、单连接客户端、共享通道和 React 生命周期分层
  store/       - Zustand 状态管理（agent + session 独立 store）
  types/       - TypeScript 类型定义
```

## 核心约定

- 组件 `PascalCase`，hooks `useXxx`，工具函数 `camelCase`
- 类型集中在 `types/` 下统一导出，API 层通过 `types/api.ts` 共享 `ApiResponse<T>`
- Store 使用 Zustand persist middleware，数据持久化到 localStorage
- Agent WebSocket 信封校验与事件路由位于 `hooks/agent/transport/`，业务处理器不得回流到组件层
- WebSocket 连接策略只由 `lib/websocket/socket-policy.ts` 定义；共享通道使用完整有效配置作为身份，业务消息不得进入离线队列
- Room 群聊面板只在 `panel/` 组合会话、Goal 与输入区模型；普通和虚拟消息流必须共用 `feed/group-conversation-round.tsx`，不得复制轮次分支
- DM 与 Room 只通过 `shared/session/use-conversation-session.ts` 串联运行时、滚动、历史和时间线；具体面板只装配业务模型
- DM/Room 滚动视口、历史提示、错误和浮动控制统一由 `shared/conversation-panel-layout.tsx` 渲染，不复制表面布局 class
- Room 主 Feed 与 Thread 共用 `room/group/round/round-agent-model.ts` 的 Agent 聚合状态；状态优先级不得在视图中重复推导
- Room 创建与管理弹窗只通过 `members/use-create-room-form.ts` 管理不变量，并以 `RoomDialogSubmission` 对象提交；视图组件不得在渲染期修正表单状态
- Home 侧栏与聊天通知只消费 `home-directory-resource.ts` 的共享目录快照；bootstrap 请求、刷新排队和全局目录事件不得在消费者中重复实现
- Agent Options 以 `agents/options/editor/agent-options-draft.ts` 的单一草稿为编辑真相；名称校验与保存完成必须同时匹配 Agent 作用域和草稿版本
- 记忆列表请求必须绑定 Agent，文档加载与保存必须绑定 `agentId:path`；SDK 实时内容优先于旧 HTTP 响应，保存完成不得覆盖更新的草稿
- Workspace 文件快照与写命令按 Agent 作用域隔离；同 Agent 的后发刷新使先发请求失效，外部打开 Agent 信号只消费一次
- Room 页面数据资源必须绑定当前 `roomId`；模型只做投影，命令只返回当前作用域结果，会话快照只通过专用协议写回
- 宽侧栏由 `shared/ui/sidebar/wide-panel/` 管理；折叠栏与展开面板共用主 Tab、Nexus 入口和系统操作，路由/Store 同步只留在入口控制器
- 技能市场由 `features/capability/skills/controller/` 按目录、外部搜索、来源和操作拆分状态；子视图只消费窄 Props，不得依赖完整控制器
- 频道连接与 IM 配对分别持有命令互斥入口；写操作后必须使旧列表请求失效或按当前筛选刷新，视图不得复制协议字段别名
- 定时任务弹窗的表单和调度各自维护单一草稿对象，资源层按执行模式加载依赖并拒绝过期响应；Room 任务只允许绑定明确执行成员
- Goal 面板只通过 `shared/goal/use-goal-controller.ts` 读写状态；资源快照必须绑定会话键，刷新拒绝过期响应，所有写命令共享互斥入口
- 桌面运行时只通过 `config/desktop-runtime/index.ts` 暴露稳定门面，消费者不得读取宿主原始全局对象或复制 URL 协议判断
- Composer 由 `features/conversation/shared/composer/controller/` 分离草稿、投递、Goal/Loop、键盘和视图派生；面板只消费控制器结果
- General 设置由 `features/settings/general/` 统一编排；默认模型值直接派生自用户偏好和 Provider 默认值，不维护镜像选择状态
- DM/Room 虚拟消息流共用 `features/conversation/shared/feed/` 的容器测量与轮次导航协议；高度估算必须响应容器宽度变化
- DM/Room 时间线只从 `features/conversation/shared/timeline/timeline-model.ts` 派生轮次顺序；窗口加载必须用会话代次隔离在途请求，并对失败执行有限重试
- 对话滚动只通过 `features/conversation/shared/timeline/scroll/` 协调；面板不得复制底部阈值、RAF 动画、历史前插锚点或轮次 DOM 标记
- DM/Room Todo 只从 `features/conversation/shared/todos/` 的单遍轮次投影派生；计划、运行时任务和状态别名不得在面板中重复推导
- 会话导航由 `shared/session-navigator/` 分离纯 DOM 定位、活动轮同步与跳转事务；缺失窗口加载必须绑定会话键，失效目标不得继续阻塞时间线加载
- 消息项由 `features/conversation/shared/message/item/controller/` 统一完成顺序、权限、过程链和最终回复投影；`view/` 不重复推导领域状态
- 消息内容块按 `blocks/{question,code,artifact,tool}/` 分域；工具状态与权限摘要只能由 `tool/tool-block-model.ts` 派生
- Room 桌面布局由 `features/conversation/room/surface/layout/` 分离 Header、辅助面板、Thread 和布局控制；移动端与桌面端共用 Surface 纯派生
- 子智能体列表与线程资源必须绑定来源/任务作用域并拒绝旧请求写回；发送与停止共享命令互斥入口，UI 只依据服务端 capabilities 开放动作
- 环境变量统一使用 `VITE_*` 前缀，通过 `import.meta.env` 读取

## 配置文件

- `env.example` - 环境变量模板（开发/生产/域名）
- `vite.config.ts` - Vite 构建与别名配置
- `postcss.config.mjs` - PostCSS + Tailwind 4
- `tsconfig.json` - TypeScript 配置
- `Dockerfile` - 生产容器构建
