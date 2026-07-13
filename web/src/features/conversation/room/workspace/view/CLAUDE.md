# Workspace View

Room Workspace 的纯视图与布局边界。

## 职责

- `use-workspace-file-list-layout.ts` 管理文件列表宽度与拖拽监听。
- `workspace-file-browser.tsx` 渲染目录工具栏、错误、空状态和文件树。
- `workspace-dialogs.tsx` 渲染创建、重命名、删除弹窗并连接右键菜单。
- `workspace-context-menu.tsx` 用动作数据投影右键菜单，不解释命令结果。

## 边界

- 视图只定义自己需要的窄接口，不导入完整控制器类型。
- 文件浏览器与弹窗只接收主控制器对应的 `browser` / `dialogs` 控制面。
- 视图不直接调用 Workspace API，不推导 Agent 作用域。
- 跨 Room 与 Landing 复用的文件树归 `shared/ui/workspace/tree` 所有，Room 不得反向暴露私有视图。
