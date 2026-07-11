# Workspace File Tree

跨 Room 与 Landing 复用的 Workspace 文件树。

## 职责

- `workspace-file-tree-model.ts`：文件层级和扩展名视觉规则的纯投影。
- `workspace-file-tree.tsx`：公共入口，只构建树和稳定动作对象。
- `workspace-file-tree-row.tsx`：递归行、展开状态和行内动作视图。

## 边界

- 文件树只消费 `WorkspaceFileEntry` 和调用方动作，不读取 Room 状态或调用 API。
- 扩展名规则使用数据表维护，不在视图中增加类型分支。
- 递归层只传一个稳定动作对象，避免每层扩散同组回调。
