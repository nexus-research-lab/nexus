# 应用布局

- `app-layout.tsx` 是路由壳层，负责保持应用导航常驻并承载子路由 Outlet。
- `mobile-app-route-model.ts` 是手机布局的信息架构真相源：聊天、联系人、能力为一级目录，其余业务路由为带返回栏的全屏二级页面。
- Room 在手机布局中使用自己的会话 Header；联系人、能力、设置等页面由 `mobile-app-page-header.tsx` 提供统一返回语义。
- 手机一级目录只显示占满窗口的目录壳，不挂载被挤窄的桌面 Outlet；目录壳内部保留带短标签的左侧 Dock 与右侧列表，进入 Room 后隐藏目录壳并改用返回导航。桌面仍保留侧栏与主内容双栏。
- 应用布局可以组合 Feature；通用 `shared/ui/layout/` 不得反向依赖 Feature。
- 无侧栏页面通过显式布局参数表达，不复制第二套 Outlet 骨架。
