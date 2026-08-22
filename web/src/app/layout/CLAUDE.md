# 应用布局

- `app-layout.tsx` 是路由壳层，负责保持应用导航常驻并承载子路由 Outlet。
- 可见 Header 通过 `data-desktop-window-drag-region` 接入 macOS 统一标题栏并由宿主仲裁短按与拖动；Windows 的拖窗止于独立原生栏，该属性在 WebView 内不产生非客户区，不得额外叠加透明拖动条。
- `/app` 在 macOS 保留不改变画布布局的 60px 透明拖动面，让主内容区顶部可拖窗；浏览器与 Windows 不显示该拖动面，首页画布从各自客户区顶部开始。
- macOS 窄窗口 Header 必须同时声明 `data-desktop-window-controls-leading`，让返回与标题内容避开原生 traffic lights；不得用业务断点复制固定 padding。
- `mobile-app-route-model.ts` 是手机布局的信息架构真相源：聊天、联系人、能力为一级目录，其余业务路由为带返回栏的全屏二级页面；设置页标题必须随 URL 中的当前分区变化，不能退回笼统的“设置”。
- Room 在手机布局中使用自己的会话 Header；联系人、能力、设置等页面由 `mobile-app-page-header.tsx` 提供统一返回语义。页面级动作通过 `mobile-app-page-header-actions.tsx` 与其 Context 投影到标题右侧，不得在正文顶部另起悬空工具行。
- 手机一级目录只显示占满窗口的目录壳，不挂载被挤窄的桌面 Outlet；目录壳内部保留带短标签的左侧 Dock 与右侧列表，进入 Room 后隐藏目录壳并改用返回导航。桌面仍保留侧栏与主内容双栏。
- 应用布局可以组合 Feature；通用 `shared/ui/layout/` 不得反向依赖 Feature。
- 无侧栏页面通过显式布局参数表达，不复制第二套 Outlet 骨架。
