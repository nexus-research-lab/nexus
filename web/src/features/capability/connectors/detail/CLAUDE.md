# Connector Detail

- 详情模型统一解释连接状态、OAuth 应用动作、配置错误、主动作和能力顺序。
- `connector-detail-header.tsx` 负责身份、面包屑和动作，`connector-detail-content.tsx` 负责状态、能力与文档。
- 详情入口只协调资源状态和当前能力弹窗；能力弹窗独立渲染。
- 多分支状态使用有序规则与映射表达，不在 JSX 中堆叠条件链。
