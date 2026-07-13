# Connector Model

- 本目录保存 catalog 与 detail 共同消费的纯领域状态，不读取 React 状态或触发请求。
- `connector-state-model.ts` 是连接状态、主动作、OAuth 应用动作和配置错误的唯一解释入口。
- catalog 与 detail 可以基于共享状态继续生成自身展示模型，但不得重新判断原始连接器字段。
- 状态选择使用带兜底项的有序规则；新增状态时必须同时检查列表动作和详情动作。
