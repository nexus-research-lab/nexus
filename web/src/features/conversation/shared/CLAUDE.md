# Conversation Shared

跨 DM 与 Room 复用的对话基础设施。

## 根模块

- `conversation-panel-layout.tsx`：会话面板通用布局和浮层控件。
- `conversation-panel-styles.ts`：统一消息流、助手正文、告警与 Composer 的桌面宽度阶梯。
- `conversation-panel-model.ts`：把共享会话控制器和面板环境投影为 Frame、导航、视口和滚动控件模型。
- `use-conversation-panel-environment.ts`：统一读取布局模式和 Provider 告警状态；用户头像已退出消息渲染契约，不得沿面板链保留无消费字段。
- `use-conversation-snapshot-reporter.ts`：按会话作用域报告稳定快照，并统一活跃时间、当前已加载消息计数与显式 `has_user_input` 投影；只有可见且非 synthetic 的 user 消息属于用户输入，消息计数不得代替该事实。Conversation scope 切换后的首个 effect 必须跳过，且消息 identity 必须属于当前 scope，防止上一会话的消息集合在清空前污染新 draft。
- `conversation-reliability-notice.tsx`：只消费结构化可靠性快照，在 Composer 状态栈投影连接恢复、Provider retry，以及完整说明问题、当前影响和安全下一步的用户级失败文案；不渲染内部详情。
- `read-resource-reliability-notice.tsx`：为外部 Session、round index 等只读资源用自然文案说明发生了什么、已有内容是否受影响和接下来能做什么；提示保持可见，不抢焦点、不自动重试。
- `editor/text/` 与 `editor/workspace-file-preview-panel.tsx`：文本草稿、加载/保存响应和编辑入口按 exact Agent + path 隔离；文件切换立即重建预览，未成功读取不得显示或保存上一文件内容。
- `slash-command-presentation.ts` 与 `slash-command-token.tsx`：识别消息开头的通用 `/<command>`，为 Composer 镜像和用户消息提供同一轻量命令标签，不改写草稿或持久化正文。
- `execution/`：读取后端安全的 managed `ExecutionView`，以同一 WorkGraph 在 DM/Room 投影目标、Plan revision、Work Item、依赖、Assignment、Attempt、Submission 与 Acceptance；planless/runtime-only Graph 不属于这个公共资源，也不得填充固定工作图入口的 Surface 或触发 Agent Dock。

## 约束

- 共享层只承载 DM 与 Room 语义完全一致的结构，不吸收各领域的差异字段。
- 纯投影不得持有 React 状态或调用领域 API。
- 可靠性展示按 transport recovery、Provider retry、用户级 failure 的优先级投影；分类与恢复证据归 `hooks/agent/reliability/`，视图不得解析错误文本或关联 ID。failure 以持久 polite status 完整说明问题、已有内容或当前输入的影响和安全下一步，不得把 `delivery_unknown` 简化成普通“重试”。
- 具体 Feed、Goal、Execution 和 Composer 模型由各自领域定义。
- active Plan 且包含 Work Items 的 managed Execution 存在时替代 legacy Todo 浮动进程；planless Execution、runtime-only round、普通聊天、裸 `@` 通信与原生 Todo 继续使用原路径，视图不得把 Goal、参与人数、mention 或运行节点推断成 Plan。
- 常规桌面保持紧凑阅读宽度；超宽屏只按共享阶梯放宽消息轨道和 Composer，助手正文使用更小的可读上限，禁止各消费面自行复制宽度断点。
- 会话底部工作区以 Composer 为底座形成单一向上工作栈：Goal/告警是紧贴 Composer 的状态层，Task 与回到底部共享工作栈顶边的居中浮动层。Task 存在时占中心主位，回到底部作为同一行相邻圆形动作；Task 缺席时回到底部单独居中。不得为浮动层在 Goal 与 Composer 之间插入透明 runway，工作栈中的上层组件应逐级收窄并保持清晰层级；Composer 上缘羽化仅在它直接接触消息区时生效，一旦 Goal 或告警占据状态层就必须收回到输入壳内，不得覆盖状态组件。Dock 容器和中间包装不得接收指针，只有 Task 与回到底部的真实按钮热区接收；仅在控件真实可见时，阅读 viewport 尾部才保留 56px 避让，隐藏时不得制造空白。控件显隐与 Task 展开不得改变 viewport 高度。
- Session 导航只能绝对定位在 `ConversationPanelViewportArea` 内，不得用 Composer 猜测高度设置固定 bottom；Goal、附件或多行输入改变底部区域时，导航可用高度必须自动跟随真实 viewport。
- 主消息 viewport 保留 `tabIndex={-1}` 供程序化导航，但显式移除浏览器原生轮廓；Safari 不得在滚动区底边绘制跨栏蓝线。
- 回到底部入口的可见文案和可访问名称必须跟随当前界面语言，不得在共享按钮中保存固定中文；Room 不得向该控件注入未读定位分支。
- 更早消息加载状态属于 DM 与 Room 共用 viewport，必须从会话目录按当前界面语言投影，不得在共享布局中保存固定中文。
- Round index 读取失败时必须保留同 Session 的最后成功索引并继续 indexed 模式；只有成功返回的空索引才能进入 legacy 历史模式，scope 或权限失效必须清除旧索引。
- Workspace 文本草稿只能属于创建它的 exact Agent + path；切换 scope 时旧异步响应、实时内容和保存结果都必须被栅栏隔离，首次读取失败不得退化为旧文件预览或可编辑空白文件。
- 共享 WebSocket 和 Room snapshot 必须捕获挂载时的 auth owner generation；身份 reset 后、React cleanup 前到达的旧事件不得写入 Room context、消息状态或 Workspace Live Store。
