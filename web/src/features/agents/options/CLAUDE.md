# Agent Options

本目录拥有 Agent 配置编辑器、业务弹窗和字段子域。

- `AgentOptionsInlineEditor` 与 `AgentOptionsDialogEditor` 是两个明确壳层入口；不得恢复通过可选参数拼装内联导航、Footer 和关闭策略的组合模式。
- `editor/` 管理草稿、异步校验和保存事务，组合控制器只返回内容与动作模型。
- `components/` 只渲染身份、技能、权限、内容选择、动作和弹窗导航视图。
- `dialog/` 提供 Contacts 创建/编辑 Agent 的 Portal 壳层。
- `agent-options-mutation.ts` 定义创建和更新共用的字段边界，`use-existing-agent-options-commands.ts` 负责既有 Agent 的保存与名称校验。
- 可编辑 Options 只由 `lib/agent-options.ts` 的 `pickAgentEditableOptions` 投影，编辑器初值和持久化载荷不得各维护一份字段表。
- Agent Options 业务组件不得放入 `shared/ui/dialog/`。
