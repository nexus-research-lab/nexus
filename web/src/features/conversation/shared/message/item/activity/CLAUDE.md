# 消息活动领域

- `message-activity-state.ts` 定义活动状态契约，并集中工具与权限的基础分类。
- `message-activity-blocks.ts` 提供活动规则共用的类型安全反向块查找，不承载业务优先级。
- `message-live-activity.ts` 按权限、运行阶段和流式内容的优先级推导轮次级活动状态。
- `message-content-activity.ts` 从结构化内容投影推导块级活动状态，不依赖 React 或具体视图模型。

活动领域只接收消息协议和集合型投影；视图负责提供已消费块、已结束工具和隐藏工具集合，不得把 DOM 或组件状态引入本目录。
