# 会话消息协议

- `attachment.ts` 只描述用户消息附件和 Workspace 作用域。
- `content.ts` 只描述 Assistant 结构化内容块。
- `entity.ts` 只描述可持久化消息实体及其前端流式状态。
- `event.ts` 只描述运行事件载荷；通用 WebSocket 信封直接使用生成协议。
- 消费者直接导入职责文件，不建立聚合出口或恢复旧 `message.ts`。
