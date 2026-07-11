# Workspace Surface

- `workspace-surface-header.tsx` 只组合单行身份、导航和尾部插槽；不存在真实消费者的布局模式不得保留在公共契约中。
- `workspace-surface-toolbar-action.tsx` 统一 Surface 工具栏动作外观，不依赖 Header 的布局实现。
- `workspace-header-layout.ts` 保存侧边栏与主内容区共用的高度基线，布局双方不得复制数值。
- `workspace-surface-scaffold.tsx` 只提供 Header 与主画布骨架；业务滚动、状态和命令留在调用方。
- 标题、标签和中部导航的可选组合在各自私有组件内收口，根 Header 不维护布尔状态矩阵。
