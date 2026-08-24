# DM Chat Panel

## 分层

- `dm-chat-panel.tsx`：客户端入口，只组合模型与视图。
- `dm-chat-panel-types.ts`：外部输入契约。
- `controller/`：按 Goal、会话、Composer 和纯投影阶段装配视图模型。
- `view/`：定义并渲染具体视图模型。

## 约束

- 运行时会话、历史、时间线和滚动统一进入 `shared/session/use-conversation-session.ts`。
- 导航、视口和滚动控件统一复用 `shared/conversation-panel-model.ts`。
- 视图不得调用 API 或重新推导消息分组；入口不得持有业务状态。
- 外部 Props 与内部 ViewModel 分离，消费者只依赖入口导出的 Props。
- Surface 已确定提供的 Agent 身份和布局必须保持必填，不在 Panel 内保存默认值兼容面。
- `embeddedEditor` 只裁掉导航、Goal、能力菜单与 Session 设置，消息、流式、Markdown、Tool、确认交互和 Composer 必须继续走标准 DM 投影；`visibleAfterUnixMilli` 是必填的可见边界，fork 继承的旧消息只供模型上下文使用，不进入嵌入时间线；`introduction` 只在 viewport 顶部投影本地接待话术和示例，不写入 transcript 或模型上下文；不得为嵌入式编辑器复制业务聊天气泡。
- 嵌入编辑器必须把接待说明作为新 Session 的顶部初始滚动锚点，并让 live 高度保护使用顶部对齐；未溢出内容从上向下连续增长，溢出后的 FOLLOW 和用户上滚后的 READING 不另建滚动实现。
