# Connector Auth

- 本目录负责 OAuth、Device Flow、直接凭证和连接前附加信息。
- 认证弹窗只持有表单状态，请求和反馈由 controller 负责。
- `device-flow/` 独立拥有 Device Flow 弹窗、生命周期 Hook 和可取消轮询器。
- OAuth 应用弹窗模型统一生成重置身份、Callback URL、提供方文案和保存约束；区块视图不读取原始详情字段。
- OAuth 跨窗口事件只传递结构化结果，不直接修改目录状态。
