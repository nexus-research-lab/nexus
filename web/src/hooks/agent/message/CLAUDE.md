# Agent Message

- `assistant-message-model.ts` 负责 Assistant 快照规范化、同消息合并和内容块身份，不处理集合顺序。
- `message-collection-model.ts` 负责消息唯一性、集合 upsert、时间排序和历史快照合并。
- `stream-message-reducer.ts` 只把单条流式事件归约到消息集合，不维护 React 调度或 WebSocket 生命周期。
- 内容块身份通过类型解析表定义；新增 `ContentBlock` 类型时必须显式声明身份规则。
- 历史、实时快照、流式 patch 和本地 optimistic 消息最终都必须满足 `message_id` 唯一。
