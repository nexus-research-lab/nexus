# Agent Options

本目录拥有 Agent 配置编辑器、业务弹窗和字段子域。

- 普通 Agent 的内联身份页简介正文由 identity/agent-profile-file-editor.tsx 读取并完整展示根级 AGENTS.md，独立处理 Markdown 预览、编辑和确认保存；该用户可审阅正文不因页面密度优化而折叠。主 Nexus 不生成 AGENTS.md，因此隐藏这块文件简介。创建来源例外：默认行为模板从后端读取，并随创建事务一次性落为新 Agent 的 AGENTS.md；不得把模板混入数据库摘要字段。

- `AgentOptionsInlineEditor` 与 `AgentOptionsDialogEditor` 是两个明确壳层入口；不得恢复通过可选参数拼装内联导航、Footer 和关闭策略的组合模式。
- `AgentOptionsInlineEditor` 可由既有 Agent 详情启用自动保存：草稿停止变化后再提交，不渲染底部保存/删除操作区，并通过外层 Header 回报保存状态；显式提交场景仍保留原操作区。弹窗 Footer 继续遵循 Dialog 壳层自己的分区语义。
- `AgentOptionsDialogEditor` 在宽桌面使用紧凑的中性左侧导航；窗口小于 `xl` 时提前切成顶部标签条，手机内容保持单列滚动，不等待表单已经被挤压后才响应。活动项只使用共享中性浅底和较强文字，不绘制蓝色边框或独立图标底。
- Agent 选项块采用可触控但克制的控件高度和明确的组间距；内联详情头像保持 56px 识别尺度，左缘与正文 gutter 对齐，并通过中性文字动作打开网格选择面板，不得把头像或蓝色 pill 做成页面主视觉。工具页只在连接器列表下方、默认折叠的高级设置中提供独立权限控制，产品选择器统一仅提供请求批准、自动接受编辑和完全访问；不展示工具预授权开关，低风险网页检索由宿主统一预授权。默认 Connector 保留用途、连接状态与开关，图标和文字链接到对应连接器详情，开关独立控制挂载；选中态复用共享中性底色，展开权限设置后显示危险模式警示。
- 编辑器输入统一使用 `create/edit` 来源对象；模式、Agent ID 和初始值不得拆回可冲突的可选参数集合。
- Agent 名称允许重复，workspace 由稳定 Agent ID 定位；编辑器只做与服务端规则一致的本地格式预检，不发送名称可用性请求或显示成功占位。
- `editor/` 管理草稿、异步校验和保存事务，组合控制器只返回内容与动作模型。
- `components/` 只渲染身份、技能、权限、内容选择、动作和弹窗导航视图。
- `dialog/` 提供 Contacts 创建/编辑 Agent 的 Portal 壳层；壳层使用 plain 标题，不显示设置图标、生成式副标题或内部 Agent ID；宽桌面弹窗使用稳定的视口高度，切换栏目时只更新内部内容并由内容区独立滚动；手机弹窗接近全屏，底部操作始终固定。
- `agent-options-mutation.ts` 定义创建和更新共用的字段边界，`use-existing-agent-options-commands.ts` 负责既有 Agent 的保存与名称校验。
- 可编辑 Options 只由 `lib/agent-options.ts` 的 `pickAgentEditableOptions` 投影，编辑器初值和持久化载荷不得各维护一份字段表。
- Agent Options 业务组件不得放入 `shared/ui/dialog/`。

权限选择统一为 default/auto/bypassPermissions；auto 是 SDK 动态审核，历史 acceptEdits 只读保留，不把旧值伪装成已启用审核。
