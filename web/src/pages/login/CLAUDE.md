# Login Page

- `login-page.tsx` 只装配 bootstrapping、redirect 和 ready 页面状态。
- 页面背景、品牌入口与宣传标题由 `features/access/access-page-frame.tsx` 和其 CSS 统一拥有，与 Setup 共用；登录页只组合自身插画、产品说明和凭证表单。
- `login-page-model.ts` 负责站内重定向校验与认证页面状态投影。
- `use-login-page-controller.ts` 负责 Auth 请求、凭证草稿和提交反馈。
- `login-auth-panel.tsx` 负责禁用态与密码登录态的具体交互视图；表单结构、输入、按钮、表面和文字层级分别复用共享 Field、Input、Button、Panel 与 Typography，页面只拥有布局和认证状态，不覆盖控件几何或复制浮层材质。
- `login-auth-panel.test.tsx` 通过具名字段、键盘提交、阻塞态和安全状态重读证明视图迁移不改变认证交互；禁用密码登录仍保留部署状态说明及刷新入口。
- `browser-tests/login.spec.ts` 在真实 `/login` 路由复用全站主题、语言与视口矩阵，验证表单几何、焦点和恢复；认证 API 必须全部拦截，不得向真实账户发送测试登录。
- `login-page.css` 只拥有登录页辅助插画投影；品牌图标归 Access，主操作继续服从共享 Button 的无阴影合同。
- 装饰背景上的表单统一选择 Panel `filled`，由共享面板 token 减弱背景线穿透；不添加页面专属透明度、阴影或 blur。

重定向必须先解析为同源站内路径；登录页、产品根路由和外部 origin 统一回到 Launcher，避免认证回环或开放重定向。
