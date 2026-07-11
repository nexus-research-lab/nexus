# Assistant 消息视图

- `message-assistant-section.tsx`: 组合助手消息的头部、正文、过程和统计。
- `assistant-message-header.tsx`: 渲染身份、模型、时间与停止动作。
- `assistant-process-callchain.tsx`: 渲染过程折叠、过程内容和生成文件。
- `pending-permission-list.tsx`: 把未匹配权限请求适配为待确认工具块。

本目录只消费控制器已经推导出的显示状态；不得重新排序消息、匹配权限或选择最终回复。
Assistant 入口按 header、permissions、direct、process、final、activity、footer 和 layout 消费状态；子视图直接依赖领域类型，不索引上层聚合状态。
