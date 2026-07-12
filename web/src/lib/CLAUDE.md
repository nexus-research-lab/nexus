# 前端基础库

- 根目录只保留跨领域复用且无业务状态的纯基础能力。
- `unknown-value.ts` 只提供未知值的结构读取原语；领域字段集合由消费者定义。
- `agent-runtime-status.ts` 统一跨页面 Agent 运行状态解码。
- `agent-options.ts` 统一跨 Config、Settings、Contacts、Room 与 Agent 编辑器复用的 Options 默认值、目录和纯投影。
- `settings/` 统一 Config 与 Settings 共同依赖的偏好值清洗和 Options 合并规则。
- `avatar.ts` 统一头像标识、图标编号范围和稳定 Room 默认头像。
- `format/` 按展示值类型保存无状态格式化规则，不建立聚合出口。
- API、会话与 WebSocket 等有明确协议所有权的能力归各自子目录。
- Feature 不得通过本目录建立领域转发层；消费者直接导入具体基础函数。
- Config 与 Feature 只能共同依赖基础规则，不得让基础配置反向读取 Feature 实现。
- 禁止恢复 `utils.ts` catch-all；样式类名组合直接依赖 `shared/ui/class-name.ts`。
