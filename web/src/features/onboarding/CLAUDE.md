# 应用引导域

本目录拥有跨页面 Tour 目录和引导中心业务编排；通用浮层、Portal 与持久化原语仍归 `shared/ui/onboarding/`。

- `tours/` 定义稳定 ID、锚点和步骤描述，不读取路由或 React 状态。
- `guide-center/` 管理目录投影、自动启动与跨页面导航命令。
- 页面只注册当前 Tour 并提供锚点，不复制引导中心导航规则。
