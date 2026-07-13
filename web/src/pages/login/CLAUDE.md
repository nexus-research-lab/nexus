# Login Page

- `login-page.tsx` 只装配 bootstrapping、redirect 和 ready 页面状态。
- `login-page-model.ts` 负责站内重定向校验与认证页面状态投影。
- `use-login-page-controller.ts` 负责 Auth 请求、凭证草稿和提交反馈。
- `login-auth-panel.tsx` 负责禁用态与密码登录态的具体交互视图。

重定向必须先解析为同源站内路径；登录页、落地页和外部 origin 统一回到 Launcher，避免认证回环或开放重定向。
