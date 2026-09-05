# Agent Options

本目录拥有 Agent 配置编辑器、业务弹窗和字段子域。

- 普通 Agent 的内联身份页简介正文由 identity/agent-profile-file-editor.tsx 读取并完整展示根级 AGENTS.md，独立处理 Markdown 预览、编辑和确认保存；该用户可审阅正文不因页面密度优化而折叠。主 Nexus 不生成 AGENTS.md，因此隐藏这块文件简介。创建来源例外：默认行为模板从后端读取，并随创建事务一次性落为新 Agent 的 AGENTS.md；不得把模板混入数据库摘要字段。

- `AgentOptionsInlineEditor` 与 `AgentOptionsDialogEditor` 是两个明确壳层入口；不得恢复通过可选参数拼装内联导航、Footer 和关闭策略的组合模式。
- `AgentOptionsInlineEditor` 可由既有 Agent 详情启用自动保存：草稿停止变化后再提交，不渲染底部保存/删除操作区，并通过外层 Header 回报保存状态；显式提交场景仍保留原操作区。弹窗 Footer 继续遵循 Dialog 壳层自己的分区语义。
- `AgentOptionsDialogEditor` 在宽桌面使用紧凑的中性左侧导航；窗口小于 `xl` 时提前切成顶部标签条，手机内容保持单列滚动，不等待表单已经被挤压后才响应。导航项固定复用 `UiButton variant="ghost"` 与 `aria-current`，活动项只使用共享中性浅底和较强文字，不绘制蓝色边框或独立图标底。
- Agent 选项块采用可触控但克制的控件高度和明确的组间距；内联详情头像保持 56px 识别尺度，左缘与正文 gutter 对齐，并通过中性文字动作打开网格选择面板，不得把头像或蓝色 pill 做成页面主视觉。工具页必须同时展示全部权限模式的名称与行为差异，预授权工具和默认 Connector 也保留用途、连接状态与开关，帮助用户在操作前完成比较；权限模式固定复用 `UiChoiceButton tone="neutral"`，选中态使用共享中性底色且不以阴影强调，危险模式警示由 `UiInlineNotice` 承载，Connector 加载态使用共享 Spinner 配方。
- 编辑器输入统一使用 `create/edit` 来源对象；模式、Agent ID 和初始值不得拆回可冲突的可选参数集合。
- Agent 名称允许重复，workspace 由稳定 Agent ID 定位；编辑器只做与服务端规则一致的本地格式预检，不发送名称可用性请求或显示成功占位。
- `editor/` 管理草稿、异步校验和保存事务，组合控制器只返回内容与动作模型。
- `components/` 只渲染身份、技能、权限、内容选择、动作和弹窗导航视图。
- `dialog/` 提供 Contacts 创建/编辑 Agent 的 Portal 壳层；壳层使用 plain 标题，不显示设置图标、生成式副标题或内部 Agent ID；宽桌面弹窗使用稳定的视口高度，切换栏目时只更新内部内容并由内容区独立滚动；手机弹窗接近全屏，底部操作始终固定。
- `agent-options-mutation.ts` 定义创建和更新共用的字段边界，`use-existing-agent-options-commands.ts` 负责既有 Agent 的保存与名称校验。
- 可编辑 Options 只由 `lib/agent-options.ts` 的 `pickAgentEditableOptions` 投影，编辑器初值和持久化载荷不得各维护一份字段表。
- Agent Options 业务组件不得放入 `shared/ui/dialog/`。
- 高级页的工具与 Connector 授权条目复用 `UiListRow variant="outlined"` 和共享图标/排版；行本身只展示信息，唯一命中区是 `GlassSwitch`。断开连接且未选中的 Connector 不可启用，断开但已选中的仍允许关闭，UI 重构不得改变此权限边界。
