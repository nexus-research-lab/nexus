# 应用布局

- `app-layout.tsx` 是路由壳层，负责保持应用导航常驻并承载子路由 Outlet。
- 应用布局可以组合 Feature；通用 `shared/ui/layout/` 不得反向依赖 Feature。
- 无侧栏页面通过显式布局参数表达，不复制第二套 Outlet 骨架。
