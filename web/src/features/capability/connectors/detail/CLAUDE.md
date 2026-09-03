# Connector Detail

- 共享状态模型统一解释连接状态、OAuth 应用动作、配置错误和主动作；详情模型只处理认证标签与能力顺序。
- `connector-detail-header.tsx` 负责对象身份和动作，`connector-detail-content.tsx` 负责状态、能力与文档；二级页内容轴和导航由 `CapabilityDetailPage` 统一持有，通用协议原理不作为每个 Connector 的正文重复展示。
- 返回动作必须复用 `UiButton`，身份、说明、事实与准备提示必须选择 App Typography 角色；不得在 Connector 详情重新定义按钮圆角、焦点、字号或状态胶囊。
- 窄屏返回语义由应用页面 Header 提供；桌面详情导航必须复用能力域唯一二级 Header，与原生窗口控件中线对齐。
- 详情入口只协调资源状态和当前能力弹窗；能力弹窗独立渲染。
- 飞书云文档详情仅在当前 owner 的活动连接持有 OAuth 应用配置时显示“更换飞书应用”；动作打开统一连接方式弹窗，官方扫码可选择已有应用或创建新应用，手工 App ID / Secret 仅作兜底。普通断开也必须清除用户授权和 owner 级应用凭据，避免后续静默复用固定 App ID。
- 多分支状态使用有序规则与映射表达，不在 JSX 中堆叠条件链。
- 能力详情弹窗以能力名和正文为主，不重复 Connector 副标题或成功图标；能力项使用普通列表，OAuth scopes 默认折叠为技术信息。
