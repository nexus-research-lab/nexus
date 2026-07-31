# Workspace File Tree

跨 Room 与 Landing 复用的 Workspace 文件树。

## 职责

- `workspace-file-tree-model.ts`：文件层级、Material 文件图标映射和单行选中/展开展示的纯投影。
- `workspace-file-tree.tsx`：公共入口，只构建树和稳定动作对象。
- `workspace-file-tree-row.tsx`：递归行、展开状态和窄职责的指示器、子树与行内动作视图。

## 边界

- 文件树只消费 `WorkspaceFileEntry` 和调用方动作，不读取 Room 状态或调用 API。
- 文件名与扩展名规则使用数据表维护，视图只消费已解析的 SVG 资源，不增加类型分支。
- 递归层只传一个稳定动作对象，避免每层扩散同组回调。
