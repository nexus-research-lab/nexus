# Conversation Shared

跨 DM 与 Room 复用的对话基础设施。

## 根模块

- `conversation-panel-layout.tsx`：会话面板通用布局和浮层控件。
- `conversation-panel-model.ts`：把共享会话控制器投影为导航、视口和滚动控件模型。
- `use-conversation-snapshot-reporter.ts`：按会话作用域报告稳定快照。

## 约束

- 共享层只承载 DM 与 Room 语义完全一致的结构，不吸收各领域的差异字段。
- 纯投影不得持有 React 状态或调用领域 API。
- 具体 Feed、Goal 和 Composer 模型由各自领域定义。
