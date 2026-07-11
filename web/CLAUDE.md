# web/ - React 19 + Vite 7 前端

React 19 + Vite 7 + React Router 7 + Tailwind 4 + Zustand + TypeScript

## 目录结构

```
src/
  app/         - 应用 Provider、路由、样式与常驻布局壳层
  bootstrap/   - 启动阶段编排、React 根渲染与桌面恢复；`recovery/` 分离 chunk/auth 错误、重载哨兵和空白渲染 watchdog
  entries/     - App、Settings 与 OAuth 等构建入口
  pages/       - 页面入口与浏览器协调；`room/` 和 `contacts/` 各自分离页面控制器与 URL 协调
  features/    - 领域功能实现；`home/home-directory-resource.ts` 负责侧栏与通知共用的聊天目录，`home/hero/` 分离 ASCII Hero 的视图、Canvas 生命周期和粒子模型，`home/notifications/` 分离通知投影、浏览器边界和 Room 协议，`home/sidebar/` 分离聊天/联系人入口、目录投影、未读聚合与 Room 命令，`agents/options/` 统一可编辑字段投影、mutation 参数、草稿、校验与保存事务，`contacts/` 只提供目录、卡片和详情视图，`memory/catalog/` 负责 Agent 记忆目录请求与投影，`memory/document/` 分离文档作用域状态、实时资源、保存事务和视图，`conversation/room/workspace/controller/` 分离 Workspace Agent 作用域、文件资源、路径模型和命令，`conversation/room/workspace/view/` 分离文件列表布局、浏览器和弹窗，`capability/skills/` 负责技能市场及其状态域，`capability/connectors/` 按 catalog/detail/auth/controller 分离目录、详情、认证和命令，`capability/channels/` 按 catalog/connection/pairings 分离频道目录、连接状态机与 IM 配对，`capability/scheduled/controller/` 分离任务列表资源与写命令，`capability/scheduled/list/` 负责列表状态和单项展示，`capability/scheduled/dialog/` 按 form/schedule/resources 分离任务表单、调度规则与依赖资源，`capability/scheduled/pickers/` 统一时间列和锚定浮层，`conversation/shared/goal/` 负责 Goal 资源快照、命令和视图，`conversation/shared/session/` 统一 DM/Room 会话基础设施，`conversation/shared/timeline/` 负责时间线投影与窗口加载，`conversation/shared/timeline/scroll/` 负责跟随、锚定、动画和轮次 DOM 协议，`conversation/shared/todos/` 负责按轮次索引并合并 TodoWrite 与运行时任务，`conversation/shared/composer/controller/` 负责 DM/Room 输入状态与动作协议，`conversation/shared/feed/` 负责 DM 轮次渲染及共享虚拟列表协议，`conversation/shared/message/item/` 按 controller 与 view/content/assistant/user 分离轮次投影、内容块、助手和用户视图，`conversation/shared/subagent/` 负责子智能体列表、线程资源和命令，`conversation/room/dm/panel/` 负责 DM 页面模型与视图，`conversation/room/surface/header/` 保存 DM/Group 共用导航，`conversation/room/surface/mobile/` 分离移动端头部、会话 Sheet 和全屏 Overlay，`conversation/room/surface/layout/` 负责桌面分栏与右栏编排，`conversation/room/group/chat/panel/` 负责 Room 会话编排，`conversation/room/group/chat/feed/` 负责 Room 轮次渲染，`conversation/room/group/round/` 负责 Room Agent 轮次与 Thread 纯投影，`conversation/room/members/` 负责 Room 成员与设置表单，`conversation/shared/session-navigator/` 负责轮次导航，`operations/subscription-admin/` 负责订阅运营，`settings/general/` 按 model/sections/components 分离通用偏好、模型与视图，`settings/personal/` 分离个人资料资源、头像/密码命令、密码规则和视图，`settings/shared/` 保存设置型表面的跨域共享 UI，`settings/provider-settings/` 按 `model/`、`actions/config/` 与其他窄动作分离 Provider 纯模型、字段联动、持久化、删除和模型命令
  config/      - 运行时配置常量；`desktop-runtime/` 按宿主配置、鉴权、OAuth 和生命周期协议分层
  hooks/       - 自定义 React Hooks；`agent/` 按动作、消息模型、会话、运行态和传输协议分层
  lib/         - 无业务状态的基础函数与协议客户端；根目录保存错误、头像和未知值等跨领域纯投影，`format/` 按展示值类型分离格式化规则，`api/` 按 core/agent/account/capability/conversation/settings 分离传输与领域协议，`websocket/` 按策略、心跳、单连接客户端、共享通道和 React 生命周期分层
  shared/      - 无业务所有权的 UI、认证、i18n 和跨页面原语；`ui/` 按 button/form/display/list/navigation 分离基础交互职责，`i18n/catalog/` 按领域分离双语文案并逐分片校验键集合，`ui/markdown/` 统一 Markdown 渲染，`ui/mention/` 统一目标选择、文本匹配和插入，`ui/overlay/` 统一锚点定位与浏览器生命周期，`ui/menu/` 保存具体菜单语义
  store/       - Zustand 状态管理（agent + session 独立 store）
  types/       - 跨领域协议类型；`capability/scheduled-task/` 分离任务定义与运行结果，`conversation/message/` 分离附件、内容、实体和事件，`conversation/interaction/` 保存权限和用户问答协议
```

## 核心约定

- 组件 `PascalCase`，hooks `useXxx`，工具函数 `camelCase`
- 模块只导出跨文件消费的契约；文件内部使用的函数、常量和类型保持私有，禁止为潜在复用扩大公开面
- 跨领域协议声明归 `types/`，匹配算法和展示规则归消费层；API 通过 `types/api.ts` 共享 `ApiResponse<T>`
- Store 使用 Zustand persist middleware，数据持久化到 localStorage
- Agent WebSocket 信封校验与事件路由位于 `hooks/agent/transport/`，业务处理器不得回流到组件层
- Agent WebSocket 业务事件按 `transport/handlers/` 的消息、权限、重同步、Session 和作用域映射分域；当前 Session 守卫与事件所有权不得重复实现
- Agent conversation 公共 Hook 只做领域装配；消息去重、ACK 失败和稳定事件分发分别归属 `message/`、`actions/` 与 `transport/`
- Agent Session 由 `hooks/agent/session/controller/` 分离身份迁移、后台/易失快照与加载上下文；总控制器不复制 React setter
- Agent 运行态由 `hooks/agent/runtime/` 按纯模型、易失快照和 React 状态分层；状态机实例不得暴露给编排层，`model/` 不得反向依赖存储或 Hook
- WebSocket 连接策略只由 `lib/websocket/socket-policy.ts` 定义；共享通道使用完整有效配置作为身份，业务消息不得进入离线队列
- Workspace 会话标签由 `shared/ui/workspace/controls/conversation-tabs/` 分离纯模型、标签事务和单项视图；活动标签必须属于打开集合，视图不得直接修正集合状态
- `shared/`、`lib/`、`store/` 与 `types/` 不得依赖 `features/`；应用壳层组合 Feature 时必须归入 `app/` 或专用导航 Feature
- API 客户端按 endpoint 所有权归入 `lib/api/{agent,account,capability,conversation,settings}/`，通用传输只归 `core/`；消费者直接导入职责文件，不保留旧路径转发层
- 共享 UI 基础组件按 `button/`、`form/`、`display/`、`list/` 与 `navigation/` 分组；消费者直接导入职责文件，不恢复根级聚合出口
- 样式类名组合只由 `shared/ui/class-name.ts` 提供；时间、Token 和头像规则分别归 `lib/format/` 与 `lib/avatar.ts`，不得恢复混合 `lib/utils.ts`
- 翻译文案按 `shared/i18n/catalog/{zh,en}/` 的同名领域分片维护；中文定义键集合，英文必须通过 `MessageSegment` 精确覆盖，不恢复巨型语言文件
- Room API 按纯模型、查询和命令拆分，目录失效事件归 `lib/conversation/`；API 不得读取 Store，Direct Room 跳转与缺失 Agent 恢复归 `features/navigation/direct-room/`
- `unknown` 错误到用户消息的基础投影只由 `lib/error-message.ts` 定义；Feature 保留领域默认文案和反馈结构，不复制同义包装函数
- 外部 Session 通道别名、标签与合成会话 ID 只由 `lib/conversation/external-session.ts` 定义，页面和标签视图不得复制解释规则
- 权限与问答协议归 `types/conversation/interaction/`；权限和未完成工具调用的共享匹配归 `lib/conversation/`，问答超时与系统事件展示规则归消息 Feature
- 会话消息协议按 `types/conversation/message/{attachment,content,entity,event}.ts` 分离；WebSocket 信封和通用事件结构直接使用生成协议，消费者不得通过根 `types` barrel 或 `data: any` 绕过领域解码
- 定时任务协议按 `types/capability/scheduled-task/{task,run}.ts` 分离任务定义与运行结果；只声明真实消费者需要的契约，不恢复未接入的状态、事件和日报镜像
- 引导浮层由 `shared/ui/onboarding/overlay/` 分离目标/卡片观察器、定位策略、贴纸模型和步骤视图；Portal 入口不得重新实现这些规则
- 应用 Tour 目录和引导中心归 `features/onboarding/`；页面只注册当前 Tour 与锚点，跨页面导航、自动启动和目录投影不得下沉到 `shared/ui`
- Room 群聊面板只在 `panel/` 组合会话、Goal 与输入区模型；普通和虚拟消息流必须共用 `feed/group-conversation-round.tsx`，不得复制轮次分支
- DM 与 Room 只通过 `shared/session/use-conversation-session.ts` 串联运行时、滚动、历史和时间线；具体面板只装配业务模型
- DM/Room 滚动视口、历史提示、错误和浮动控制统一由 `shared/conversation-panel-layout.tsx` 渲染，不复制表面布局 class
- Room 与子智能体统一消费 `conversation/shared/thread/` 的 Thread 轮次契约和消息面板；共享域不得反向依赖 Room 私有目录
- 子智能体列表与线程复用 `shared/subagent/use-scoped-resource.ts` 的作用域请求协议；线程按资源、命令和纯投影拆分，公共 Hook 只做装配
- Room 主 Feed 与 Thread 共用 `room/group/round/round-agent-model.ts` 的 Agent 聚合状态；状态优先级不得在视图中重复推导
- Room 创建与管理弹窗只通过 `members/use-create-room-form.ts` 管理不变量，并以 `RoomDialogSubmission` 对象提交；视图组件不得在渲染期修正表单状态
- Home 侧栏与聊天通知只消费 `home-directory-resource.ts` 的共享目录快照；bootstrap 请求、刷新排队和全局目录事件不得在消费者中重复实现
- Home 侧栏只通过 `home/sidebar/` 组合聊天和联系人入口；Room/DM 基础投影与未读叠加必须独立缓存，视图不得直接调用 Room API 或拼通知键
- Home ASCII Hero 的 Canvas 资源只归 `home/hero/home-ascii-scene.ts`；异步字体与尺寸重建必须绑定代次，过期任务不得启动动画循环
- Agent Options 以 `agents/options/editor/agent-options-draft.ts` 的单一草稿为编辑真相；名称校验与保存完成必须同时匹配 Agent 作用域和草稿版本
- Agent Options 的可编辑字段和创建/更新参数只由 `agents/options/` 投影；Contacts 与 Room 不得复制 Options 字段表或 Agent 更新载荷
- Agent Options 业务弹窗归 `agents/options/dialog/`；`shared/ui/dialog/` 只提供无业务编辑器依赖的弹窗原语
- Agent 技能页由 `agents/options/components/skills/` 分离可取消列表资源、互斥安装命令、搜索投影和视图；异步结果必须匹配当前 Agent，状态机不得留在卡片组件
- AskUserQuestion 以按问题索引的原子回答草稿为唯一交互状态；工具作用域、结果恢复和提交互斥由 question controller 管理，header/card 不解析轮次协议
- 消息文本协议、时间格式和消息项投影分别归属 `message-content-model.ts`、`message-time.ts` 与 `item/message-item-projection.ts`；DOM 测量和活动状态不得进入通用 helper
- 消息项控制器返回按 User/Assistant 及视觉职责分组的具体状态；视图在消费侧定义窄契约，禁止恢复跨视图的扁平 `MessageItemState`
- 记忆列表请求必须绑定 Agent，文档加载与保存必须绑定 `agentId:path`；SDK 实时内容优先于旧 HTTP 响应，保存完成不得覆盖更新的草稿
- Memory 目录规则只由 `memory/catalog/` 的纯模型与单一描述表定义；正文资源和保存互斥分别归 `memory/document/` 的独立 Hook
- Workspace 文件快照与写命令按 Agent 作用域隔离；同 Agent 的后发刷新使先发请求失效，外部打开 Agent 信号只消费一次
- Room 页面数据资源必须绑定当前 `roomId`；模型只做投影，命令只返回当前作用域结果，会话快照只通过专用协议写回
- Room 页面私有控制器归 `pages/room/controller/`，浏览器协调归 `pages/room/orchestration/`；领域 Feature 不读取路由，页面不解释服务端资源协议
- Room 成员管理由页面命令层绑定作用域并按成员依赖顺序执行；Header 只提交完整表单对象，Surface 不传播成员增删和设置更新的散装回调
- Contacts 页面使用互斥编辑状态，资源和 CRUD 归 `pages/contacts/controller/`，URL 选择与 Room 跳转归 `pages/contacts/orchestration/`
- 宽侧栏由 `features/navigation/sidebar/` 管理；折叠栏与展开面板共用主 Tab、Nexus 入口和系统操作，路由/Store 同步只留在控制器
- 技能市场由 `features/capability/skills/controller/` 按目录、外部搜索、来源和操作拆分状态；子视图只消费窄 Props，不得依赖完整控制器
- 频道连接与 IM 配对分别持有命令互斥入口；写操作后必须使旧列表请求失效或按当前筛选刷新，视图不得复制协议字段别名
- 定时任务弹窗的表单和调度各自维护单一草稿对象，资源层按执行模式加载依赖并拒绝过期响应；Room 任务只允许绑定明确执行成员
- 定时任务时间选择器共用 `capability/scheduled/pickers/time-picker-column.tsx`，锚点浮层复用 `shared/ui/overlay/`，不得在 Daily/SingleRun 中复制选项按钮
- 定时任务目录只通过 `capability/scheduled/controller/` 读写任务；不得恢复混合 Heartbeat 的 Automation 控制器，命令结果必须先于后台刷新落地
- 定时任务运行历史由 `capability/scheduled/history/` 分离 Job 作用域资源、动作事务和纯视图；弹窗壳层不得直接请求 API 或维护单项命令状态
- Goal 面板只通过 `shared/goal/use-goal-controller.ts` 读写状态；资源快照必须绑定会话键，刷新拒绝过期响应，所有写命令共享互斥入口
- 桌面运行时只通过 `config/desktop-runtime/index.ts` 暴露稳定门面，消费者不得读取宿主原始全局对象或复制 URL 协议判断
- 根启动入口只编排运行时配置与渲染阶段；失败视图、chunk/auth 恢复、一次性重载和空白 watchdog 各自拥有独立边界
- Workspace Catalog 共享 UI 按卡片框架、内容结构、动作和图标容器分离；消费者直接导入职责模块，不恢复混合聚合出口
- Composer 由 `features/conversation/shared/composer/controller/` 分离草稿、投递、Goal/Loop、键盘和视图派生；面板只消费控制器结果
- General 设置由 `features/settings/general/` 统一编排；默认模型值直接派生自用户偏好和 Provider 默认值，不维护镜像选择状态
- 设置目录由 `features/settings/settings-navigation-model.ts` 定义，主应用侧栏与独立设置窗口必须复用 `settings-sidebar-navigation.tsx`；当前分区只由 URL 查询参数派生，不维护第二份选中状态；运营分区只对非桌面端 owner/admin 暴露，旧 `/operations` 入口必须收敛到设置目录
- Personal 设置只通过 `features/settings/personal/use-personal-settings-controller.ts` 读写资料；密码规则由纯模型的有序规则表定义，区块视图不得直接调用 Auth API
- Provider 由 `features/settings/provider-settings/workspace/` 管理原子状态与请求代次，`actions/config/` 和 `actions/model/` 分离配置及模型事务，目录、格式和能力标志只由纯展示模型投影
- Agent 身份页由 `features/agents/options/components/identity/` 的单一布局结构组合；资料、标签和模型选择各自拥有窄接口，待添加标签草稿必须绑定编辑作用域
- 通用 Markdown 只归 `shared/ui/markdown/`；Conversation 的 `message/markdown-renderer.tsx` 只解释消息文件产物协议，不得成为其他 Feature 的渲染入口
- 通用 Mention 只归 `shared/ui/mention/`；目标分类和标记由消费者投影，共享视图不得解释 Agent 或 Room
- 锚定浮层共用 `shared/ui/overlay/` 的定位、Portal 和关闭生命周期；Action/Select/MultiSelect 只在 `shared/ui/menu/` 维护菜单语义，消费者直接导入具体组件
- Launcher 按 `console/` 与 `hero/` 分离 API/导航和视觉/输入；服务端动作使用完整分发表，Hero 不直接访问领域 API
- Message item 的结构化内容关联只由 `view/content/content-renderer-model.ts` 建立；Assistant/User 视图不得再次扫描整轮内容或手写不完整的 Props 比较器
- Office 预览下载与载荷上限只由 `conversation/shared/editor/office-preview-resource.ts` 管理；文档预览的加载生命周期、DOM 归一化与视图分别归属 `document/` 下的 Hook、DOM 模型和视图模块
- DM/Room 虚拟消息流共用 `features/conversation/shared/feed/` 的容器测量与轮次导航协议；高度估算必须响应容器宽度变化
- DM/Room 时间线只从 `features/conversation/shared/timeline/timeline-model.ts` 派生轮次顺序；`timeline/window-loader/` 分离候选选择、有限重试账本和调度，窗口加载必须用会话代次隔离在途请求
- 对话滚动只通过 `features/conversation/shared/timeline/scroll/` 协调；面板不得复制底部阈值、RAF 动画、历史前插锚点或轮次 DOM 标记
- DM/Room Todo 只从 `features/conversation/shared/todos/` 的单遍轮次投影派生；计划、运行时任务和状态别名不得在面板中重复推导
- 会话导航由 `shared/session-navigator/` 分离时间线数据投影、刻度视觉模型、纯 DOM 定位和活动轮同步，`session-navigator/jump/` 分离目标、串行加载与落点确认；缺失窗口加载必须绑定会话键和请求代次，失效目标不得产生副作用
- 消息项由 `features/conversation/shared/message/item/controller/` 统一完成顺序、权限、过程链和最终回复投影；`view/` 不重复推导领域状态
- 消息内容块按 `blocks/{question,code,artifact,tool}/` 分域；工具状态与权限摘要只能由 `tool/tool-block-model.ts` 派生
- Room 桌面布局由 `features/conversation/room/surface/layout/` 分离 Header、辅助面板、Thread 和布局控制；移动端与桌面端共用 Surface 纯派生
- Room Group Header 按成员头像、指南菜单和主装配分离；异步弹窗状态必须绑定 Room 身份，不能跨路由复用布尔状态
- DM/Group Header 共用 `surface/header/` 的 Tab 定义与指南菜单；移动端只在 `surface/mobile/` 组合头部、会话 Sheet 和 Overlay，聊天主体必须复用 `room-chat-surface.tsx`
- 聊天渲染错误边界归 Room Surface，并以会话身份作为 reset key；错误状态和硬编码回退文案不得跨会话、跨布局复制
- Room 会话历史由 `features/conversation/room/surface/history/` 统一排序、外部 Session 能力、删除资格和标题编辑状态，基础协议层不保存展示专属规则
- 子智能体列表与线程资源必须绑定来源/任务作用域并拒绝旧请求写回；发送与停止共享命令互斥入口，UI 只依据服务端 capabilities 开放动作
- 环境变量统一使用 `VITE_*` 前缀，通过 `import.meta.env` 读取

## 配置文件

- `env.example` - 环境变量模板（开发/生产/域名）
- `vite.config.ts` - Vite 构建与别名配置
- `postcss.config.mjs` - PostCSS + Tailwind 4
- `tsconfig.json` - TypeScript 配置
- `Dockerfile` - 生产容器构建
