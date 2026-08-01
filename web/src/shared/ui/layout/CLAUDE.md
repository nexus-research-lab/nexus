# 通用布局原语

- 本目录只保留可跨页面复用的加载、Workspace 布局模型和面板拖拽入口。
- `workspace-content-layout.ts` 是能力、设置、联系人等管理页面内容面的唯一入口；正文铺满可用工作面，水平留白只由 `--workspace-content-gutter` 控制，并通过 `clamp(20px, 2vw, 32px)` 随屏幕平滑增长。业务页面不得再写私有页面边距或 `max-width`。
- 共享 Surface Header、Agent 内联详情和横向滚动区也必须复用同一 gutter；滚动区需要出血时使用共享负边距组合，不得复制断点数值。
- 普通目录条目复用共享响应式网格，桌面显示三列、窄窗逐级收拢；定时任务正式看板保持四列并在宽度不足时横向滚动。
- `workspace-content-header.tsx` 是管理页正文标题、单句说明与页面动作的唯一 Header。
- `panel-resize-handle.tsx` 只发出横向拖拽开始事件；宽度状态、边界和窗口监听归真实布局所有者。`gutter` 变体占据真实分栏间距，不渲染线条或拖手，只通过拖拽光标提供反馈。
- 应用路由壳层归 `app/layout/`；通用布局不得组合业务 Feature。
