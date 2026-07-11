# 消息领域

- `message-content-model.ts` 负责跨消息项、时间线和会话导航共享的文本协议清理与内容提取。
- `message-time.ts` 只负责消息时间的稳定格式化，不读取视图状态。
- `item/message-item-projection.ts` 定义消息项内部的有序条目、轮次和内容投影，不承载 DOM 或视觉规则。
- 单消费者逻辑留在拥有它的 controller/view；禁止重新建立聚合 helper 或通过根 barrel 暴露内部模型。
