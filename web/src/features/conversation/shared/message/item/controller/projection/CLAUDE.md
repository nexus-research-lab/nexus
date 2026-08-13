# 消息投影

- `use-message-item-projection.ts`: 按内容排序、最终回复、Goal 完成收据、权限、活动和输出组装阶段编排纯投影，聚合 Assistant 快照上的去重记忆引用；header 身份取最终 assistant/result，不从首条过程消息误取。
- `message-item-ordering.ts`: 投影可见内容块，关联系统事件并保持消息源顺序。
- `message-item-system-events.ts`: 过滤系统消息并映射稳定展示元数据。
- `message-item-final-projection.ts`: 按内容模式策略选择直接内容、过程和最终回复；DM live 与 archived 共用稳定 final 正文 surface，内建 `show_widget` 连同匹配结果从过程提升到该 surface；Room result 修正文本文字时保留非文本块顺序，缺少正文槽位时在过程末尾补入。
- `message-item-permissions.ts`: 按 `request_id` 建立唯一 pending interaction；精确 tool 匹配只提供消息内的只读上下文关联，完整请求统一由 Composer 队列持有。
- `message-item-stats.ts`: 通过有序规则投影结果文案和统计字段。

本目录只负责从消息事实生成具体投影，不持有展开、复制、停止或 DOM 状态。
内容模式差异必须进入穷尽策略表或小型阶段函数，禁止在主 Hook 中恢复条件矩阵。
`resolveAssistantResponseSurface` 是跨 live / terminal 的正文身份真相源；DM 两种模式必须都解析为 `final`，禁止在终态把已挂载正文迁移到另一个 React 子树。
已有 `projectionFromOrderedEntries` 是有序条目到内容投影的唯一转换，局部编排不得重复实现索引映射。
