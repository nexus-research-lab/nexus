# 共享 Thread 面板

本目录只负责 DM、Room 和子智能体可复用的 Thread 消息轨道与布局。

## 职责边界

- `conversation-thread-model.ts` 定义与具体会话来源无关的轮次契约。
- `conversation-thread-panel.tsx` 统一消息渲染、跟随滚动、权限块和头尾插槽。
- 上游负责提供已过滤的消息、轮次、身份和能力动作；本目录不得调用领域 API。
- Room 与子智能体不得复制 Thread 面板结构或从对方的私有目录反向导入。
