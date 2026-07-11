# DM Chat Panel

## 分层

- `dm-chat-panel.tsx`：客户端入口，只组合模型与视图。
- `dm-chat-panel-types.ts`：外部输入契约。
- `use-dm-chat-panel-model.ts`：装配 Goal、Composer、快照和共享会话控制器。
- `dm-chat-panel-view.tsx`：定义并渲染具体视图模型。

## 约束

- 运行时会话、历史、时间线和滚动统一进入 `shared/session/use-conversation-session.ts`。
- 视图不得调用 API 或重新推导消息分组；入口不得持有业务状态。
- 外部 Props 与内部 ViewModel 分离，消费者只依赖入口导出的 Props。
