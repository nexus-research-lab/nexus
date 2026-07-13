# 消息投影

- `use-message-item-projection.ts`: 按内容排序、最终回复、权限、活动和输出组装阶段编排纯投影。
- `message-item-ordering.ts`: 投影可见内容块，关联系统事件并保持消息源顺序。
- `message-item-system-events.ts`: 过滤系统消息并映射稳定展示元数据。
- `message-item-final-projection.ts`: 按内容模式策略选择直接内容、过程和最终回复。
- `message-item-permissions.ts`: 精确匹配消息内权限请求并保留未匹配请求。
- `message-item-stats.ts`: 通过有序规则投影结果文案和统计字段。

本目录只负责从消息事实生成具体投影，不持有展开、复制、停止或 DOM 状态。
内容模式差异必须进入穷尽策略表或小型阶段函数，禁止在主 Hook 中恢复条件矩阵。
已有 `projectionFromOrderedEntries` 是有序条目到内容投影的唯一转换，局部编排不得重复实现索引映射。
