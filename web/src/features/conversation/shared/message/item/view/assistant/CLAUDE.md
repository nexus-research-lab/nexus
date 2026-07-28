# Assistant 消息视图

- `assistant-message-model.ts`: 声明消费侧窄状态，并投影 Agent 作用域与紧凑/展开布局。
- `message-assistant-section.tsx`: 只组合助手外壳、头部、正文和统计。
- `assistant-message-content.tsx`: 按活动、直接内容、过程、最终回复、警告和权限顺序组合正文段，并把已匹配权限传给实际承载工具块的内容段。
- `assistant-message-header.tsx`: 组合 32px、8px 圆角的头像与垂直居中的名称；展开态不混入时间和模型，紧凑态才显示它们，并保留外部动作与停止动作。
- `assistant-message-stats.tsx`: 负责结果统计、缓存后的模型、复制动作和流式游标，使用消费侧窄统计契约。
- `assistant-process-callchain.tsx`: 独立管理过程折叠、过程内容和收起态生成文件。
- `pending-human-interaction-list.tsx`: 把所有会阻塞 runtime 的未匹配请求投影成唯一、可直接处理的人工介入列表。
- `pending-human-question.tsx`: 负责独立请求与已匹配工具块共用的结构化回答、拒绝和兼容响应适配。

本目录只消费控制器已经推导出的显示状态；不得重新排序消息、匹配权限或选择最终回复。
Assistant 入口按 header、permissions、direct、process、final、activity、footer 和 layout 消费状态；子视图只接收职责内切片，不索引上层聚合状态。
权限只能落在实际可见的匹配工具块或独立人工介入列表之一；两条路径都必须保留完整响应能力且只渲染一次。
