# pending-queue/

L6 | 父级: web/src/features/conversation/shared/composer/components

## 职责

- `composer-pending-queue.tsx`: 装配队列头部和消息列表
- `pending-queue-item.tsx`: 展示单条待发送消息及动作
- `pending-queue-model.ts`: 处理重排、拖动身份、正文/附件和引导状态的纯投影；密度 padding 由上层 `composer-styles.ts` 唯一持有
- `use-pending-queue-controller.ts`: 管理折叠、拖拽运行时、边缘滚动和串行命令

拖拽中的 DOM/动画帧状态只存在于 controller，消息行不直接操作共享引用。重排函数保持纯函数，视图只提交排序后的 ID。
队列头、引导与删除动作统一使用共享 Button 的 `2xs`/`xs` 微型档位；本目录不得再手写 20/24px 圆角、hover、focus 或 disabled 配方。
