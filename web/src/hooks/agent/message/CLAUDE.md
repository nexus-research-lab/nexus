# Agent Message

- `assistant-message-model.ts` 负责 Assistant 快照规范化、同消息合并、内容块身份和 durable 错误展示去重；内容专有变化与状态变化同等重要，不得因 `stream_status` 未变而丢弃。错误 result 已作为最终正文时不再投影第二个系统气泡。
- `message-collection-model.ts` 负责消息唯一性、集合 upsert、时间排序和历史快照合并。
- `stream-message-reducer.ts` 分离流式元数据投影和索引内容块更新，只把单条事件归约到消息集合；仅新建 live placeholder 可为首次出现的 text/thinking 写入本地首帧标记，历史/恢复消息不得补标；完整 message 快照提交前由 transport 同步 flush 已排队 stream event，终态快照进入集合后旧 event 也不得再改写其内容或 metadata。
- `use-agent-message-collection.ts` 是消息集合的 React 状态边界，所有写入统一执行 `message_id` 去重。
- 内容块身份通过类型解析表定义；新增 `ContentBlock` 类型时必须显式声明身份规则。
- 图片身份来源通过有序解析表声明；表中顺序就是跨快照合并优先级，不允许在合并函数内追加字段短路链。
- 历史、实时快照、流式 patch 和本地 optimistic 消息最终都必须满足 `message_id` 唯一。
- Assistant 进度必须单调：更新的完整 message 快照可补齐终态字段，但任何排队中的旧 stream event 都不能缩短正文、回滚状态或覆盖已经展示的终态 metadata。
- Room durable user 广播携带 `client_message_id` 时，必须在同一次集合更新中原位替换 optimistic 节点；随后 ACK 只做幂等收口。
- `client_message_id` 作为受理关联字段写入 durable 用户历史；当前页面合并随后到达的历史快照时仍须保留本地 visual identity，避免 acknowledged 用户气泡重新挂载。
