# Agent Options

本目录拥有 Agent 配置编辑器、业务弹窗和字段子域。

- 普通 Agent 的内联身份页简介正文由 identity/agent-profile-file-editor.tsx 读取根级 AGENTS.md，独立处理 Markdown 预览、编辑和确认保存；主 Nexus 不生成 AGENTS.md，因此隐藏这块文件简介；Agent Options 元数据保存不代替文件保存。

- `AgentOptionsInlineEditor` 与 `AgentOptionsDialogEditor` 是两个明确壳层入口；不得恢复通过可选参数拼装内联导航、Footer 和关闭策略的组合模式。
- `AgentOptionsInlineEditor` 的固定操作区与正文共用同一连续底面，不使用横向实体边框切割；弹窗 Footer 仍遵循 Dialog 壳层自己的分区语义。
- `AgentOptionsDialogEditor` 在宽桌面使用左侧导航；窗口小于 `xl` 时提前切成带 8px 分隔的顶部标签条，手机内容保持单列滚动，不等待表单已经被挤压后才响应。
- Agent 选项块采用可触控的控件高度和明确的组间距；头像通过放大的当前值打开网格选择面板，工具与技能卡片允许内容自然增高，不把完整配置压缩成密集控制条。
- 编辑器输入统一使用 `create/edit` 来源对象；模式、Agent ID 和初始值不得拆回可冲突的可选参数集合。
- `editor/` 管理草稿、异步校验和保存事务，组合控制器只返回内容与动作模型。
- `components/` 只渲染身份、技能、权限、内容选择、动作和弹窗导航视图。
- `dialog/` 提供 Contacts 创建/编辑 Agent 的 Portal 壳层；宽桌面弹窗设最大高度并让内容区独立滚动，手机弹窗接近全屏、隐藏装饰性 Header 信息，底部操作始终固定。
- `agent-options-mutation.ts` 定义创建和更新共用的字段边界，`use-existing-agent-options-commands.ts` 负责既有 Agent 的保存与名称校验。
- 可编辑 Options 只由 `lib/agent-options.ts` 的 `pickAgentEditableOptions` 投影，编辑器初值和持久化载荷不得各维护一份字段表。
- Agent Options 业务组件不得放入 `shared/ui/dialog/`。
