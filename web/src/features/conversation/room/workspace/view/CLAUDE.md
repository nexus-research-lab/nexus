# Workspace View

Room Workspace 的纯视图与布局边界。

## 职责

- `use-workspace-file-list-layout.ts` 持有文件列表宽度和两档边界，鼠标生命周期复用 `useMouseDrag`；调用方在上下堆叠或专注预览时停用拖动，恢复横向布局不自动续拖。零宽/未挂载容器不改变尺寸。
- `workspace-file-browser.tsx` 不重复渲染已由预览 breadcrumb 表达的目录标题，只在贯通顶栏右端保留无描边图标操作；目录操作复用预览 chrome 的按钮配方，不另设尺寸、颜色或交互状态。贯通顶栏贴合面板边缘，Agent 筛选器与其他 Room 辅助面板共享 12px 左起点和固定尺寸，内容区再独立恢复文件预览所需的横向留白。目录初始读取使用具名 WorkspaceLoadingState，空目录使用 SidebarEmptyGuide；刷新仍优先保留已有文件树，不复制空态图标卡或匿名 Spinner；桌面分栏线从内容区开始，不切断顶栏，专注模式在超窄窗口改为上下堆叠并停用横向拖拽。
- `workspace-dialogs.tsx` 渲染创建、重命名、删除弹窗并连接右键菜单；输入和删除弹窗统一消费控制器 isMutating，写入期间禁用重复提交与退出，名称规范化和结果处理仍属于命令控制器。
- `workspace-context-menu.tsx` 用动作数据投影右键菜单，主/子层分别用公共指针/侧向定位和同一 cascade-menu preset；Portal、模态隔离、外部指针与 Escape 统一交给 anchored-overlay-layer，行、分隔线、总高度及键盘由 shared/menu 拥有。子层按真实触发器对齐与翻转，列表超限内部滚动；穿越父子间隙保持打开，进入其他主项才切换，不复制 hover timer。打开时进入首个可用项，点击/右方向键进入“打开方式”，左方向键/Escape 逐层返回，显式退出归还打开前焦点，外部点击（含源区域）保留目标焦点。桌面端只投影系统应用清单，不保存固定应用目录、不解释命令结果。相关代码/行为回归不替代用户暂缓的视觉验收。

## 边界

- 视图只定义自己需要的窄接口，不导入完整控制器类型。
- 文件浏览器与弹窗只接收主控制器对应的 `browser` / `dialogs` 控制面。
- 视图不直接调用 Workspace API，不推导 Agent 作用域。
- 跨 Room 与 Landing 复用的文件树归 `shared/ui/workspace/tree` 所有，Room 不得反向暴露私有视图。
- 文件目录初始读取直接采用 WorkspaceLoadingState 的排版与 Spinner；“打开方式”应用列表使用共享 `md` muted Spinner，上传使用 Header 对齐的 `sm` Spinner；Workspace 视图不得自行维护尺寸、颜色、旋转或 reduced-motion class。
