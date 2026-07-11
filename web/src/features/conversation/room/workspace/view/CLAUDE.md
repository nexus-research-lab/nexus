# Workspace View

Room Workspace 的纯视图与布局边界。

## 职责

- `use-workspace-file-list-layout.ts` 管理文件列表宽度与拖拽监听。
- `workspace-file-browser.tsx` 渲染目录工具栏、错误、空状态和文件树。
- `workspace-dialogs.tsx` 渲染右键菜单与创建、重命名、删除弹窗。

## 边界

- 视图只定义自己需要的窄接口，不导入完整控制器类型。
- 视图不直接调用 Workspace API，不推导 Agent 作用域。
