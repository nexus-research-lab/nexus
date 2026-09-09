# Custom MCP

- 本目录负责 owner 级自定义 MCP 的目录、脱敏表单、详情、CRUD 与启停控制器；目录不重复页签标题、数量或启用教程。
- 新建 MCP 的 owner 开关默认开启；关闭后配置仍保留在管理目录，但不进入对话选择或 runtime。Agent 与 Session 的 `connector_ids` 只能在 owner 已开启集合中继续选择。
- 目录、详情和对话选择统一通过 `ConnectorIcon` 使用 WorkGraph 的 seeded 曲线图标，同名配置保持稳定视觉身份。
- 目录行复用 `UiListRow` 的默认标题、摘要和 Badge 插槽；连接目标只通过 `code` 角色表达等宽文本，不在业务层复制字号、行高或圆角。
- 目录加载、首次空集与筛选无结果复用 UiResourceState；只有首次空集提供原有添加动作。编辑和删除只使用 UiListActionButton，开关继续有独立的事件边界，忙碌/恢复禁用规则不由行样式推断。
- `detail/` 只读取远程 HTTP/SSE MCP 的初始化信息和 `tools/list`，不请求或展示 Prompts/Resources；stdio 命令只允许 Agent runtime 执行，管理页明确显示 runtime-only 状态。
- 详情内容轴和二级导航复用 `CapabilityDetailPage`，图标、标题、状态、连接目标和动作对齐复用 `CapabilityDetailIdentity`；其余事实和恢复提示复用 Connector 详情相同的 Button、Typography、Badge、Panel 与 Resource State 语法，不保留 Custom MCP 私有控件、面包屑或反馈样式。
- `env` 与 `headers` 的服务端返回值只允许为 `null`，表示已配置但不回显；编辑时空值必须保留原秘密。
- 表单只投影 stdio、HTTP、SSE 已实现字段，不引入另一套配置协议；传输和认证方式是配置值，必须使用 `UiSegmentedControl`，不得借页面 `UiTabs` 表达。
- 类型和认证使用 `UiSegmentedControl showLabel` 的唯一具名组，普通字段及标题按弹窗实例关联。参数、环境变量和请求头行在草稿中保留稳定本地身份，名称和删除动作区分行；身份由视图提供，model 维持纯投影，保存时只输出既有协议字段。命令、参数和键名复用公共 Input 的 code 文本角色，秘密值沿用 password 控件。
- 写命令共享唯一 ref 互斥入口，成功后同时刷新自定义目录和 Connector 目录。
- 写失败只在服务端明确 `not_applied` 时允许按原输入重试；旧响应与传输中断保持结果未知，并提供“重新加载并检查”而不自动重复 CRUD。
- 创建编辑弹窗使用随内容增长的 plain 表单，不显示 Connector 装饰图标或秘密保存副标题；动态参数和秘密行使用紧凑文字动作，窄屏必须纵向收拢。
- 创建编辑弹窗的恢复说明和本地校验错误统一使用 `UiInlineNotice`；业务只提供 message、tone 和 alert 语义，不得复制提示框圆角、背景或字号。

- 未知或 accepted 写入的恢复锁独立于 busy 和反馈；普通 GET 成功不能解锁或重放。只有 committed 可在成功刷新后收口，unknown 在只读核对后提供显式“重新操作”，accepted 继续只读核对。
- 目录首次读失败显示错误而非空集；普通刷新保留快照，访问失效丢弃快照并阻止写入。工具目录同样丢弃被撤权快照，重试不能复活旧工具或服务说明。
- 自定义表单提交处理器与 fieldset 共同遵守 busy/恢复锁；恢复锁不冒充保存中，仍允许关闭弹窗核对状态。
