# Connector Auth

- 本目录负责 OAuth、Device Flow、直接凭证和连接前附加信息。
- 认证弹窗只持有表单状态，请求和反馈由 controller 负责。
- OAuth 跨窗口事件只传递结构化结果，不直接修改目录状态。
