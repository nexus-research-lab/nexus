# Workspace View

Room Workspace 的纯视图与布局边界。

## 职责

- `use-workspace-file-list-layout.ts` 管理文件列表宽度与拖拽监听。
- `workspace-file-browser.tsx` 不重复渲染已由预览 breadcrumb 表达的目录标题，只在贯通顶栏右端保留无描边图标操作；目录操作复用预览 chrome 的按钮配方，不另设尺寸、颜色或交互状态。贯通顶栏贴合面板边缘，Agent 筛选器与其他 Room 辅助面板共享 12px 左起点和固定尺寸，内容区再独立恢复文件预览所需的横向留白。组件同时渲染错误、空状态和文件树；桌面分栏线从内容区开始，不切断顶栏，专注模式在超窄窗口改为上下堆叠并停用横向拖拽。
- `workspace-dialogs.tsx` 渲染创建、重命名、删除弹窗并连接右键菜单。
- `workspace-context-menu.tsx` 用动作数据投影右键菜单；桌面端按系统返回的可用应用渲染“打开方式”，不保存固定应用清单，也不解释命令结果。

## 边界

- 视图只定义自己需要的窄接口，不导入完整控制器类型。
- 文件浏览器与弹窗只接收主控制器对应的 `browser` / `dialogs` 控制面。
- 视图不直接调用 Workspace API，不推导 Agent 作用域。
- 跨 Room 与 Landing 复用的文件树归 `shared/ui/workspace/tree` 所有，Room 不得反向暴露私有视图。
- 文件目录初始读取和“打开方式”应用列表使用共享 `md` muted Spinner，上传使用 Header 对齐的 `sm` Spinner；Workspace 视图不得自行维护尺寸、颜色、旋转或 reduced-motion class。
