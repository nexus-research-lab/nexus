# Assistant 消息视图

- `assistant-message-model.ts`: 声明消费侧窄状态，并投影 Agent 作用域与紧凑/展开布局。
- `message-assistant-section.tsx`: 只组合助手外壳、头部、正文和统计。
- `assistant-message-content.tsx`: 按活动、直接内容、过程、最终回复、警告和权限顺序组合正文段。
- `assistant-message-header.tsx`: 分离身份、模型、时间、外部动作与停止动作。
- `assistant-process-callchain.tsx`: 独立管理过程折叠、过程内容和收起态生成文件。
- `pending-permission-list.tsx`: 把未匹配权限请求适配为唯一的待确认工具块列表。

本目录只消费控制器已经推导出的显示状态；不得重新排序消息、匹配权限或选择最终回复。
Assistant 入口按 header、permissions、direct、process、final、activity、footer 和 layout 消费状态；子视图只接收职责内切片，不索引上层聚合状态。
未匹配权限独立于过程内容且只渲染一次；不得通过复用 ReactNode 或伪造过程可见性把权限挂入多个内容段。
