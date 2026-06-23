# Operation Stage Execution Visualization Plan

## 目标

把 Agent 工具执行过程可视化成“桌面现场”，让用户能看见：

1. 当前正在做什么。
2. 哪个工具打开了哪个应用窗口。
3. 输入、运行中状态、输出、产物和权限卡点如何串起来。
4. 本轮结束后留下哪些可继续的证据和文件。

核心原则：不要按每个工具单独写一套 UI。工具先归一成执行步骤，再由少数应用界面承载。

## 当前已有管线

当前代码已经有三层可复用结构：

```text
Message / permission / workspace live event
  -> OperationRuntimeEvent
  -> NexusOperationEvent / NexusOperationSnapshot
  -> StageWindowState / OperationDesktopState
  -> StageWindowContent renderer
```

对应文件：

- `operation-runtime-event-stream.ts`：从消息、权限、workspace live 事件生成 runtime event。
- `operation-projector.ts`：把 runtime 信息投影成 `NexusOperationSnapshot`。
- `operation-desktop-intents.ts`：把单个事件转成应用意图。
- `operation-scene-planner.ts`：把事件和 snapshot 规划成窗口集合。
- `operation-desktop-types.ts`：定义 `StageWindowKind`、`StageWindowPayload`、`StageWindowState`。
- `apps/operation-app-renderers.tsx`：按窗口类型渲染 Finder、Code、Terminal、Browser、Activity、Permission、Handoff 等界面。

下一步不是重做舞台，而是补一个更明确的“执行可视化契约”，把项目所有工具都稳定接入这条管线。

## 可视化对象

### 1. 执行步骤

每个 tool call、permission、artifact、handoff 都先归一成一个步骤：

```ts
interface OperationVisualStep {
  id: string;
  source: "runtime_builtin" | "nexus_mcp" | "external_mcp" | "user_mcp";
  server_name?: string | null;
  tool_name?: string | null;
  tool_leaf: string;
  group:
    | "workspace"
    | "code"
    | "terminal"
    | "browser"
    | "task"
    | "permission"
    | "goal"
    | "automation"
    | "room"
    | "connector"
    | "document"
    | "table"
    | "knowledge"
    | "image"
    | "map"
    | "ride"
    | "handoff"
    | "generic";
  phase: "queued" | "opening" | "running" | "waiting" | "writing" | "done" | "error" | "cancelled";
  app: OperationStageApp;
  subject: string;
  summary?: string | null;
  input?: Record<string, unknown> | null;
  output?: unknown;
  artifact?: OperationVisualArtifact | null;
  links: {
    session_key?: string | null;
    round_id: string;
    tool_use_id?: string | null;
    message_id?: string | null;
    permission_request_id?: string | null;
  };
}
```

这层可以由现有 `OperationRuntimeEvent` 和 `NexusOperationEvent` 扩展得到。它负责做工具来源、server 名、leaf tool、视觉分组的归一化。

### 2. 应用窗口

步骤不会直接渲染。步骤先变成应用意图，再规划窗口。

```text
OperationVisualStep
  -> app intent
  -> StageWindowState
  -> app renderer
```

一个 round 可以同时保留多个窗口：

- 当前工具对应的主窗口 focused。
- 前序工具窗口 background。
- 权限或错误窗口置顶。
- 文件、网页、图片等产物作为 artifact 保留。
- 最后一条 summary 进入交付台。

### 3. 时间线

每个 round 有一条可扫描时间线：

```text
queued -> app opening -> running/delta -> artifact/update -> result -> handoff
```

UI 上对应：

- 顶部 live strip：当前步骤、进度、下一步等待。
- 桌面窗口：具体执行现场。
- Dock：本轮已打开应用。
- 交付台：完成后的证据和继续入口。

## 状态如何可视化

| 执行状态 | 视觉表现 | 说明 |
| --- | --- | --- |
| `queued` | 桌面唤醒、Dock 轻亮、目标窗口准备打开 | 工具已出现但还没有结果 |
| `opening` | 应用窗口从 Dock/桌面图标打开 | 由 app intent 决定窗口类型 |
| `running` | 窗口聚焦、标题栏 active、内容区显示运行中态 | Terminal 光标、Browser loading、Code typing 等 |
| `waiting` | 系统设置/权限窗口置顶 | `AskUserQuestion` 或 permission request |
| `writing` | Code/文档/表格窗口展示 diff 或写入中 | workspace live event 可补实时内容 |
| `done` | 状态点转绿色，窗口沉淀到 background 或 artifact | 保留输入/输出摘要 |
| `error` | 错误窗口或当前应用错误态 | 保留 stderr、错误码、失败工具输入 |
| `cancelled` | 灰色完成态，交付台标记已取消 | 不继续播放 running 动画 |

## 工具到界面的展示方式

### Runtime 内建工具

| 工具族 | 窗口 | 执行过程 |
| --- | --- | --- |
| `Read` / `Write` / `Edit` / `MultiEdit` / `NotebookEdit` | Code | 打开文件、显示光标/扫描线、显示 diff、保存态 |
| `Glob` / `Grep` / `LS` | Finder | 展示目录树、搜索条件、命中文件和选中项 |
| `Bash` / `KillShell` | Terminal | 输入命令、stdout/stderr、退出状态、打开本地 HTML 时联动 Browser |
| `WebSearch` / `WebFetch` | Safari | 地址栏、搜索结果、抓取摘要、引用片段 |
| `Task` / `TaskOutput` / `TodoWrite` | Activity Monitor | 子任务、todo、进度、资源感知状态 |
| `Skill` | Nexus/知识窗口 | skill 名、加载片段、相关文件 |
| `AskUserQuestion` | System Settings | 问题、选项、确认/拒绝/回答 |

### Nexus MCP 工具

| 工具族 | 窗口 | 执行过程 |
| --- | --- | --- |
| `nexus_goal` | Nexus 控制台 | 当前目标、创建/更新状态、完成/阻塞变更 |
| `nexus_automation` | 自动化控制台 | 任务列表、创建/更新、run、events、delivery retry |
| `nexus_room` | Room 通讯窗口 | 公开消息/定向消息、目标成员、发送状态 |
| `nexus_connectors` | 连接器控制台/API 窗口 | connector 列表、REST 请求、响应、截断标记 |
| `nexus_imagegen` | 图像/媒体窗口 | prompt、生成中、结果图、落盘 artifact |

### 文档、表格、知识库和外部 MCP

| 来源 | 窗口 | 第一版策略 |
| --- | --- | --- |
| `feishu_docx_*` | 文档/表格/知识库窗口 | 按 tool leaf 精确分类 |
| `tencent_docs` | 文档/表格窗口 | 按 server 名 + tool name 关键词分类 |
| `dingtalk_ai_table` | 数据表窗口 | 表格/record/field 专用展示 |
| `yuque` | 知识库/文档窗口 | 搜索、知识库、文档阅读 |
| `amap_maps` | 地图窗口 | 第一版 generic，后续地图专用 |
| `didi_ride` | 出行窗口 | 第一版 generic，后续出行专用 |
| 用户 MCP | 通用工具窗口 | 根据 annotations 和样本逐步升级 |

## 第一阶段落地范围

先做最小但正确的一步：补可视化归一层和分类，不新建大量 UI。

### 1. 工具名归一

新增或扩展工具解析函数：

```ts
parse_operation_tool_name("mcp__nexus_automation__create_scheduled_task")
// -> { source: "nexus_mcp", server_name: "nexus_automation", tool_leaf: "create_scheduled_task" }
```

需要支持：

- `mcp__server__tool`
- `server.tool`
- `server/tool`
- 纯 leaf tool：`Read`、`Bash`、`create_goal`

### 2. 可视化分组表

在现有 `operation-tool-catalog.ts` / `operation-tool-visual-contract.ts` 之上补充：

- `nexus_goal`
- `nexus_automation`
- `nexus_room`
- `nexus_connectors`
- `nexus_imagegen`
- external MCP server 名映射
- unknown/user MCP fallback

输出必须是“视觉分组”，不是一堆组件名。

### 3. 扩展窗口类型

当前 `StageWindowKind` 已有 `finder`、`code_editor`、`browser`、`terminal`、`task_board`、`permission_wait`、`generic_tool` 等。

第一阶段建议只加少数真正需要的新 kind：

- `automation_console`
- `connector_console`
- `image_studio`
- `document_viewer`
- `table_viewer`
- `knowledge_base`

地图/出行先不加专用窗口，用 `generic_tool` 承载。

### 4. 复用现有 renderer

新增 UI 时优先复用：

- 文档/表格：先复用 `DocumentPreview`。
- Nexus 工具：先复用 `NexusToolSurface`，按类型换标题和字段。
- API 调用：用 `NexusToolSurface` 展示 request/response，比新写复杂 inspector 更省。
- 自动化：先扩展 `ActivityMonitorSurface` 或 `RunManifestSurface`，不单独做复杂 dashboard。

## 第二阶段体验补齐

当分类稳定后，再给高价值工具做专用执行动画：

1. Code 写入：workspace live event 驱动真正的 typed/diff 变化。
2. Terminal：stdout/stderr 分流、exit code、长输出折叠。
3. Browser：WebSearch 结果卡、WebFetch 摘要、HTML iframe。
4. Permission：真实 confirm/deny/answer 已接入后，展示等待和恢复。
5. Automation：任务状态机：draft -> scheduled -> running -> delivered/retry/recovered。
6. Imagegen：生成中骨架、结果图、文件路径、下载/打开。

## 第三阶段动态发现

外部 MCP 和用户 MCP 不能靠静态枚举。

需要从 runtime context/status 读取：

- `mcp_tools`
- `system_tools`
- `deferred_builtin_tools`

然后生成动态工具目录：

```ts
interface RuntimeToolCatalogEntry {
  server_name?: string | null;
  tool_name: string;
  description?: string | null;
  annotations?: {
    read_only?: boolean;
    destructive?: boolean;
    open_world?: boolean;
  };
  visual_group: OperationVisualStep["group"];
  renderer_kind: StageWindowKind;
}
```

未知工具默认策略：

1. `read_only` -> 低风险蓝色工具窗口。
2. `destructive` -> 权限/风险强调。
3. `open_world` -> 外部服务/API 窗口。
4. 无 annotation -> generic window，只显示脱敏 input/output。

## 典型过程示例

### 写文件再跑测试

```text
Write/Edit
  -> Code 窗口打开
  -> workspace live 显示写入中
  -> diff/save 完成
Bash
  -> Terminal 聚焦
  -> stdout/stderr 滚动
  -> exit status
round summary
  -> 交付台展示改动文件和测试结果
```

### 创建定时任务

```text
mcp__nexus_automation__create_scheduled_task
  -> 自动化控制台打开
  -> 表单摘要：schedule / target / delivery
  -> result 展示 job_id 和下一次触发时间
get_scheduled_task_status
  -> 同一控制台更新状态
```

### 调飞书文档

```text
feishu_docx_search
  -> 文档搜索窗口
  -> 展示 query 和命中文档
feishu_docx_read
  -> 文档窗口打开
  -> 正文预览
feishu_docx_append_markdown
  -> 文档编辑窗口
  -> append block / result
```

### 未知用户 MCP

```text
mcp__custom_server__foo
  -> parse server/tool
  -> 动态 catalog 未命中
  -> generic tool window
  -> 显示工具名、server、输入摘要、输出摘要、风险 annotation
```

## 实施顺序

1. 新增工具名解析和工具来源分类。
2. 扩展视觉分组表，覆盖上一份工具清单里的 Nexus MCP 和外部 MCP。
3. 扩展 `StageWindowKind`，但只加必要的 6 个。
4. 在 `operation-desktop-intents.ts` 里按视觉分组生成 app intent。
5. 在 `operation-scene-planner.ts` 里把新 intent 映射到窗口。
6. 在 `operation-app-renderers.tsx` 里先复用现有 surface，必要时加薄 wrapper。
7. 补 `verify-operation-stage-projector.mjs` 样例：automation、connector、imagegen、unknown MCP。

## 不做

- 不为每个 MCP leaf tool 写组件。
- 不为高德/滴滴马上做地图和打车完整 UI。
- 不在 projector 里直接写 JSX 或 UI 状态。
- 不把远端 MCP 工具名静态写死成完整清单。

## 判断完成的标准

- 任意工具都能在舞台上得到一个可解释窗口，而不是空白或 JSON 垃圾堆。
- Nexus 内建 MCP 有明确业务窗口，不再全部落入 generic。
- 未知 MCP 也能展示 server、tool、输入、输出、风险 annotation。
- 同一个 round 的多个工具能在桌面上保留轨迹：当前窗口 focused，历史窗口 background，产物进入交付台。
