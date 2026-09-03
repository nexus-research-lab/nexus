# Connector Auth

- 本目录负责 OAuth、Device Flow、直接凭证和连接前附加信息。
- 认证弹窗只持有表单状态，请求和反馈由 controller 负责。
- `device-flow/` 独立拥有 Device Flow 弹窗、生命周期 Hook 和可取消轮询器。
- `feishu/` 独立拥有飞书扫码主入口、手工 App ID / Secret 兜底表单与两阶段展示模型。
- `shop-domain/` 独立拥有 Shopify 域名规范化、一次性请求事务和输入弹窗。
- OAuth 应用弹窗模型统一生成重置身份、Callback URL、提供方文案和保存约束；区块视图不读取原始详情字段。
- OAuth 跨窗口事件只传递结构化结果，不直接修改目录状态。
- 连接弹窗统一使用 plain chrome：标题直接命名目标，正文只保留当前步骤、必填字段与必要的文档入口；不得展示 runtime/MCP 装配说明、装饰性凭证图标、教程卡或重复副标题。
- OAuth、Device Flow、直接凭证与本机配对共享 Dialog、Panel、Field、Typography、Badge 和 Spinner 所有权；各 Provider 只提供步骤与数据，不得重写字号、圆角或加载动画。
