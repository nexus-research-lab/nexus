# 应用引导域

本目录拥有跨页面 Tour 目录和引导中心业务编排；通用浮层、Portal 与持久化原语仍归 `shared/ui/onboarding/`。

- `tours/` 定义稳定 ID、锚点和步骤描述，不读取路由或 React 状态。
- `guide-center/` 管理目录投影、自动启动与跨页面导航命令。
- `provider-setup/` 提供从聊天表面启动的 Provider/默认模型初始化向导；向导只编排现有 Settings API，不替代高级 Provider 设置。可见结构固定为单栏 plain 选择、凭据和验证流程，不展示吉祥物侧栏、重复 Logo 或连接后的功能营销清单。
- 页面只注册当前 Tour 并提供锚点，不复制引导中心导航规则。
