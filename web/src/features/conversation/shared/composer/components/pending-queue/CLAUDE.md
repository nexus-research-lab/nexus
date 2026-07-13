# pending-queue/

L6 | 父级: web/src/features/conversation/shared/composer/components

## 职责

- `composer-pending-queue.tsx`: 装配队列头部和消息列表
- `pending-queue-item.tsx`: 展示单条待发送消息及动作
- `pending-queue-model.ts`: 处理重排和消息展示投影
- `use-pending-queue-controller.ts`: 管理折叠、拖拽运行时、边缘滚动和串行命令

拖拽中的 DOM/动画帧状态只存在于 controller，消息行不直接操作共享引用。重排函数保持纯函数，视图只提交排序后的 ID。
