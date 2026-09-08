# 应用引导域

本目录拥有跨页面 Tour 目录和引导中心业务编排；通用浮层、Portal 与持久化原语仍归 `shared/ui/onboarding/`。

- `tours/` 定义稳定 ID、锚点和步骤描述，不读取路由或 React 状态。
- `guide-center/` 管理目录投影、自动启动与跨页面导航命令。
- `provider-setup/` 提供从聊天表面启动的 Provider/默认模型初始化向导；向导只编排现有 Settings API，不替代高级 Provider 设置。保存 Provider、连接测试、默认选择是三个独立阶段：每次副作用前先写 owner-scoped 恢复记录，响应丢失后只用精确 Provider key、configuration version、非敏感配置指纹、测试时间或偏好快照对账，不能重复已完成阶段。恢复记录不得保存 API 密钥、Base URL、完整请求或 HTTP 身份；无法证明时保持只读并由用户明确开启新意图。可见结构固定为单栏 plain 选择、凭据和验证流程，不展示吉祥物侧栏、重复 Logo 或连接后的功能营销清单。
- 页面只注册当前 Tour 并提供锚点，不复制引导中心导航规则。
- Provider 目录初始读取使用共享 `lg` muted Spinner，连接验证阶段使用 `sm` primary Spinner；向导不得自行维护尺寸、颜色、旋转或 reduced-motion class。
