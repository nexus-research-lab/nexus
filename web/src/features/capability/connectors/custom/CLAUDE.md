# Custom MCP

- 本目录负责 owner 级自定义 MCP 的目录、脱敏表单与 CRUD 控制器。
- 自定义 MCP 作为动态 Connector 暴露，Agent 与 Session 的开启状态继续只由既有 `connector_ids` 管理。
- `env` 与 `headers` 的服务端返回值只允许为 `null`，表示已配置但不回显；编辑时空值必须保留原秘密。
- 表单只投影 stdio、HTTP、SSE 已实现字段，不引入另一套配置协议。
- 写命令共享唯一 ref 互斥入口，成功后同时刷新自定义目录和 Connector 目录。
- 创建编辑弹窗使用随内容增长的 plain 表单，不显示 Connector 装饰图标或秘密保存副标题；动态参数和秘密行使用紧凑文字动作，窄屏必须纵向收拢。
