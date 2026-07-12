# Agent Options 弹窗

- 弹窗只负责组合共享 Dialog、标题与编辑器窗口骨架，不自行维护 Escape、焦点或滚动锁。
- `agent-options-dialog-model.ts` 用关闭、新建、编辑判别状态投影标题，不接受可选字段矩阵。
- 字段状态和保存事务统一委托 `AgentOptionsDialogEditor`，不得在弹窗维护镜像草稿。
- 当前唯一消费者是 Contacts；直接导入具体组件，不提供无价值 barrel。
