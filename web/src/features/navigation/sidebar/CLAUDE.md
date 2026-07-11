# 应用宽侧栏

- `sidebar-wide-panel.tsx` 只组合折叠/展开视图和唯一引导中心弹层。
- `use-sidebar-wide-panel-controller.ts` 独占路由、认证、通知与 Sidebar Store 装配。
- `sidebar-wide-panel-model.ts` 纯派生主 Tab、标签和 Nexus 激活状态。
- `use-sidebar-panel-resize.ts` 只管理拖拽边界，不读取 Store。
- `view/` 只处理折叠与展开布局，共用主 Tab、Nexus 入口和系统操作。

路由到主 Tab 的映射保持单一来源；业务引导统一由 `features/onboarding/` 提供。
