# Agent Message

- `assistant-message-model.ts` 负责 Assistant 快照规范化、同消息合并、内容块身份和终态失败身份提取；内容专有变化与状态变化同等重要，不得因 `stream_status` 未变而丢弃。Provider 错误原文从最终正文去重，当前可靠性投影只返回 exact round/Agent round 与分类，不恢复旧的会话错误字符串扫描入口。
- `message-collection-model.ts` 负责消息唯一性、集合 upsert、时间排序和历史快照合并。
- `message-window-model.ts` 把驻留消息约束为一段连续 round 窗口加实时/固定尾部：按 round 数与估算字节预算从指定一端整轮淘汰，并输出窗口与最新内容之间唯一的 `gapAfterRoundId`。
- `stream-message-reducer.ts` 分离流式元数据投影和索引内容块更新，只把单条事件归约到消息集合；仅新建 live placeholder 可为首次出现的 text/thinking 写入本地首帧标记，历史/恢复消息不得补标；完整 message 快照提交前由 transport 同步 flush 已排队 stream event，终态快照进入集合后旧 event 也不得再改写其内容或 metadata。
- `use-agent-message-collection.ts` 是消息集合的 React 状态边界，所有写入统一执行 `message_id` 去重。
- 内容块身份通过类型解析表定义；新增 `ContentBlock` 类型时必须显式声明身份规则。
- 图片身份来源通过有序解析表声明；表中顺序就是跨快照合并优先级，不允许在合并函数内追加字段短路链。
- 历史、实时快照、流式 patch 和本地 optimistic 消息最终都必须满足 `message_id` 唯一。
- Assistant 进度必须单调：更新的完整 message 快照可补齐终态字段，但任何排队中的旧 stream event 都不能缩短正文、回滚状态或覆盖已经展示的终态 metadata。
- 用户输入行的身份是后端受理游标 `input_seq`：ACK、实时 durable 行、重连重放与历史快照中游标相同的行是同一行，只占一个节点；本合同生效前落盘的行没有游标，仍以 `message_id` 为身份。不得再按 `client_message_id` 集合或正文判断两行是否重复。
- 尚无游标的 optimistic 行只由点名其 `client_message_id` 的受理证据绑定一次（ACK、先于 ACK 到达的 Room durable 广播、ACK 丢失后的 DM 历史行），且必须在同一次集合更新中原位替换；随后到达的 ACK 或重放只做幂等收口。Room 历史不保存 `client_message_id`，ACK 丢失的 Room optimistic 行要等重放的 ACK 或 durable 广播收口。受理证据到达时，游标相同的行与被点名的 optimistic 行必须一起收口成一行：ACK 丢失后历史先带来游标行、optimistic 行仍并存时，不能只更新前者而留下后者。
- 配对后 durable 字段以服务端为准，本页发出的行保留 `client_message_id` 仅作为 Feed 节点 key，避免 acknowledged 用户气泡重新挂载；`message-collection-model.test.ts` 按 DM/Room 覆盖发送、刷新与重连下的重复和丢失。
