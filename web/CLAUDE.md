# web/ - React 19 + Vite 7 前端

React 19 + Vite 7 + React Router 7 + Tailwind 4 + Zustand + TypeScript

## 目录结构

```
src/
  app/         - 应用 Provider、路由、样式与常驻布局壳层；`auth/` 持有认证事务，`runtime-options-resource.ts` 负责编排运行时配置拉取与快照提交
  bootstrap/   - 启动阶段编排、React 根渲染与桌面恢复；`recovery/` 分离 chunk/auth 错误、重载哨兵和空白渲染 watchdog
  entries/     - App、Settings 与 OAuth 等构建入口
  pages/       - 页面入口与浏览器协调；`room/` 和 `contacts/` 各自分离页面控制器与 URL 协调
  features/    - 领域功能实现；`home/home-directory-resource.ts` 负责侧栏与通知共用的聊天目录，`home/hero/` 分离 ASCII Hero 的视图、Canvas 生命周期和粒子模型，`home/notifications/` 分离通知投影、浏览器边界和 Room 协议，`home/sidebar/` 分离聊天/联系人入口、目录投影、未读聚合与 Room 命令，`agents/agent-detail-navigation.ts` 统一 Room 与联系人详情的 Agent 栏目信息架构，`agents/options/` 统一可编辑字段投影、mutation 参数、草稿、校验、自动保存调度与显式保存事务，`contacts/` 只提供目录、卡片和详情视图，`memory/catalog/` 负责 Agent 记忆目录请求与投影，`memory/document/` 分离文档作用域状态、实时资源、保存事务和视图，`conversation/room/workspace/controller/` 分离 Workspace Agent 作用域、文件资源、路径模型和命令，`conversation/room/workspace/view/` 分离文件列表布局、浏览器和弹窗，`capability/skills/` 负责技能市场及其状态域，`capability/connectors/` 按 catalog/detail/auth/controller 分离目录、详情、认证和命令，`capability/channels/` 按 catalog/connection/pairings 分离频道目录、连接状态机与 IM 配对，`capability/scheduled/controller/` 分离任务列表资源与写命令，`capability/scheduled/board/` 负责真实状态看板、必要信息卡片和建议空态，`capability/scheduled/dialog/` 按 form/schedule/resources 分离任务表单、调度规则与依赖资源，`capability/scheduled/pickers/` 统一时间列和锚定浮层，`conversation/shared/goal/` 负责 Goal 资源快照、命令和视图，`conversation/shared/session/` 统一 DM/Room 会话基础设施并由各投影定义窄 Session Source，`conversation/shared/timeline/` 负责时间线投影与窗口加载，`conversation/shared/timeline/scroll/` 负责跟随、锚定、动画和轮次 DOM 协议，`conversation/shared/todos/` 负责合并 TodoWrite、TaskCreate/List/Update 与运行时任务，`conversation/shared/composer/controller/` 负责 DM/Room 输入状态与动作协议，`conversation/shared/feed/` 负责 DM 轮次渲染及共享虚拟列表协议，`conversation/shared/message/item/` 按 controller 与 view/content/assistant/user 分离轮次投影、内容块、助手和用户视图，`conversation/shared/subagent/` 负责子智能体列表和只读线程资源，`conversation/room/dm/panel/` 负责 DM 页面模型与视图，`conversation/room/surface/header/` 保存 DM/Group 共用导航，`conversation/room/surface/mobile/` 分离移动端头部、会话 Sheet 和全屏 Overlay，`conversation/room/surface/layout/` 负责桌面分栏与右栏编排，`conversation/room/group/chat/panel/` 负责 Room 会话编排，`conversation/room/group/chat/feed/` 负责 Room 轮次渲染，`conversation/room/group/round/` 负责 Room Agent 轮次与 Thread 纯投影，`conversation/room/members/` 负责 Room 成员与设置表单，`conversation/shared/session-navigator/` 负责轮次导航，`settings/operations/` 负责角色受限的订阅运营与公共 Provider 管理，`settings/general/` 按 model/sections/components 分离通用偏好、模型与视图，`settings/personal/` 分离个人资料资源、头像/密码命令、密码规则和视图，`settings/shared/` 保存设置型表面的跨域共享 UI，`settings/provider-settings/` 按 `model/`、`actions/config/` 与其他窄动作分离 Provider 纯模型、字段联动、持久化、删除和模型命令
  features/conversation/shared/execution/ - 后端 WorkGraph 的唯一只读资源、状态投影、Composer 实时缩略图与 DM/Room 共用完整工作图 Surface
  config/      - `runtime-endpoints.ts`、`runtime-options.ts` 与 `conversation-policy.ts` 分离端点解析、当前作用域快照和固定会话策略；`desktop-runtime/` 按宿主配置、鉴权、OAuth 和生命周期协议分层
  hooks/       - 自定义 React Hooks；`agent/` 按动作、消息模型、会话、运行态和传输协议分层
  lib/         - 无业务状态的基础函数与协议客户端；根目录保存错误、头像和未知值等跨领域纯投影，`format/` 按展示值类型分离格式化规则，`api/` 按 core/agent/account/capability/conversation/settings 分离传输与领域协议，`websocket/` 按策略、心跳、单连接客户端、共享通道和 React 生命周期分层
  shared/      - 无业务所有权的 UI、认证 Context、i18n 和跨页面原语；`ui/` 按 button/form/display/list/navigation 分离基础交互职责，`ui/liquid-glass/` 分离能力探测、动画资源、滤镜链和组件装配，`i18n/catalog/` 按领域分离双语文案并逐分片校验键集合，`ui/markdown/` 统一 Markdown 渲染，`ui/mention/` 统一目标选择、文本匹配和插入，`ui/overlay/` 统一锚点定位与浏览器生命周期，`ui/menu/` 保存具体菜单语义
  store/       - Zustand 状态管理（agent + session 独立 store，room-navigation 按 Room 持久化标签集合、顺序与活动会话）
  types/       - 跨领域协议类型；`capability/scheduled-task/` 分离任务定义与运行结果，`conversation/message/` 分离附件、内容、实体和事件，`conversation/interaction/` 保存权限和用户问答协议
```

## 核心约定

- 组件 `PascalCase`，hooks `useXxx`，工具函数 `camelCase`
- 模块只导出跨文件消费的契约；文件内部使用的函数、常量和类型保持私有，禁止为潜在复用扩大公开面
- 跨领域协议声明归 `types/`，匹配算法和展示规则归消费层；API 通过 `types/api.ts` 共享 `ApiResponse<T>`
- Store 使用 Zustand persist middleware，数据持久化到 localStorage
- Agent WebSocket 信封校验与事件路由位于 `hooks/agent/transport/`，业务处理器不得回流到组件层
- Agent WebSocket 业务事件按 `transport/handlers/` 的消息、权限、重同步、Session 和作用域映射分域；权限事件在 `handlers/permission/` 分离未知载荷解码与状态副作用，当前 Session 守卫、字段回退和事件所有权不得重复实现
- Agent conversation 公共 Hook 只做领域装配；消息去重、ACK 失败和稳定事件分发分别归属 `message/`、`actions/` 与 `transport/`
- Agent Session 由 `hooks/agent/session/controller/` 分离身份迁移、后台/易失快照与加载上下文；总控制器不复制 React setter
- Agent 运行态由 `hooks/agent/runtime/` 按纯模型、易失快照和 React 状态分层；状态机实例不得暴露给编排层，`model/` 不得反向依赖存储或 Hook
- Agent 目录 Store 只保留静态目录与当前选择；运行态事件只在会话/工作区链路中消费，不回写 Agent 目录状态
- WebSocket 连接策略只由 `lib/websocket/socket-policy.ts` 定义；共享通道使用完整有效配置作为身份，业务消息不得进入离线队列；Room/DM 的 Session bind 由共享通道内部的逻辑租约统一引用计数并在重连后重放，单个组件 cleanup 不得直接解绑仍被其他消费者使用的 Session
- Workspace 会话标签由 `shared/ui/workspace/controls/conversation-tabs/` 分离纯模型、标签事务和单项视图；`store/room-navigation.ts` 按 Room 持久化完整打开集合、顺序与活动项，首次进入只打开恢复目标，历史会话只在用户显式选择后加入；活动标签必须属于打开集合，视图不得直接修正集合状态
- `shared/`、`lib/`、`store/` 与 `types/` 不得依赖 `features/`；应用壳层组合 Feature 时必须归入 `app/` 或专用导航 Feature
- `types/` 只声明跨层协议，不得导入 Config、Lib 或运行时投影；Agent 会话作用域键只由 `lib/conversation/agent-conversation-identity.ts` 计算
- API 客户端按 endpoint 所有权归入 `lib/api/{agent,account,capability,conversation,settings}/`，通用传输在 `core/` 按请求、响应、错误和鉴权事件拆分；消费者直接导入职责文件，不保留旧路径转发层
- 共享 UI 基础组件按 `button/`、`form/`、`display/`、`list/` 与 `navigation/` 分组；消费者直接导入职责文件，不恢复根级聚合出口
- Surface 搜索入口统一由 `UiSearchInput` 提供中性灰白底、hairline 边界及交互态；消费者只调整尺寸和布局，不得局部覆写背景、边框或阴影
- Light/Sunny 壳层以 `#f9f9f7` 为页面真相源，导航、目录、主画布依靠相邻中性灰阶分区；主侧栏外缘只绘制一根不透明 hairline，展开态从物理窗口顶端贯穿到底部，折叠态从 Header 底部开始以避开原生窗口按钮，内部 Dock 不再叠加竖线或外投影。Nexus 品牌蓝只用于发送、保存、创建、连接等主行动，以及焦点、运行态和明确选中模式；普通导航与次级工具保持黑白灰，teal 只表达次级数据/文件类型，红绿黄只表达危险、成功和警告
- 主侧栏品牌栏只保留 Launcher 字标与折叠控制；一级导航只承载聊天、联系人和能力，Nexus 主智能体以不可删除的默认 DM 固定在聊天目录顶部；底部统一承载设置、引导与按认证状态显示的退出，展开态将退出和常用入口分居两侧
- Liquid Glass 由专用 Hook 持有能力启用与 Web Animation 生命周期，Filter 视图只描述 SVG 资源链；组件 render 阶段不得写状态，消费者不得通过目录 barrel 导入
- 样式类名组合只由 `shared/ui/class-name.ts` 提供；时间、Token 和头像规则分别归 `lib/format/` 与 `lib/avatar.ts`，不得恢复混合 `lib/utils.ts`
- Agent Options 默认值、权限/工具目录、归一化和可编辑字段投影只由 `lib/agent-options.ts` 定义；Config、Settings、Contacts、Room 与编辑器不得跨 Feature 取规则
- 翻译文案按 `shared/i18n/catalog/{zh,en}/` 的同名领域分片维护；中文定义键集合，英文必须通过 `MessageSegment` 精确覆盖，不恢复巨型语言文件
- Room API 按纯模型、查询和命令拆分，目录失效事件归 `lib/conversation/`；API 不得读取 Store，Direct Room 跳转与缺失 Agent 恢复归 `features/navigation/direct-room/`
- `unknown` 错误到用户消息的基础投影只由 `lib/error-message.ts` 定义；Feature 保留领域默认文案和反馈结构，不复制同义包装函数
- 外部 Session 通道别名、标签与合成会话 ID 只由 `lib/conversation/external-session.ts` 定义，页面和标签视图不得复制解释规则
- 权限与问答协议归 `types/conversation/interaction/`；权限和未完成工具调用的共享匹配归 `lib/conversation/`，问答超时与系统事件展示规则归消息 Feature
- 会话消息协议按 `types/conversation/message/{attachment,content,entity,event}.ts` 分离；WebSocket 信封和通用事件结构直接使用生成协议，消费者不得通过根 `types` barrel 或 `data: any` 绕过领域解码。`delivery_mode` 明确区分可恢复的 `durable`、随 round 收口的 `ephemeral` 与只留在当前时间线的 `transient`；只有 durable 消息可进入后台缓存和未读
- SDK 工具输入与保留型配置对象只允许 `unknown` 值；具体工具 Feature 在消费入口校验字段，不得用断言或 `any` 把外部载荷伪装成完整领域对象
- 定时任务协议按 `types/capability/scheduled-task/{task,run}.ts` 分离任务定义与运行结果；只声明真实消费者需要的契约，不恢复未接入的状态、事件和日报镜像
- 引导浮层由 `shared/ui/onboarding/overlay/` 分离目标/卡片观察器、定位策略、贴纸模型和步骤视图；Portal 入口不得重新实现这些规则
- Tour 定义、Context 契约、Context 实例和 Provider 分别归 `tour-contract.ts`、`tour-context.ts` 与 `tour-provider.tsx`；消费者不得从 Provider 提取类型形成循环依赖
- 应用 Tour 目录和引导中心归 `features/onboarding/`；页面只注册当前 Tour 与锚点，跨页面导航、自动启动和目录投影不得下沉到 `shared/ui`
- Room 群聊面板只在 `panel/` 组合会话、Goal 与输入区模型；普通和虚拟消息流必须共用 `feed/group-conversation-round.tsx`，不得复制轮次分支
- DM 与 Room 只通过 `shared/session/use-conversation-session.ts` 串联运行时、滚动、历史和时间线；具体面板只装配业务模型
- DM/Room 滚动视口、历史提示、错误和浮动控制统一由 `shared/conversation-panel-layout.tsx` 渲染，不复制表面布局 class
- Room 与子智能体统一消费 `conversation/shared/thread/` 的 Thread 轮次契约和消息面板；共享域不得反向依赖 Room 私有目录
- 子智能体列表与线程复用 `shared/subagent/use-scoped-resource.ts` 的作用域请求协议；线程按资源和纯投影拆分为只读执行记录，公共 Hook 只做装配；Room 由私有适配层复用成员选择器并按任务 `host_agent_id` 过滤，共享域不得反向依赖 Room
- Room 主 Feed 与 Thread 共用 `room/group/round/round-agent-model.ts` 的 Agent 聚合状态；状态优先级不得在视图中重复推导
- Room 创建与管理弹窗只通过 `members/use-create-room-form.ts` 管理不变量，并以 `RoomDialogSubmission` 对象提交；视图组件不得在渲染期修正表单状态
- Room 成员的 `participation_paused` 是 Room 级持久调度状态：管理弹窗只维护成员草稿，页面命令只提交真实差异；后端暂停时收口该成员当前 slot 并闸住 queue、Agent wake、Goal continuation 与 WorkGraph dispatch，恢复后继续原样保留的工作，前端不得把它降格为一次性停止输出
- Home 侧栏与聊天通知只消费 `home-directory-resource.ts` 的共享目录快照；聊天完成订阅固定挂在 `AppLayout`，不得依赖宽屏侧栏是否渲染；bootstrap 请求、刷新排队和全局目录事件不得在消费者中重复实现；聊天执行态与待确认人工交互统一由 `home/room-activity-resource.ts` 按 `roomId` 短期投影，DM 与群组共用规则，聊天和联系人侧栏均不订阅 Agent runtime
- Home 侧栏只通过 `home/sidebar/` 组合聊天和联系人入口；Room/DM 基础投影与未读叠加必须独立缓存，视图不得直接调用 Room API 或拼通知键
- Group Room 完成事件须同时记录精确消息锚点与未读计数；Room 导航不得预先清除它们，只有对应 Feed 证明消息已进入视口后才逐条消费。DM 仍在进入会话时清理自身未读，未读计数和最后更新时间不能推断第一条未读 Agent 回复
- Home ASCII Hero 的 Canvas 资源只归 `home/hero/home-ascii-scene.ts`；异步字体与尺寸重建必须绑定代次，过期任务不得启动动画循环
- Agent Options 以 `agents/options/editor/agent-options-draft.ts` 的单一草稿为编辑真相；名称校验与保存完成必须同时匹配 Agent 作用域和草稿版本
- Agent Options 的可编辑字段和创建/更新参数只由 `agents/options/` 投影；Contacts 与 Room 不得复制 Options 字段表或 Agent 更新载荷
- Agent Options 业务弹窗归 `agents/options/dialog/`；`shared/ui/dialog/` 只提供无业务编辑器依赖的弹窗原语
- Agent 技能页由 `agents/options/components/skills/` 分离可取消列表资源、互斥安装命令、搜索投影和视图；异步结果必须匹配当前 Agent，状态机不得留在卡片组件
- AskUserQuestion 以按问题索引的原子回答草稿为唯一交互状态；工具作用域、结果恢复和提交互斥由 question controller 管理，item/view 不解析轮次协议；Composer 是唯一外层 task surface，选项只使用行级选择态，不恢复嵌套卡片
- AskUserQuestion 的未知工具输入只由 question model 校验并归一化；camelCase 兼容停留在解析入口，内部契约只保留 `multi_select`
- 消息文本协议、时间格式和消息项投影分别归属 `message-content-model.ts`、`message-time.ts` 与 `item/message-item-projection.ts`；DOM 测量和活动状态不得进入通用 helper
- 消息项控制器返回按 User/Assistant 及视觉职责分组的具体状态；视图在消费侧定义窄契约，禁止恢复跨视图的扁平 `MessageItemState`
- 记忆列表请求必须绑定 Agent，文档加载与保存必须绑定 `agentId:path`；SDK 实时内容优先于旧 HTTP 响应，保存完成不得覆盖更新的草稿
- Memory 目录规则只由 `memory/catalog/` 的纯模型与单一描述表定义；正文资源和保存互斥分别归 `memory/document/` 的独立 Hook
- Workspace 文件快照与写命令按 Agent 作用域隔离；同 Agent 的后发刷新使先发请求失效，外部打开 Agent 信号只消费一次
- Room 页面数据资源必须绑定当前 `roomId`；模型只做投影，命令只返回当前作用域结果，会话快照只通过专用协议写回
- Room 页面私有控制器归 `pages/room/controller/`，浏览器协调归 `pages/room/orchestration/`；领域 Feature 不读取路由，页面不解释服务端资源协议
- Room 成员管理由页面命令层绑定作用域并按“添加成员 → 更新设置 → 暂停/恢复变化 → 移除成员”的依赖顺序执行；Header 只提交完整表单对象，Surface 不传播成员增删、参与状态和设置更新的散装回调
- Contacts 页面使用互斥编辑状态，Agent 目录与 Agent 视角通讯客户端的联系人、Session、消息和命令归 `pages/contacts/controller/`，URL 选择与 Room 跳转归 `pages/contacts/orchestration/`
- 宽侧栏由 `features/navigation/sidebar/` 管理；展开与收起共用单一常驻壳层、固定 48px 一级导航 Dock 和系统操作，Dock 图标交互面与 32px 聊天头像同尺度，只有目录可见性与外层宽度变化，路由/Store 同步只留在控制器
- 能力侧栏归 `features/capability/sidebar/`；导航项由定义表投影，摘要刷新合并和窗口重验证只由专用资源 Hook 管理，业务行不得伪装成共享 UI
- 能力、设置与联系人等管理页面共用 `shared/ui/layout/workspace-content-layout.ts` 定义的铺满内容面，并由单一 `--workspace-content-gutter` 在 20–32px 间随屏幕平滑调整；正文、共享 Surface Header、Agent 内联详情和横向滚动区必须保持同一左右基线。页面用 `workspace-content-header.tsx` 统一标题、单句说明与动作；能力目录由 `features/capability/shared/capability-page-layout.tsx` 组合筛选、分区节奏、三列间距、可见条目边框与无品牌资源时的方形身份图标，工作循环条目按稳定 `slug` 复用公共数学曲线头像。普通目录在桌面统一使用三列，窄窗逐级收拢；定时任务正式看板保持四列，宽度不足时横向滚动而不折成两列。不得再用 Surface Header 重复“图标 + 能力名”，作用于搜索结果的来源模式进入筛选工具区，窄屏复用应用返回栏而不重复身份标题。目录行只展示标题、一行说明和一行元数据，完整步骤与技术字段进入详情或折叠区；详情页继续使用适合长文阅读的窄版心
- 工作循环的触发协议与统计值只由 `features/capability/loops/loop-presentation.ts` 投影为当前语言；目录和详情不得直接展示后端枚举或拼接固定英文单位
- 技能市场由 `features/capability/skills/controller/` 按目录、外部搜索、来源和操作拆分状态；Nexus 内置 Skill 的双语说明统一由 `lib/skill-description.ts` 做只读展示投影，并由目录、详情、Agent、Room 与 Composer 复用；能力页已安装、更新与社区结果共用 `features/capability/skills/shared/skill-directory-card.tsx`，但已安装目录卡只保留名称、说明和真实动作，来源、作用域、标签与启用位置进入详情；所有 Skill 资源头像只通过 `shared/ui/display/seeded-avatar.tsx` 按稳定名称生成跨目录、详情和预览一致、圆心固定的静态数学曲线身份；子视图只消费窄 Props，不得依赖完整控制器
- 频道连接与 IM 配对分别持有命令互斥入口；`channels/connection/login/` 独占扫码会话和串行轮询但复用连接命令锁，`channels/connection/view/` 按字段区、Footer 和展示投影拆分并由消费者定义窄接口；写操作后必须刷新当前服务端快照，视图不得复制协议字段别名
- 定时任务弹窗的表单和调度各自维护单一草稿对象，基础字段的目标/会话文案由纯模型投影，高级设置按字段职责组合；资源层按执行模式加载依赖并拒绝过期响应，Room 任务只允许绑定明确执行成员
- 定时任务时间选择器共用 `capability/scheduled/pickers/time-picker-column.tsx`，锚点浮层复用 `shared/ui/overlay/`，不得在 Daily/SingleRun 中复制选项按钮
- 定时任务目录只通过 `capability/scheduled/controller/` 读写任务；不得恢复混合 Heartbeat 的 Automation 控制器，命令结果必须先于后台刷新落地
- 定时任务运行历史由 `capability/scheduled/history/` 分离 Job 作用域资源、动作事务和纯视图；弹窗壳层不得直接请求 API 或维护单项命令状态
- Goal 面板只通过 `shared/goal/use-goal-controller.ts` 读写状态；资源快照必须绑定会话键，刷新拒绝过期响应，所有写命令共享互斥入口；状态条只显示实际 token 用量，预算只保留为编辑配置和运行限制，不投影为第二套用户用量
- 桌面运行时只通过 `config/desktop-runtime/index.ts` 暴露稳定门面，消费者不得读取宿主原始全局对象或复制 URL 协议判断
- 窗口手势面统一使用 `data-desktop-window-drag-region`。macOS 宿主以 4px 阈值仲裁短按与拖窗，让可见 Header、`/app` 主内容区顶部透明拖动面与原生 traffic lights 共面；Windows 使用 WebView2 原生 `app-region`，Header 空白区进入系统命中测试，标签、按钮、链接、编辑控件与显式排除项保持 `no-drag`。Windows 独立原生标题/菜单栏位于 WebView 上方，Web 不得为其预留右侧 caption 空域；除 macOS `/app` 的透明手势面外，页面不得重新添加顶部安全行或全宽点击遮罩。
- 根启动入口只编排运行时配置与渲染阶段；普通入口先加载受保护的运行时配置，OAuth 公开回调入口必须显式跳过该预取并直接渲染 token 交换页；失败视图、chunk/auth 恢复、一次性重载和空白 watchdog 各自拥有独立边界
- API/WebSocket 地址、用户作用域运行时快照和固定会话策略分别归 `config/runtime-endpoints.ts`、`config/runtime-options.ts` 与 `config/conversation-policy.ts`；配置层不得请求网络或依赖 Feature
- 认证 Provider 归 `app/auth/`，登录/登出后的运行时配置刷新只通过 `app/runtime-options-resource.ts`；`shared/auth/` 只暴露 Context 契约和消费 Hook
- Workspace Catalog 共享 UI 按卡片框架、内容结构、动作和图标容器分离；消费者直接导入职责模块，不恢复混合聚合出口
- Workspace Surface Header 固定为真实使用的单行布局，标题、导航和尾部动作按职责组合；工具栏动作从 Header 独立导入，不恢复无消费者的密度模式
- 技能详情按 route/controller/model/view 分离，详情资源用请求代次拒绝旧响应；更新和删除只复用市场命令的明确结果，不在视图重复调用 API
- 连接器目录卡片的尾部动作只由 `capability/connectors/catalog/connector-card-model.ts` 投影；已连接项必须提供可访问的断开入口并复用控制器断开事务，详情页保留同一动作；飞书云文档的连接入口统一先选择官方扫码或手工兜底，官方扫码可在飞书选择已有应用或创建新应用，桌面端通过原生宿主、Web 端通过收到 App ID 后的延迟弹窗尝试拉起当前用户授权链接，Web 被浏览器拦截时必须显示明确的主按钮，手工 App ID / Secret 不得成为断开后静默复用的固定状态
- Room 当前 Session 的 Agent execution 展示锚点只由 `hooks/agent/runtime/model/room-agent-execution-state.ts` 维护：首次批量恢复采用服务端 `display_order` 或 message/slot timestamp + index 建立 canonical 顺序，后续 permission、slot、stream、status、message fallback 证据统一换算到同一时间尺度，只能追加并接管既有节点；acknowledged tombstone 保持原 shell 但不再携带交互。精确停止由 runtime 易失层按 `agent_round_id` 维护 stopping，并由 terminal event / `interrupt_ack` 幂等收口；Timeline、Room Feed 与 Thread 只消费该投影，不得按完成顺序重排
- MessageItem 的 Assistant 内容模式只由 `features/conversation/shared/message/item/message-item-projection.ts` 的穷尽策略同时选择 direct/final 正文 surface 与 pending interaction owner；DM 与 Room 的人工介入统一由 Composer 原位替换输入壳，消息和 Thread 只保留不可操作的等待证据。DM live/archive 的最终正文必须保持同一 React 子树；`view/content/content-renderer-model.ts` 让 live 空文本先挂载 Markdown 身份以承接首批流式正文，历史空文本不得占位
- Composer 由 `features/conversation/shared/composer/controller/` 分离草稿、分阶段消息投递、Goal/Loop、有序键盘守卫，以及输入/运行时/模式/动作视图投影；未发送的正文、图片/文件附件、Message/Goal 模式、Room Goal 负责人和 Mention 目标组成单一草稿胶囊，按包含 Session ID 的 Room/DM 内存作用域隔离，切换 Session 恢复各自待发送状态且不同聊天互相隔离；本地协议派发后立即认领并清空原修订草稿，ACK 失败只在没有新输入时恢复；textarea 高度只由浏览器真实排版在正文、原生 input/IME 与宽度变化时重测；输入历史仍按不含 Session ID 的逻辑聊天作用域共享；`components/{footer,pending-queue,loop-picker}/` 分别拥有展示和局部交互，面板只装配子域
- Composer 上方 WorkGraph 的展开宽度由活动 Dock 的本地空间约束，DAG 按自身实测宽度压缩层间距，只有低于最小可读间距时才横向滚动；禁止再用全窗口宽度推断被侧栏和辅助面板挤压后的聊天栏尺寸
- 会话底部工作区由 `conversation/shared/conversation-panel-layout.tsx` 统一组合为一个以 Composer 为底座的向上工作栈：Goal/告警紧贴 Composer 形成第一层，权威 Execution 存在时由 WorkGraph 胶囊占据工作栈顶边并替代 legacy Task，Task 仅在 `executor_agent_id + agent_round_id` 精确命中 WorkAttempt 时展开于对应节点内部；否则当前会话 Task 继续以“当前步骤/总步数 · 当前摘要”显示，缺失关联键不得猜挂。普通聊天和裸 `@` 不得因参与人数被推断成 Plan。回到底部在同一行相邻显示且进程缺席时单独居中。透明 Dock 与中间包装不接收指针，只有真实按钮拥有局部热区；禁止再用透明 runway 拉开 Goal 与 Composer。只有进程或回到底部控件真实可见时才在消息尾部保留避让，隐藏时不制造空白；控件显隐和展开不得改变阅读 viewport 高度。进程数据沿 Room/DM 面板模型进入共享视图，不在 Room Surface 顶部另设状态条。Composer Footer 使用输入壳容器宽度收敛动作：宽壳居中显示 `Powered by Nexus`，窄壳隐藏品牌标注并把空间还给功能控件；窄壳 Goal 模式必须重排为两行并保留负责人、取消和提交动作，不以全窗口断点推导壳内密度
- Composer 附件只由 `shared/composer/attachments/` 的有序规则表分类并生成文件选择过滤；剪贴板先投影为明确动作，整批校验必须先于上传，DM/Room 必须提供窄上传目标
- 停止动作按执行所有权归属：DM 可由 Composer 提供当前会话 `onStop`；Room Agent execution 常驻控制条通过自身 `agent_round_id` 定向中断，Composer 的“全部停止”只聚合点击瞬间仍 active 且尚未 stopping 的精确目标并逐个发送同一命令，禁止退化为无目标 session interrupt
- General 设置由 `features/settings/general/` 统一编排；默认模型值直接派生自用户偏好和 Provider 默认值，不维护镜像选择状态
- 设置目录由 `features/settings/settings-navigation-model.ts` 定义，主应用侧栏与独立设置窗口必须复用 `settings-sidebar-navigation.tsx`；当前分区只由 URL 查询参数派生，不维护第二份选中状态；运营分区只对非桌面端 owner/admin 暴露，旧 `/operations` 入口必须收敛到设置目录
- 运营能力归 `features/settings/operations/`，可以组合设置域内的 Provider 与共享视图；不得恢复与 `settings` 双向依赖的顶层 `features/operations/`
- Personal 设置只通过 `features/settings/personal/use-personal-settings-controller.ts` 读写资料；密码规则由纯模型的有序规则表定义，区块视图不得直接调用 Auth API
- Provider 由 `features/settings/provider-settings/workspace/` 管理原子状态与请求代次，`actions/config/` 和 `actions/model/` 分离配置及模型事务，目录、格式和能力标志只由纯展示模型投影
- CC Switch 本地 Provider 导入归 `features/provider-imports/cc-switch/`；初始化向导与 Provider 设置只组合桌面入口、默认模型策略和同步完成回调，不复制预览、选择或同步状态机
- Agent 身份页由 `features/agents/options/components/identity/` 的单一布局结构组合；资料、标签和模型选择各自拥有窄接口，待添加标签草稿必须绑定编辑作用域
- 通用 Markdown 只归 `shared/ui/markdown/`；Conversation 的 `message/markdown-renderer.tsx` 只解释消息文件产物协议，不得成为其他 Feature 的渲染入口
- 通用 Mention 只归 `shared/ui/mention/`；目标分类和标记由消费者投影，共享视图不得解释 Agent 或 Room
- 锚定浮层共用 `shared/ui/overlay/` 的定位、Portal 和关闭生命周期；Select/MultiSelect 在 `shared/ui/menu/` 复用内部开关、触发键盘协议和 listbox 框架，ActionMenu 保持外部受控，消费者直接导入具体组件
- 全局反馈只通过 `shared/ui/feedback/feedback-banner-viewport.tsx` 展示当前单条状态；tone 视觉与时长归纯定义表，业务消费者不得恢复单元素 Stack 数组
- Launcher 按 `console/` 与 `hero/` 分离 API/导航和视觉/输入；服务端动作使用完整分发表，Hero 不直接访问领域 API
- Message item 的结构化内容关联只由 `view/content/content-renderer-model.ts` 建立；Assistant/User 视图不得再次扫描整轮内容或手写不完整的 Props 比较器
- Office 预览下载与载荷上限只由 `conversation/shared/editor/office-preview-resource.ts` 管理；文档预览的加载生命周期、DOM 归一化与视图分别归属 `document/` 下的 Hook、DOM 模型和视图模块
- DM/Room 虚拟消息流共用 `features/conversation/shared/feed/` 的容器测量与轮次导航协议；高度估算必须响应容器宽度变化
- DM/Room canonical 根轮次只从 `features/conversation/shared/timeline/timeline-model.ts` 派生；Room Feed 再由 `room/group/chat/feed/group-agent-timeline-model.ts` 把各 `agent_round` 投影为保留根因果关系的稳定时间线节点，Thread 仍使用 canonical root；`timeline/window-loader/` 分离候选选择、有限重试账本和调度，窗口加载必须用会话代次隔离在途请求
- Room 未读 Agent 队列只使用完成事件的精确消息身份、`room_seq` 和稳定 `room-agent-round` 节点；Agent 节点可能插入旧 root，标记必须以不改变测量高度的 overlay 随真实节点渲染，Composer 上方入口按目标相对视口显示向上或向下，static/virtual Feed 共用同一状态且不得改变 DM 的回到底部控件
- 对话滚动只通过 `features/conversation/shared/timeline/scroll/` 协调；面板不得复制底部阈值、RAF 动画、历史前插锚点或轮次 DOM 标记
- DM/Room Todo 只从 `features/conversation/shared/todos/` 的单遍轮次投影派生；计划、运行时任务和状态别名不得在面板中重复推导
- 会话导航由 `shared/session-navigator/` 分离时间线数据投影、刻度视觉模型、纯 DOM 定位和活动轮同步，`session-navigator/jump/` 分离目标、串行加载与落点确认；缺失窗口加载必须绑定会话键和请求代次，失效目标不得产生副作用
- 消息项由 `features/conversation/shared/message/item/controller/` 统一完成顺序、权限、过程链和最终回复投影，`controller/display/` 分离纯显示状态与展开生命周期；Assistant 视图按模型、内容、头部、过程和权限适配分工，DM live 的连续普通工具由 `item/process/dm-tool-run-segments.ts` 以首个 `tool_use.id` 稳定成段，未解析或新增长段保持展开，已解析段在叙事/final 恢复边界后折叠，Room 与人工交互工具不进入该压缩路径；User 视图按展示模型、头部、正文和编辑器分工，未匹配权限只进入 Composer 队列，不得在消息正文恢复独立操作面
- `MessageItem` 直接从 `message/item/message-item.tsx` 导入；消息目录不提供只做转发的聚合出口
- 消息内容块按 `blocks/{question,code,artifact,tool}/` 分域；Question 的卡片展示与草稿/提交控制分别归 `question/{card,controller}/`，跨消息项的工具名称与输入摘要归消息域 `tool-activity.ts`，Tool 的执行阶段与权限详情只由 `tool/tool-block-model.ts` 派生，头部交互由 `tool/header/` 的纯投影解释
- Artifact 文件和图片分别归 `blocks/artifact/{file,image}/`，路径解析与浏览器下载/桌面 reveal 只由 Artifact 根域实现，消息渲染器不得直接调用文件动作 API
- Room 桌面布局由 `features/conversation/room/surface/layout/` 分离 Header、辅助面板、Thread 和布局控制；移动端与桌面端共用 Surface 纯派生
- Room Group Header 按成员头像、指南菜单和主装配分离；异步弹窗状态必须绑定 Room 身份，不能跨路由复用布尔状态
- DM/Group Header 共用 `surface/header/` 的 Tab 定义与指南菜单；移动端只在 `surface/mobile/` 组合头部、会话 Sheet 和 Overlay，聊天主体必须复用 `room-chat-surface.tsx`
- 聊天渲染错误边界归 Room Surface，并以会话身份作为 reset key；错误状态和硬编码回退文案不得跨会话、跨布局复制
- Room 会话历史由 `features/conversation/room/surface/history/` 统一排序、外部 Session 能力、删除资格和标题编辑状态，基础协议层不保存展示专属规则
- 子智能体列表与线程资源必须绑定来源/任务作用域并拒绝旧请求写回；Room 调用者切换必须同步清理不属于新 Agent 的详情选择；任务详情只读展示 transcript，不提供发送或停止动作
- 环境变量统一使用 `VITE_*` 前缀，通过 `import.meta.env` 读取

## 配置文件

- `env.example` - 环境变量模板（开发/生产/域名）
- `vite.config.ts` - Vite 构建与别名配置
- `postcss.config.mjs` - PostCSS + Tailwind 4
- `tsconfig.json` - TypeScript 配置
- `Dockerfile` - 生产容器构建
