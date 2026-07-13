# 通用布局原语

- 本目录只保留可跨页面复用的加载、Workspace 布局模型和面板拖拽入口。
- `panel-resize-handle.tsx` 只发出横向拖拽开始事件；宽度状态、边界和窗口监听归真实布局所有者。
- 应用路由壳层归 `app/layout/`；通用布局不得组合业务 Feature。
