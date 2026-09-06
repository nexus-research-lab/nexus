# Connector Detail

- 共享状态模型统一解释连接状态、OAuth 应用动作、配置错误和主动作；详情模型只处理认证标签 key 与能力顺序。详情状态、动作、准备说明和辅助标签消费当前语言目录，服务端能力正文与外部应用真实菜单名称保持原样。
- `connector-detail-header.tsx` 只负责投影 Connector 身份内容和动作资格，`connector-detail-content.tsx` 负责状态、能力与文档；二级页内容轴和导航由 `CapabilityDetailPage` 统一持有，对象身份几何由 `CapabilityDetailIdentity` 持有，通用协议原理不作为每个 Connector 的正文重复展示。
- 返回动作必须复用 `UiButton`，身份、说明、事实与准备提示必须选择 App Typography 角色；不得在 Connector 详情重新定义按钮圆角、焦点、字号或状态胶囊。
- 窄屏返回语义由应用页面 Header 提供；桌面详情导航必须复用能力域唯一二级 Header，与原生窗口控件中线对齐。
- 详情入口只协调共享 `UiResourceState` 与当前能力弹窗；保留加载、失败、缺失和带旧详情的刷新失败优先级，仅显式重试/返回触发动作，切换 Connector 身份清除旧能力预览。
- 飞书云文档详情仅在当前 owner 的活动连接持有 OAuth 应用配置时显示“更换飞书应用”；动作打开统一连接方式弹窗，官方扫码可选择已有应用或创建新应用，手工 App ID / Secret 仅作兜底。普通断开也必须清除用户授权和 owner 级应用凭据，避免后续静默复用固定 App ID。
- 多分支状态使用有序规则与映射表达，不在 JSX 中堆叠条件链。
- RichMail 连接准备说明使用 `UiPanel` 的 filled 表面，保留一个可执行主标题，阅读步骤使用 supporting、路径分组使用 metadata；长操作步骤是自然换行正文。连接事实按实际工作面宽度分列，原始端点允许换行；没有服务端有效期事实时不显示固定 Token 到期天数。
- 能力详情弹窗以能力名和正文为主；Header 自动为实际 Backdrop dialog 提供名称，业务只以隐藏说明关联 Connector 身份，不重复可见副标题或成功图标。能力项使用共享紧凑 ListRow，OAuth scopes 通过共享 `UiDisclosure` 默认折叠，正文和技术标识允许换行。
