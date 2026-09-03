# Custom MCP

- 本目录负责 owner 级自定义 MCP 的目录、脱敏表单、详情、CRUD 与启停控制器；目录不重复页签标题、数量或启用教程。
- 新建 MCP 的 owner 开关默认开启；关闭后配置仍保留在管理目录，但不进入对话选择或 runtime。Agent 与 Session 的 `connector_ids` 只能在 owner 已开启集合中继续选择。
- 目录、详情和对话选择统一通过 `ConnectorIcon` 使用 WorkGraph 的 seeded 曲线图标，同名配置保持稳定视觉身份。
- `detail/` 只读取远程 HTTP/SSE MCP 的初始化信息和 `tools/list`，不请求或展示 Prompts/Resources；stdio 命令只允许 Agent runtime 执行，管理页明确显示 runtime-only 状态。
- 详情返回动作、状态、事实和恢复提示复用 Connector 详情相同的 Button、Typography、Badge、Panel 与 Resource State 语法，不保留 Custom MCP 私有控件或反馈样式。
- `env` 与 `headers` 的服务端返回值只允许为 `null`，表示已配置但不回显；编辑时空值必须保留原秘密。
- 表单只投影 stdio、HTTP、SSE 已实现字段，不引入另一套配置协议。
- 写命令共享唯一 ref 互斥入口，成功后同时刷新自定义目录和 Connector 目录。
- 写失败只在服务端明确 `not_applied` 时允许按原输入重试；旧响应与传输中断保持结果未知，并提供“重新加载并检查”而不自动重复 CRUD。
- 创建编辑弹窗使用随内容增长的 plain 表单，不显示 Connector 装饰图标或秘密保存副标题；动态参数和秘密行使用紧凑文字动作，窄屏必须纵向收拢。
