# Connector Catalog

- 本目录负责搜索、分类、分组和列表项展示。
- 列表组件由消费者定义窄 props，不接收完整控制器。
- 分类与分组保持纯函数，不读取 React 状态或发请求。
- `connector-card-model.ts` 将共享连接状态投影为列表徽标和尾部动作，卡片视图不解释原始状态字段。
