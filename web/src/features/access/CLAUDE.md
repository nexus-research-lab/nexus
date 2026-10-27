# Access page presentation

- `access-page-frame.tsx` 由 Login 与 Setup 页面消费，统一品牌入口、背景与响应式双栏；宽表单用于并排字段，普通表单用于单列凭证。
- `access-page.css` 是访问入口的唯一品牌背景、Logo 投影和宣传标题尺度所有者；普通标题、正文、表单和按钮仍使用共享 UI。
- 页面提供文案、插画、辅助说明和表单，不把认证状态、路由决策、凭证或初始化命令放进此领域展示层。
- 唯一工程合同见 `docs/specs/frontend-engineering-spec.md`；真实 Login/Setup 浏览器测试拦截所有认证与初始化请求，不访问真实部署。

- 品牌名遵循用户要求写为 Nexus，色彩与品牌背景几何保持共享访问入口所有权。
- `AccessPageFrame` 的 `<main>` 整体是桌面窗口手势面（`data-desktop-window-drag-region`，控件按全局规则 no-drag），入口页无应用壳顶栏也能拖动窗口；可选 `headerAction` 只占左上角并通过 `data-desktop-window-controls-leading` 让出 traffic lights，不铺全宽顶栏。
