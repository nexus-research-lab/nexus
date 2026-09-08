# Conversation Shared

跨 DM 与 Room 复用的对话基础设施。

## 根模块

- `conversation-panel-layout.tsx`：会话面板通用布局和浮层控件。
- `conversation-panel-styles.ts`：统一消息流、助手正文、告警与 Composer 的桌面宽度阶梯。
- `conversation-panel-model.ts`：把共享会话控制器和面板环境投影为 Frame、导航、视口和滚动控件模型。
- `conversation-empty-introduction.tsx`：在 DM/Room canonical timeline 为空时，以可见 Feed 为基准居中展示同一静态身份与建议面板；四条建议固定复用同构的纵向轻边框 `UiButton outline`，图标在左上、文案在左下并保持足够卡片宽度和高度，Room 文案还需平衡左右列的视觉重量；这些同层快捷入口默认透明且不加阴影，只以边框和 hover 底色反馈交互。身份图形允许在居中外框内做 1–2px 光学校正，但不得移动整组中心线；只在用户选择建议后发出普通消息，不持久化自身。
- `use-conversation-panel-environment.ts`：统一读取布局模式和 Provider 告警状态；用户头像已退出消息渲染契约，不得沿面板链保留无消费字段。
- `use-conversation-snapshot-reporter.ts`：按会话作用域报告稳定快照，并统一活跃时间、当前已加载消息计数与显式 `has_user_input` 投影；只有可见且非 synthetic 的 user 消息属于用户输入，消息计数不得代替该事实。Conversation scope 切换后的首个 effect 必须跳过，且消息 identity 必须属于当前 scope，防止上一会话的消息集合在清空前污染新 draft。
- `conversation-reliability-notice.tsx`：只消费结构化可靠性快照，在 Composer 状态层投影标题和一句说明；不渲染内部详情，只为消息结果确认和会话内容加载提供“刷新”，其余情况复用当前 Composer、权限卡或设置入口；提示骨架、tone 和动作状态复用 `UiInlineNotice`。
- `read-resource-reliability-notice.tsx`：为外部 Session、round index 等只读资源说明当前不可用内容和一个直接恢复方式；提示保持可见，不抢焦点、不自动重试，并与 Conversation failure 共用 `UiInlineNotice` 而不另写边框、圆角和按钮。
- `provider-unavailable-banner.tsx`：只负责 Provider 配置入口和弹窗生命周期；可见提示同样复用 `UiInlineNotice`，不得把配置恢复重新做成独立告警卡。
- `editor/text/` 与 `editor/workspace-file-preview-panel.tsx`：文本草稿、加载/保存响应和编辑入口按 exact Agent + path 隔离；文件切换立即重建预览，未成功读取不得显示或保存上一文件内容。
- `slash-command-presentation.ts` 与 `slash-command-token.tsx`：识别消息开头的通用 `/<command>`，为 Composer 镜像和用户消息提供同一轻量命令标签，不改写草稿或持久化正文。
- `execution/`：读取后端安全的 managed `ExecutionView`，以同一 WorkGraph 在 DM/Room 投影目标、Plan revision、Work Item、依赖、Assignment、Attempt、Submission 与 Acceptance；planless/runtime-only Graph 不属于这个公共资源，也不得填充固定工作图入口的 Surface 或触发 Agent Dock。

## 约束

- 共享层只承载 DM 与 Room 语义完全一致的结构，不吸收各领域的差异字段。
- 纯投影不得持有 React 状态或调用领域 API。
- Composer 状态层同时只显示一项，优先级固定为 Conversation reliability、round index 读取、Provider 配置、Goal；较低项保留状态，前项解除后再显示。Conversation 内部仍按 transport recovery、Provider retry、用户级 failure 投影；分类与恢复证据归 `hooks/agent/reliability/`，视图不得解析错误文本或关联 ID。新 submission 不能清除旧 `delivery_unknown` 或 Provider retry；只有 exact ACK、round 进展或 Session 对账才是恢复证据。failure 以持久 polite status 显示一句说明；只有可安全读取的恢复动作才显示按钮，不得把 `delivery_unknown` 简化成普通“重试”。
- 具体 Feed、Goal、Execution 和 Composer 模型由各自领域定义。
- active Plan 且包含 Work Items 的 managed Execution 存在时替代 legacy Todo 浮动进程；planless Execution、runtime-only round、普通聊天、裸 `@` 通信与原生 Todo 继续使用原路径，视图不得把 Goal、参与人数、mention 或运行节点推断成 Plan。
- 常规桌面保持紧凑阅读宽度；超宽屏只按共享阶梯放宽消息轨道和 Composer，助手正文使用更小的可读上限，禁止各消费面自行复制宽度断点。
- 会话底部工作区以 Composer 为底座形成单一向上工作栈：Goal/告警是紧贴 Composer 的状态层，WorkGraph、Room 协作与 Task 共用一个 32px 视觉基线的居中活动层；回到底部始终保持同一中心轴，有活动状态时悬在其上方，没有活动状态时单独居中。生成中该入口显示三点波浪，输出结束后改为向下箭头，不得因运行态切换位置或尺寸。不得为浮动层在 Goal 与 Composer 之间插入透明 runway，工作栈中的上层组件应逐级收窄并保持清晰层级；Composer 上缘羽化仅在它直接接触消息区时生效，一旦 Goal 或告警占据状态层就必须收回到输入壳内，不得覆盖状态组件。Dock 容器和中间包装不得接收指针，只有活动状态与回到底部的真实按钮热区接收；仅在控件真实可见时，阅读 viewport 尾部才保留 56px 避让，隐藏时不得制造空白。控件显隐与 Task 展开不得改变 viewport 高度。
- Session 导航只能绝对定位在 `ConversationPanelViewportArea` 内，不得用 Composer 猜测高度设置固定 bottom；Goal、附件或多行输入改变底部区域时，导航可用高度必须自动跟随真实 viewport。
- 主消息 viewport 保留 `tabIndex={-1}` 供程序化导航，但显式移除浏览器原生轮廓；Safari 不得在滚动区底边绘制跨栏蓝线。
- 回到底部入口的可见文案和可访问名称必须跟随当前界面语言，不得在共享按钮中保存固定中文；Room 不得向该控件注入未读定位分支。
- 更早消息加载状态属于 DM 与 Room 共用 viewport，必须从会话目录按当前界面语言投影，不得在共享布局中保存固定中文。
- Round index 读取失败时必须保留同 Session 的最后成功索引并继续 indexed 模式；只有成功返回的空索引才能进入 legacy 历史模式，scope 或权限失效必须清除旧索引。
- Workspace 文本草稿只能属于创建它的 exact Agent + path；切换 scope 时旧异步响应、实时内容和保存结果都必须被栅栏隔离，首次读取失败不得退化为旧文件预览或可编辑空白文件。
- 共享 WebSocket 和 Room snapshot 必须捕获挂载时的 auth owner generation；身份 reset 后、React cleanup 前到达的旧事件不得写入 Room context、消息状态或 Workspace Live Store。
