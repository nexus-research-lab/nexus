# Operation Stage Tool Inventory and Interface Plan

## 目标

把当前 Nexus 项目里 Agent/runtime 可能使用的工具按真实来源梳理清楚，再把它们映射成少数几类舞台界面。本文只记录当前代码能证明的状态，不把未来可能接入的工具算作已存在工具。

## 证据入口

- Agent runtime 选项装配：`internal/runtime/clientopts/options_agent_client.go`
- SDK bridge v0.1.12 参数透传：`nexus-agent-sdk-bridge/client/transport_config.go`
- SDK bridge 自定义工具接口：`nexus-agent-sdk-bridge/tools/tool.go`
- DM runtime 注入：`internal/service/dm/service_runtime_client.go`
- Room runtime 注入：`internal/service/room/execution.go`
- MCP server 注入：`internal/app/server/builder_*_mcp.go`
- Nexus MCP 工具注册：`internal/mcp/*/tool/registry.go`
- Agent 工具配置 UI：`web/src/features/agents/options/agent-options-constants.ts`
- 当前舞台识别表：`web/src/features/conversation/operation/operation-tool-catalog.ts`

## 工具来源分层

### 1. Runtime 内建工具

这些工具不是 Nexus Go 代码自己实现的。Nexus 通过 `nexus-agent-sdk-bridge` 把 allow/deny/MCP 配置透传给 Claude/nxs runtime，runtime 返回的消息里会出现这些工具调用。

当前 Agent 工具配置 UI 暴露的内建/基础工具：

| 工具名 | 当前用途 | 舞台界面 |
| --- | --- | --- |
| `Task` | 委派子任务 | 活动监视器 |
| `TaskOutput` | 读取子任务输出 | 活动监视器 |
| `Bash` | 运行命令 | 终端 |
| `KillShell` | 停止命令 | 终端 |
| `Glob` | 文件匹配 | 访达 |
| `Grep` | 内容搜索 | 访达 |
| `LS` | 目录列表 | 访达 |
| `Read` | 读取文件 | Code 阅读器 |
| `Edit` | 修改文件 | Code 编辑器 |
| `Write` | 创建或覆盖文件 | Code 编辑器 |
| `NotebookEdit` | 修改 notebook | Code 编辑器 |
| `WebFetch` | 抓取网页 | Safari |
| `WebSearch` | 网页搜索 | Safari |
| `TodoWrite` | 更新计划 | 活动监视器 |
| `ExitPlanMode` | 退出计划模式 | 活动监视器 |
| `AskUserQuestion` | 向用户提问/确认 | 系统设置/确认窗口 |
| `Skill` | 读取 skill 上下文 | Nexus 知识窗口 |
| `nexus_imagegen` | 图片生成 MCP server 的整组授权别名 | 图像/媒体窗口 |

当前舞台额外识别但 UI 没完整对齐的工具：

| 工具名 | 状态 | 处理 |
| --- | --- | --- |
| `MultiEdit` | 舞台已识别，Agent 工具配置 UI 未列出 | 应补到 UI，归入 Code 编辑器 |
| `EnterPlanMode` | 舞台已识别，但 Agent sessions 默认 deny | 保留识别，标注为受限/不可触发 |

当前 Agent sessions 默认追加 deny 的 harness/旧计划工具：

- `EnterPlanMode`
- `ScheduleWakeup`
- `CronCreate`
- `CronList`
- `CronDelete`

这些不应作为舞台第一阶段产品工具展开；如果 runtime 历史消息里出现，走受限/兼容展示。

### 2. Nexus 托管 MCP 工具

这些是 Nexus Go 后端自己注册到 runtime 的 MCP tools。模型看到的完整名称可能是短名，也可能是 `mcp__<server>__<tool>` 形式；分类时必须按 server + leaf tool 双重识别。

#### `nexus_goal`

| 工具 | 用途 | 舞台界面 |
| --- | --- | --- |
| `get_goal` | 读取当前 goal | Nexus 控制台 |
| `create_goal` | 创建 goal | Nexus 控制台 |
| `update_goal` | 标记 goal 状态 | Nexus 控制台 |

#### `nexus_imagegen`

| 工具 | 用途 | 舞台界面 |
| --- | --- | --- |
| `generate_image` | 生成图片 | 图像/媒体窗口 |
| `edit_image` | 编辑图片 | 图像/媒体窗口 |

#### `nexus_automation`

| 工具 | 用途 | 舞台界面 |
| --- | --- | --- |
| `list_scheduled_tasks` | 列定时任务 | 自动化控制台 |
| `search_scheduled_task_history` | 搜索任务历史 | 自动化控制台 |
| `create_scheduled_task` | 创建定时任务 | 自动化控制台 |
| `update_scheduled_task` | 更新定时任务 | 自动化控制台 |
| `delete_scheduled_task` | 删除定时任务 | 自动化控制台 |
| `enable_scheduled_task` | 启用定时任务 | 自动化控制台 |
| `disable_scheduled_task` | 停用定时任务 | 自动化控制台 |
| `get_scheduled_task_status` | 查看任务状态 | 自动化控制台 |
| `run_scheduled_task` | 立即运行任务 | 自动化控制台/终端摘要 |
| `get_scheduled_task_runs` | 查看运行记录 | 自动化控制台 |
| `get_scheduled_task_events` | 查看事件 | 自动化控制台 |
| `get_scheduled_task_daily_report` | 日报 | 自动化控制台/报告窗口 |
| `retry_scheduled_task_delivery` | 重试投递 | 自动化控制台 |
| `recover_scheduled_task` | 恢复 running run | 自动化控制台 |

#### `nexus_room`

| 工具 | 可用条件 | 舞台界面 |
| --- | --- | --- |
| `publish_public_message` | Room runtime | Room 通讯窗口 |
| `send_directed_message` | Room private messages enabled | Room 通讯窗口 |

#### `nexus_connectors`

| 工具 | 用途 | 舞台界面 |
| --- | --- | --- |
| `connector_list` | 列当前用户已连接 connector | 连接器控制台 |
| `connector_call` | 用 connector access token 调 REST API | 连接器控制台/通用 API 窗口 |
| `feishu_docx_read` | 读飞书文档 | 文档窗口 |
| `feishu_docx_search` | 搜索飞书文档 | 文档搜索窗口 |
| `feishu_docx_sheet_sheets` | 列 Sheet | 表格窗口 |
| `feishu_docx_sheet_values` | 读 Sheet 值 | 表格窗口 |
| `feishu_docx_sheet_find` | 搜索 Sheet | 表格窗口 |
| `feishu_docx_bitable_tables` | 列 Bitable 表 | 数据表窗口 |
| `feishu_docx_bitable_fields` | 列字段 | 数据表窗口 |
| `feishu_docx_bitable_records` | 读记录 | 数据表窗口 |
| `feishu_docx_create` | 创建飞书文档 | 文档窗口 |
| `feishu_docx_append_markdown` | 追加 Markdown | 文档编辑窗口 |
| `feishu_docx_update_block` | 更新 Block | 文档编辑窗口 |
| `feishu_docx_drive_list` | 云空间列表 | 云盘/访达窗口 |
| `feishu_docx_wiki_spaces` | 知识库空间列表 | 知识库窗口 |
| `feishu_docx_wiki_space` | 知识库空间详情 | 知识库窗口 |
| `feishu_docx_wiki_nodes` | 知识库节点列表 | 知识库窗口 |
| `feishu_docx_wiki_node` | 知识库节点详情 | 知识库窗口 |

### 3. 自动增挂的外部 MCP server

`internal/app/server/builder_connector_mcp.go` 会在用户已连接对应 connector 时，把外部 MCP server 加进 runtime：

| Connector | MCP server 名 | 工具名来源 | 舞台策略 |
| --- | --- | --- | --- |
| 高德地图 | `amap_maps` | 远端 MCP 动态返回 | 地图/位置窗口，未适配前通用工具窗口 |
| 滴滴出行 | `didi_ride` | 远端 MCP 动态返回 | 出行窗口，未适配前通用工具窗口 |
| 钉钉 AI 表格 | `dingtalk_ai_table` | 用户提供的 MCP URL 动态返回 | 数据表窗口，未适配前通用工具窗口 |
| 腾讯文档 | `tencent_docs` | 远端 MCP 动态返回 | 文档/表格窗口，未适配前通用工具窗口 |
| 语雀 | `yuque` | `npx -y yuque-mcp` 动态返回 | 知识库/文档窗口，未适配前通用工具窗口 |

这些 server 的具体 leaf tool 不在 Nexus 源码里，不能写死。舞台需要从 runtime context/status 里的 `mcp_tools` 动态读取，再按 server 名和 annotations 分类。

### 4. 用户自定义 MCP server

Agent options 里有 `mcp_servers`。这部分完全由用户配置，Nexus 只透传给 runtime。舞台第一阶段不能假设具体工具名，只能按 MCP server 名、tool name、annotations 和输入输出做通用展示。

### 5. Connector 目录里的当前能力

Connector 目录不是一组独立模型工具；它们主要通过 `connector_list`、`connector_call` 或专用 MCP server 进入 runtime。

当前 catalog 中 `available` 的 connector：

- `feishu-docx`
- `amap`
- `didi`
- `dingtalk-ai-table`
- `tencent-docs`
- `yuque`
- `github`

其中：

- `feishu-docx` 已有 Nexus 专用工具。
- `amap`、`didi`、`dingtalk-ai-table`、`tencent-docs`、`yuque` 会在已连接时增挂外部 MCP server。
- `github` 当前 catalog 是 available，但没有在 `builder_connector_mcp.go` 里自动增挂外部 MCP；可通过 `connector_call` 走 REST API。

## 舞台界面分类

第一阶段不要按每个工具做一个组件。按少数界面承载工具族：

| 界面类 | 承载工具 |
| --- | --- |
| 访达 | `Glob`、`Grep`、`LS`、workspace/drive/wiki 列表类 |
| Code 编辑器/阅读器 | `Read`、`Write`、`Edit`、`MultiEdit`、`NotebookEdit` |
| 终端 | `Bash`、`KillShell`，以及 run 类任务的输出摘要 |
| Safari/研究窗口 | `WebSearch`、`WebFetch`、本地 HTML artifact |
| 活动监视器 | `Task`、`TaskOutput`、`TodoWrite`、计划状态 |
| 系统设置/确认窗口 | `AskUserQuestion` 和 permission request |
| Nexus 控制台 | `nexus_goal`、通用 Nexus MCP、未知 MCP |
| 自动化控制台 | `nexus_automation` |
| Room 通讯窗口 | `nexus_room` |
| 连接器控制台/API 窗口 | `connector_list`、`connector_call` |
| 文档/表格/知识库窗口 | `feishu_docx_*`、腾讯文档、语雀、钉钉 AI 表格 |
| 地图/出行窗口 | 高德、滴滴；未适配前走通用工具窗口 |
| 图像/媒体窗口 | `generate_image`、`edit_image` |
| 交付台 | round summary、最终 artifact、下一步 |

## 分阶段计划

### Phase 1: 工具分类真相源

目标：让舞台分类覆盖项目实际工具，而不是只覆盖 runtime 常见工具。

- 扩展 `operation-tool-catalog.ts`，支持 `mcp__server__tool`、`server.tool`、`server/tool` 的 leaf 识别。
- 增加 `tool_source`：`runtime_builtin`、`nexus_mcp`、`external_mcp`、`user_mcp`。
- 把 `MultiEdit` 加进 Agent 工具配置 UI，或明确标为 runtime-only recognized。
- 把 `EnterPlanMode` 标为 denied/legacy，避免舞台误导用户以为能主动触发。
- 为 `nexus_goal`、`nexus_automation`、`nexus_room`、`nexus_connectors`、`nexus_imagegen` 增加一等分类。

### Phase 2: 核心 runtime 工具体验

目标：先把日常编码会话的主路径做好。

- Code：Read/Write/Edit/MultiEdit/NotebookEdit。
- Finder：Glob/Grep/LS 和 workspace live events。
- Terminal：Bash/KillShell。
- Safari：WebSearch/WebFetch/HTML preview。
- System gate：AskUserQuestion/permission request。
- Activity Monitor：Task/TaskOutput/TodoWrite。

### Phase 3: Nexus 内建 MCP 体验

目标：Nexus 自己的产品能力不要长期落到 generic JSON 卡片。

- Goal：当前目标、创建、完成/阻塞状态。
- Automation：任务列表、运行记录、事件、日报、重试/恢复。
- Room：公开消息、定向消息的发送状态。
- Connectors：连接列表、connector_call 的请求/响应检查器。
- Imagegen：生成/编辑中的图片窗口和落盘 artifact。

### Phase 4: 文档、表格、知识库和外部 MCP

目标：把接入工具做成领域窗口，而不是每个 remote tool 一个组件。

- 飞书文档：文档、Sheet、Bitable、Drive、Wiki 五类窗口。
- 腾讯文档/语雀/钉钉 AI 表格：先按 server 名映射到文档/表格/知识库窗口。
- 高德/滴滴：先 generic，后续做地图/出行专用窗口。
- 用户自定义 MCP：默认 generic；根据 annotations 和 tool name 逐步沉淀适配器。

### Phase 5: 动态发现

目标：不靠手写完整远端 MCP 工具名。

- 读取 SDK bridge/runtime context 中的 `mcp_tools`、`system_tools`、`deferred_builtin_tools`。
- 把动态工具注册到舞台 store，只把稳定的 Nexus 内建工具写死。
- 保存未知工具样本，作为后续适配依据。

## 当前缺口

- 舞台现有分类还没有覆盖 `nexus_goal`、`nexus_automation`、`nexus_room`、`nexus_connectors`、`nexus_imagegen` 的一等语义。
- 外部 MCP server 的具体工具名目前无法从源码静态列出，必须运行时发现。
- Agent 工具配置 UI、runtime deny 策略、舞台识别表之间有不一致：`MultiEdit`、`EnterPlanMode` 是第一批要对齐的点。
- `github` connector 是 available，但没有专用 MCP server 注入；舞台应按 `connector_call` 处理它。
