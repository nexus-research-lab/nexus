# Agent Options 弹窗

- 弹窗只负责 Portal、Escape 关闭、标题与编辑器窗口骨架。
- 字段状态和保存事务统一委托 `AgentOptionsDialogEditor`，不得在弹窗维护镜像草稿。
- 当前唯一消费者是 Contacts；直接导入具体组件，不提供无价值 barrel。
