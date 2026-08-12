# Nexus 后端 API 说明书

本说明书基于后端路由定义 `internal/app/server/routes.go` 与前端 API 客户端 `web/src/lib/api/*.ts` 整理，描述后端提供给前端的全部 HTTP 接口与 WebSocket 实时通信协议。

## 目录

- [通用约定](#通用约定)
- [1. 核心与系统](#1-核心与系统)
- [2. 认证与个人资料](#2-认证与个人资料)
- [3. 设置（偏好 / 运行时 / Provider）](#3-设置偏好--运行时--provider)
- [4. Agent 管理](#4-agent-管理)
- [5. Session 会话与消息](#5-session-会话与消息)
- [6. Workspace 工作区](#6-workspace-工作区)
- [7. Skill 技能](#7-skill-技能)
- [8. Room 房间与对话](#8-room-房间与对话)
- [9. Launcher 启动器](#9-launcher-启动器)
- [10. Capability 能力总览与 Loop](#10-capability-能力总览与-loop)
- [11. Connector 连接器](#11-connector-连接器)
- [12. Channel 通道与配对](#12-channel-通道与配对)
- [13. Scheduled Tasks 定时任务](#13-scheduled-tasks-定时任务)
- [14. Heartbeat 心跳自动化](#14-heartbeat-心跳自动化)
- [15. Execution WorkGraph](#15-execution-workgraph)
- [16. Goal 目标](#16-goal-目标)
- [17. Admin 订阅管理](#17-admin-订阅管理)
- [18. WebSocket 实时通信](#18-websocket-实时通信)
- [附：路径前缀与别名](#附路径前缀与别名)

---

## 通用约定

### 基础前缀

- 所有 API 默认前缀：`/nexus/v1`（由后端 `config.APIPrefix` 控制，前端 `getAgentApiBaseUrl()` 解析 `VITE_API_URL`，默认 `/nexus/v1`）。
- WebSocket 默认路径：`/nexus/v1/chat/ws`（前端 `getAgentWsUrl()` 解析 `VITE_WS_URL`）。
- 桌面端运行时可通过桌面壳配置覆盖 `apiBaseUrl` / `wsUrl`。

### 认证

- 所有请求携带 Cookie（前端 `fetch` 固定 `credentials: "include"`）。
- 桌面端通过 `applyDesktopRequestHeaders` 注入会话令牌头。
- 未认证返回 `401`，前端会广播 `nexus:auth-required` 事件（`notify_on_401` 可在单次请求关闭）；WebSocket 鉴权失败以关闭码 `4401` 通知。

### 响应格式

统一响应体 `ApiResponse<T>`：

```jsonc
{
  "data": { /* 业务数据 */ }
  // 其余元字段可选
}
```

错误响应示例：

```jsonc
{ "detail": "错误描述" }              // 直接 detail
{ "message": "错误描述" }             // 直接 message
{ "data": { "detail": "...", "request_id": "..." } } // 嵌套错误
```

前端 `requestApi<T>` 在 `response.ok` 时取出 `data` 返回；失败时抛出 `ApiRequestError(message, status)` 或 `UnauthorizedError`。

### 请求约定

- `body` 为对象/数组时自动 `JSON.stringify` 并设置 `Content-Type: application/json`。
- `FormData` / `URLSearchParams` / `Blob` 等保持原样，不强制 JSON。
- 默认超时 `30_000ms`，可通过 `timeout_ms` 覆盖（如 Git Skill 操作使用 `360_000ms`）。
- 可传入 `AbortSignal` 进行取消。

---

## 1. 核心与系统

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| GET | `/health` | 健康检查 | — |
| GET | `/system/version` | 后端服务版本信息（project/version/git_commit/build_date/goos/goarch/target/release_url） | — |
| GET | `/runtime/options` | 运行时配置（default_agent_id、默认 provider/model、preferences） | `hydrateRuntimeOptions` |

`/runtime/options` 在应用启动时拉取，用于初始化默认 Agent、Provider 与用户偏好。

---

## 2. 认证与个人资料

| 方法 | 路径 | 说明 | 请求体 / 参数 | 前端函数 |
|------|------|------|---------------|---------|
| GET | `/auth/status` | 登录状态 | — | `getAuthStatus` |
| POST | `/auth/login` | 登录 | `{ username, password }` | `loginApi` |
| POST | `/auth/logout` | 登出 | — | `logoutApi` |
| GET | `/settings/profile` | 个人资料（含 token 用量、订阅、可改密标识） | — | `getPersonalProfileApi` |
| PATCH | `/settings/profile` | 更新个人资料 | `{ avatar }` | `updatePersonalProfileApi` |
| POST | `/settings/profile/password` | 修改密码 | `{ current_password, new_password }` | `changePasswordApi` |

`AuthStatus` 字段：`auth_required`、`authenticated`、`username`、`user_id`、`display_name`、`role`、`avatar`、`auth_method`、`setup_required`、`access_token_enabled` 等。

---

## 3. 设置（偏好 / 运行时 / Provider）

### 偏好与运行时

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/settings/preferences` | 获取用户偏好 |
| PATCH | `/settings/preferences` | 更新用户偏好 |
| GET | `/settings/runtime/nxs/status` | NXS 运行时状态（前端 `getNxsRuntimeStatusApi`，超时 8s） |

模型偏好使用 `{ provider, model }` 结构。主会话、后台、图片生成和视觉模型分别保存在 `default_model_selection`、`default_background_model_selection`、`default_image_generation_model_selection` 与 `default_vision_model_selection`。主链无法承载图片时，nxs 才把这个视觉模型作为 `ViewImage` 的按需分析入口。

### Provider 配置（`/settings/providers`）

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| GET | `/settings/provider-presets` | Provider 预设列表 | `listProviderPresetsApi` |
| GET | `/settings/providers` | Provider 配置列表 | `listProviderConfigsApi` |
| GET | `/settings/providers/options` | Provider 可选项（query: `agent_runtime_kind`） | `listProviderOptionsApi` |
| POST | `/settings/providers` | 创建 Provider 配置 | `createProviderConfigApi` |
| PUT | `/settings/providers/{provider}` | 更新 Provider 配置 | `updateProviderConfigApi` |
| DELETE | `/settings/providers/{provider}` | 删除 Provider 配置（query: `force=1` 强删） | `deleteProviderConfigApi` |
| POST | `/settings/providers/{provider}/models/fetch` | 拉取 Provider 远端模型 | `fetchProviderModelsApi` |
| PUT | `/settings/providers/{provider}/models/{model_id}` | 更新单个模型 | `updateProviderModelApi` |
| POST | `/settings/providers/{provider}/models/{model_id}/default` | 设为默认模型 | — |
| POST | `/settings/providers/{provider}/test` | 测试 Provider 配置 | `testProviderConfigApi` |
| POST | `/settings/providers/{provider}/models/{model_id}/test` | 测试单个模型 | `testProviderModelApi` |

`GET /settings/providers/options` 返回按用途过滤的模型列表，包括 `chat_items`、`background_items`、`image_generation_items` 和 `vision_items`。`vision_items` 只包含模型卡明确声明支持图片输入的已启用模型；能力未知的模型不会自动进入该列表。

Provider 预设通过 `endpoint_mode` 声明端点来源：`fixed` 使用内置目录，`resource` 由用户填写资源级 Base URL，`custom` 使用完整自定义端点。内置 Azure OpenAI 预设属于 `resource`：接受资源根地址、`/openai/` 或 `/openai/v1` 并统一保存为 v1 Base URL；Azure 请求中的 `model` 必须使用实际 deployment name，因此该预设关闭远端模型同步，使用“添加模型”录入 deployment name。历史上以 `provider=azure` 保存、且地址可安全归一化的 Custom Provider 会在读取时投影为内置 Azure preset，下一次保存后正式持久化；其他自定义 Azure operation URL 保持原配置。

### 共享项目 ACL

仅在 Linux `NEXUS_RUNTIME_ISOLATION_MODE=enforce` 下可用；其他部署返回 `501`。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/projects` | owner/admin 查看完整 registry；普通成员只看到自己已加入的项目，成员表仅保留自身 |
| POST | `/projects` | owner/admin 创建或 ensure 项目，body: `{ project_id }` |
| PUT | `/projects/{project_id}/members/{owner_user_id}` | admin 设置成员权限，body: `{ access: "read" \| "write" \| "none" }` |

新项目会由 root-owned launcher 原子授予创建者 `write`；对已存在项目执行
`POST /projects` 不会自动加入调用者。

---

## 4. Agent 管理

| 方法 | 路径 | 说明 | 请求体 / 参数 | 前端函数 |
|------|------|------|---------------|---------|
| GET | `/agents` | Agent 列表 | — | `getAgents` |
| POST | `/agents` | 创建 Agent，并将行为模板原子写入新 workspace 的 `AGENTS.md` | `{ name, options, avatar, description, profile_template, vibe_tags }` | `createAgentApi` |
| GET | `/agents/profile-template` | 获取创建 Agent 使用的默认行为模板 | — | `getAgentProfileTemplateApi` |
| GET | `/agents/validate/name` | 校验名称（query: `name`, `exclude_agent_id`） | — | `validateAgentNameApi` |
| GET | `/agents/{agent_id}` | Agent 详情 | — | — |
| PATCH | `/agents/{agent_id}` | 更新 Agent | `{ name, options, avatar, description, vibe_tags }` | `updateAgentApi` |
| DELETE | `/agents/{agent_id}` | 删除 Agent | — | `deleteAgentApi` |
| GET | `/agents/{agent_id}/sessions` | Agent 的会话列表 | — | `getAgentSessionsApi` |
| GET | `/agents/{agent_id}/private-domain/threads` | 私域线程列表 | — | — |
| GET | `/agents/{agent_id}/private-domain/threads/{thread_id}/events` | 私域线程事件 | — | — |

`Agent.options.skill_ids` 保存平台 Skill 的稳定 ID，或用户级外部 Skill 的
`external:<skill_name>` 引用；`Agent.options.disabled_skill_ids` 保存显式停用
名称。平台 Skill 由全局兼容根提供；外部 Skill 共享
`<workspace>/<owner>/.agents/skills`（系统 owner 使用 `<workspace>/.agents/skills`）。
Agent workspace Skill 只保留在所属 Agent 的 workspace，仅在该 Agent 设置页可见，
文件存在时默认启用，显式停用只写入该 Agent 的 `disabled_skill_ids`。
Skill 列表中的 `source_type`/`source_kind` 描述文件来源，`storage_scope`/`origin_kind`
描述存储与创建归属。Agent workspace 来源不进入全局技能库，也不能由其它 Agent
发现、引用或复制。

`description` 是目录与提示词中的短摘要；`profile_template` 是创建期行为模板，两者不可互换。前端先从服务端读取默认模板，用户修改后随创建请求提交；传空时服务端仍使用同一默认模板。

### Agent 技能挂载

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| GET | `/agents/{agent_id}/skills` | Agent 可用技能及启用状态，包含该 Agent 私有 workspace Skill | `getAgentSkillsApi` |
| POST | `/agents/{agent_id}/skills` | 兼容启用入口（body: `{ skill_name }`）；新 UI 使用 PATCH | `installSkillApi` |
| PATCH | `/agents/{agent_id}/skills/{skill_name}` | 原子切换技能（body: `{ enabled, target_scope }`）；必填 `target_scope` 为 `global_library` 或 `agent_workspace` | `setAgentSkillEnabledApi` |
| DELETE | `/agents/{agent_id}/skills/{skill_name}` | 兼容删除入口；全局 Skill 解除当前 Agent 绑定，workspace Skill 删除本地文件 | `uninstallSkillApi` |

`scope: room` 是 Room 使用范围，不是独立来源；它继续出现在全局技能库，但不会
进入 Agent 列表或 Agent 启用矩阵，只能由 Room 设置选择。

---

## 5. Session 会话与消息

### 会话列表

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| GET | `/sessions` | 全部会话（DM 视角） | `getConversations` |
| POST | `/sessions` | 创建会话 | — |
| PATCH | `/sessions/{session_key}` | 更新会话 | — |
| DELETE | `/sessions/{session_key}` | 删除会话 | — |

### 消息与轮次

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| GET | `/sessions/messages` | 按 `session_key` 查消息（分页） | `getSessionMessagesApi` |
| GET | `/sessions/rounds` | 会话轮次索引 | `getSessionRoundIndexApi` |
| GET | `/sessions/{session_key}/messages` | 按路径查消息 | — |

消息分页 query 参数：`limit`、`before_round_id`、`before_round_timestamp`、`around_round_id`、`around_limit`。返回 `{ items, has_more, next_before_round_id, next_before_round_timestamp }`。

### 子 Agent 任务（Subagent Tasks）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/sessions/{session_key}/tasks` | 任务列表 |
| GET | `/sessions/{session_key}/tasks/{task_id}/messages` | 任务消息 |
| POST | `/sessions/{session_key}/tasks/{task_id}/messages` | 向任务发送消息 |
| POST | `/sessions/{session_key}/tasks/{task_id}/stop` | 停止任务 |

列表响应的 `data` 为 `{ runtime_kind, capabilities, items }`。每个 item 也携带自身的
`runtime_kind` 与 `{ observe, transcript, stop, send_message, resume }`，历史记录与当前
会话 runtime 不一致时以前者为准。`nxs` 支持观察、完整 thread、停止、续聊与同 task
恢复；Claude Code 支持观察、完整 thread 与停止，不支持宿主侧续聊/恢复；未知 runtime
只开放观察和 transcript。向不支持续聊的 runtime 发送消息会返回 HTTP 409，错误码为
`subagent_operation_unsupported`。

task 消息优先投影 `transcript_path`。Claude Code 的 `local_agent` 若只提供指向 child
JSONL 的 `output_file`，服务端也会将它投影成与主会话一致的富消息 thread；普通文本
`output_file` 则保留为 output 摘要。

> Room 会话下的对应接口见 [Room 对话子任务](#对话-conversation)。

---

## 6. Workspace 工作区

针对单个 Agent 的工作区文件操作。

| 方法 | 路径 | 说明 | 请求体 / 参数 | 前端函数 |
|------|------|------|---------------|---------|
| GET | `/agents/{agent_id}/workspace/files` | 文件树 | — | `getWorkspaceFilesApi` |
| GET | `/agents/{agent_id}/workspace/memory` | SDK 文件式记忆投影（索引、主题、日志及 frontmatter 元数据） | — | `getAgentMemorySnapshotApi` |
| DELETE | `/agents/{agent_id}/workspace/memory` | 删除正文记忆并同步清理短索引 | query: `path`（仅 `memory/**/*.md`） | `deleteAgentMemoryDocumentApi` |
| GET | `/agents/{agent_id}/workspace/file` | 读文件内容 | query: `path` | `getWorkspaceFileContentApi` |
| PUT | `/agents/{agent_id}/workspace/file` | 写文件内容 | `{ path, content }` | `updateWorkspaceFileContentApi` |
| POST | `/agents/{agent_id}/workspace/upload` | 上传文件 | FormData: `file`, `path?` | `uploadWorkspaceFileApi` |
| GET | `/agents/{agent_id}/workspace/download` | 下载文件 | query: `path`, `disposition=attachment\|inline` | `downloadWorkspaceFileApi` / `getWorkspaceFilePreviewUrl` |
| POST | `/agents/{agent_id}/workspace/reveal` | 在文件夹中定位 | `{ path }` | 桌面端调用 |
| POST | `/agents/{agent_id}/workspace/entry` | 新建文件/目录 | `{ path, entry_type, content }` | `createWorkspaceEntryApi` |
| PATCH | `/agents/{agent_id}/workspace/entry` | 重命名 | `{ path, new_path }` | `renameWorkspaceEntryApi` |
| DELETE | `/agents/{agent_id}/workspace/entry` | 删除条目 | query: `path` | `deleteWorkspaceEntryApi` |

桌面端调用 `reveal`；浏览器端通过 `download` 接口下载文件。

长期记忆由内置 `nxs` SDK 子进程维护为 Agent 工作区中的 `MEMORY.md` 索引与 `memory/` 主题文件。Nexus 管理的 runtime 会把该工作区固定为唯一记忆根，不接受宿主环境、请求环境或远端记忆配置改写；Nexus 不参与提取或召回，只提供同一根目录的投影供 Web 展示，正文编辑仍使用通用工作区文件接口。删除正文记忆必须走专用接口，由服务端同时移除 `MEMORY.md` 中对应的一行索引；索引文件本身不可删除。

---

## 7. Skill 技能

### 全局技能市场

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| GET | `/skills` | 全局技能库（query: `agent_id`,`category_key`,`source_type`,`scope`,`q`）；带 `agent_id` 时追加该 Agent 私有 workspace Skill | `getAvailableSkillsApi` |
| GET | `/skills/{skill_name}` | 技能详情（query: `agent_id`） | `getSkillDetailApi` |
| GET | `/skills/{skill_name}/agents` | 当前用户各 Agent 的启用矩阵 | `getSkillAgentsApi` |
| POST | `/skills/import/local` | 导入本地技能；认证部署仅允许 FormData `file`，未认证本地单用户部署兼容 `local_path` | `importLocalSkillApi` |
| POST | `/skills/import/git` | 从 Git 仓库导入（body: `{ url, branch, path }`） | `importGitSkillApi` |
| GET | `/skills/search/external` | 搜索社区或私有技能（query: `q`,`include_readme`,`source_id`）；传 `source_id` 时只请求该来源 | `searchExternalSkillsApi` |
| GET | `/skills/external/preview` | 社区技能预览（query: `detail_url`） | `getExternalSkillPreviewApi` |
| POST | `/skills/import/skills-sh` | 从社区来源导入 | `importExternalSkillApi` |
| POST | `/skills/import/source` | 从私有来源导入（body: `{ source_id, skill_id }`），服务端重新解析索引与下载地址 | `importExternalSkillApi` |
| GET | `/skills/sources` | 社区与私有来源配置列表；凭据只返回是否已配置 | `listExternalSkillSourcesApi` |
| POST | `/skills/sources` | 测试并新增私有来源（body: `{ name, url, auth_type, token? }`） | `createExternalSkillSourceApi` |
| PATCH | `/skills/sources/{source_id}` | 更新来源配置 | `updateExternalSkillSourceApi` |
| DELETE | `/skills/sources/{source_id}` | 删除用户私有来源；不删除已导入 Skill | `deleteExternalSkillSourceApi` |
| POST | `/skills/check-updates` | 检查更新 | `checkSkillUpdatesApi` |
| POST | `/skills/update-imported` | 批量更新已导入 | `updateImportedSkillsApi` |
| POST | `/skills/{skill_name}/update` | 更新单个技能 | `updateSingleSkillApi` |
| DELETE | `/skills/{skill_name}` | 删除技能 | `deleteSkillApi` |

> Git 类操作耗时较长，前端统一使用 `360_000ms` 超时。

---

## 8. Room 房间与对话

### 房间管理

| 方法 | 路径 | 说明 | 请求体 | 前端函数 |
|------|------|------|--------|---------|
| GET | `/rooms/dm/{agent_id}` | 确保并返回 DM 房间 | — | `ensureDirectRoom` |
| GET | `/rooms` | 房间列表（query: `limit`） | — | `listRooms` |
| POST | `/rooms` | 创建房间 | `{ agent_ids, name, description, title, avatar, skill_names?, host_agent_id?, host_auto_reply_enabled?, private_messages_enabled? }` | `createRoom` |
| GET | `/rooms/{room_id}` | 房间详情 | — | — |
| PATCH | `/rooms/{room_id}` | 更新房间 | 同上 | `updateRoom` |
| DELETE | `/rooms/{room_id}` | 删除房间 | — | `deleteRoom` |
| GET | `/rooms/{room_id}/contexts` | 房间上下文聚合（room+members+conversations+sessions） | — | `getRoomContexts` |

### 成员管理

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| POST | `/rooms/{room_id}/members` | 添加成员（body: `{ agent_id }`） | `addRoomMember` |
| DELETE | `/rooms/{room_id}/members/{agent_id}` | 移除成员 | `removeRoomMember` |

### 对话（Conversation）

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| POST | `/rooms/{room_id}/conversations` | 创建对话（body: `{ title? }`） | `createRoomConversation` |
| PATCH | `/rooms/{room_id}/conversations/{conversation_id}` | 更新对话 | `updateRoomConversation` |
| DELETE | `/rooms/{room_id}/conversations/{conversation_id}` | 删除对话 | `deleteRoomConversation` |
| GET | `/rooms/{room_id}/conversations/{conversation_id}/messages` | 对话消息（分页参数同 Session） | `getRoomConversationMessages` |
| POST | `/rooms/{room_id}/conversations/{conversation_id}/attachments/upload` | 上传对话附件（FormData） | `uploadRoomConversationAttachmentApi` |
| POST | `/rooms/{room_id}/conversations/{conversation_id}/close` | 关闭对话运行时 | `closeRoomConversationRuntime` |

#### 对话子任务

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/rooms/{room_id}/conversations/{conversation_id}/tasks` | 任务列表 |
| GET | `.../tasks/{task_id}/messages` | 任务消息 |
| POST | `.../tasks/{task_id}/messages` | 发送任务消息 |
| POST | `.../tasks/{task_id}/stop` | 停止任务 |

响应结构、runtime capability 与 transcript 投影规则同 Session 子 Agent 接口。Room
task 的控制请求由 task item 的 `host_agent_id` 路由到实际承载该 subagent 的 Agent slot。

---

## 9. Launcher 启动器

| 方法 | 路径 | 说明 | 请求体 | 前端函数 |
|------|------|------|--------|---------|
| GET | `/launcher/bootstrap` | 启动引导数据 | — | `getLauncherBootstrapApi` |
| GET | `/launcher/suggestions` | 启动建议 | — | — |
| POST | `/launcher/query` | 解析启动查询 | `{ query }` → `{ action_type, target_id, initial_message? }` | `queryLauncher` |

`action_type` 取值：`open_agent_dm` / `open_room` / `open_app`。

---

## 10. Capability 能力总览与 Loop

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| GET | `/capability/summary` | 能力汇总（技能数、已连连接器、定时任务、通道、配对、Loop） | `getCapabilitySummaryApi` |
| GET | `/capability/loops` | Loop 列表（query: `locale`） | `listLoopsApi` |
| GET | `/capability/loops/{slug}` | Loop 详情（query: `locale`） | `getLoopApi` |

---

## 11. Connector 连接器

| 方法 | 路径 | 说明 | 请求体 / 参数 | 前端函数 |
|------|------|------|---------------|---------|
| GET | `/connectors` | 连接器列表（query: `q`,`category`,`status`） | — | `getConnectorsApi` |
| GET | `/connectors/categories` | 分类 | — | — |
| GET | `/connectors/count` | 数量 | — | — |
| GET | `/connectors/{connector_id}` | 连接器详情 | — | `getConnectorDetailApi` |
| PUT | `/connectors/{connector_id}/oauth-client` | 保存自有 OAuth Client | `{ client_id, client_secret }` | `saveConnectorOauthClientApi` |
| DELETE | `/connectors/{connector_id}/oauth-client` | 删除自有 OAuth Client | — | `deleteConnectorOauthClientApi` |
| GET | `/connectors/{connector_id}/auth-url` | 获取授权 URL | query: `redirect_uri`, `shop` | `getConnectorAuthUrlApi` |
| POST | `/connectors/oauth/callback` | OAuth 回调 | `{ code, state, redirect_uri }` | `completeConnectorOAuthApi` |
| POST | `/connectors/{connector_id}/device/start` | 启动 Device Flow | — | `startConnectorDeviceAuthApi` |
| POST | `/connectors/{connector_id}/device/poll` | 轮询 Device Flow | `{ device_code }` | `pollConnectorDeviceAuthApi` |
| POST | `/connectors/{connector_id}/connect` | 授权连接 | `{ auth_code? / api_key? / token? / redirect_uri? }` | `connectConnectorApi` |
| POST | `/connectors/{connector_id}/disconnect` | 断开连接 | — | `disconnectConnectorApi` |

---

## 12. Channel 通道与配对

### 通道配置（`/capability/channels`）

支持的 `channel_type`：`dingtalk` / `wechat` / `weixin-personal` / `feishu` / `telegram` / `discord`。

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| GET | `/capability/channels` | 通道配置列表 | `listChannelsApi` |
| PUT | `/capability/channels/{channel_type}/config` | 保存配置（body: `{ agent_id, config, credentials }`） | `upsertChannelConfigApi` |
| DELETE | `/capability/channels/{channel_type}/config` | 删除配置 | `deleteChannelConfigApi` |
| DELETE | `/capability/channels/{channel_type}/accounts/{account_id}` | 删除账号 | `deleteChannelAccountApi` |
| POST | `/capability/channels/{channel_type}/login` | 启动登录流程 | `startChannelLoginApi` |
| GET | `/capability/channels/{channel_type}/login/{login_id}` | 查询登录状态 | `getChannelLoginApi` |
| POST | `/capability/channels/{channel_type}/login/{login_id}/verify-code` | 提交验证码（body: `{ verify_code }`） | `submitChannelLoginVerifyCodeApi` |

`ChannelLoginView.status`：`running` / `verify_code_required` / `succeeded` / `error` / `expired` / `cancelled`。

### 配对（Pairing）

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| GET | `/capability/pairings` | 配对列表（query: `channel_type`,`status`,`agent_id`） | `listPairingsApi` |
| POST | `/capability/pairings` | 创建配对 | `createPairingApi` |
| PATCH | `/capability/pairings/{pairing_id}` | 更新配对 | `updatePairingApi` |
| DELETE | `/capability/pairings/{pairing_id}` | 删除配对 | `deletePairingApi` |

`ImPairingStatus`：`pending` / `active` / `disabled` / `rejected`；`ImChatType`：`dm` / `group`。

### 通道消息入口（外部适配器调用）

| 方法 | 路径 |
|------|------|
| POST | `/channels/messages` |
| POST | `/channels/internal/messages` |
| POST | `/channels/discord/messages` |
| POST | `/channels/telegram/messages` |
| POST | `/channels/dingtalk/messages` |
| POST | `/channels/feishu/messages` |
| POST | `/channels/weixin-personal/messages` |

---

## 13. Scheduled Tasks 定时任务

定时任务同时提供 **结构化路径**（`/capability/scheduled/tasks`）与 **扁平别名**（`/scheduled/tasks`），接口等价。前端统一使用结构化路径。

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| GET | `/capability/scheduled/reports/daily` | 日报 | — |
| GET | `/capability/scheduled/tasks` | 任务列表（query: `agent_id`） | `listScheduledTasksApi` |
| POST | `/capability/scheduled/tasks` | 创建任务 | `createScheduledTaskApi` |
| PATCH | `/capability/scheduled/tasks/{job_id}` | 更新任务 | `updateScheduledTaskApi` |
| DELETE | `/capability/scheduled/tasks/{job_id}` | 删除任务 | `deleteScheduledTaskApi` |
| POST | `/capability/scheduled/tasks/{job_id}/run` | 立即执行 | `runScheduledTaskApi` |
| POST | `/capability/scheduled/tasks/{job_id}/recover` | 恢复运行 | `recoverScheduledTaskRunApi` |
| GET | `/capability/scheduled/tasks/{job_id}/status` | 状态详情（含 recent_runs/events） | — |
| PATCH | `/capability/scheduled/tasks/{job_id}/status` | 更新状态 | `updateScheduledTaskStatusApi` |
| GET | `/capability/scheduled/tasks/{job_id}/runs` | 执行记录列表 | `listScheduledTaskRunsApi` |
| GET | `/capability/scheduled/tasks/{job_id}/events` | 事件列表 | — |
| POST | `/capability/scheduled/tasks/{job_id}/runs/{run_id}/delivery/retry` | 重试投递 | `retryScheduledTaskRunDeliveryApi` |

运行时通过 `nexus_automation` MCP 暴露 8 个意图级工具，底层 HTTP 和 Service 接口不受模型工具粒度约束：

| 工具 | 说明 |
|------|------|
| `create_scheduled_task` | 创建任务 |
| `find_scheduled_tasks` | 查找当前或已删除任务；历史查询使用 `include_deleted=true` |
| `update_scheduled_task` | 修改任务以及通过 `enabled` 启停任务 |
| `delete_scheduled_task` | 删除任务 |
| `inspect_scheduled_task` | 通过 `view=status|runs|events` 检查状态、运行历史或审计 |
| `get_scheduled_task_report` | 按日期聚合运行和投递情况 |
| `run_scheduled_task` | 立即执行一次，不改变后续排程 |
| `repair_scheduled_task` | 通过 `action=recover|retry_delivery` 恢复卡住运行或补发失败投递 |

创建和更新任务可传 `expires_at`（RFC3339）。到期后任务自动停用，但不会中断已经开始的 run；更新时传 `clear_expires_at: true` 可清除截止时间。

运行记录的 `trigger_kind` 使用 `scheduled`、`misfire`、`manual` 区分正常到点、错过窗口处理和手动执行。`cron` 只用于 `schedule.kind`，不再表示整个任务系统。

调度策略由服务端环境变量控制：

| 配置项 | 默认值 | 说明 |
|------|------|------|
| `AUTOMATION_SCHEDULER_LEASE_SECONDS` | `30` | 多实例 leader 租约时长 |
| `AUTOMATION_RECURRING_JITTER_MAX_SECONDS` | `900` | 循环任务稳定 jitter 上限 |
| `AUTOMATION_MISFIRE_POLICY` | `run_once` | 恢复时补跑一次；可设为 `skip` |
| `AUTOMATION_MISFIRE_GRACE_SECONDS` | `60` | `skip` 策略允许的延迟窗口 |
| `AUTOMATION_MAX_ENABLED_TASKS_PER_USER` | `100` | 单用户已启用任务上限 |

---

## 14. Heartbeat 心跳自动化

| 方法 | 路径 | 说明 | 请求体 | 前端函数 |
|------|------|------|--------|---------|
| GET | `/automation/heartbeat/{agent_id}` | 心跳配置与状态 | — | `getHeartbeatConfigApi` |
| PUT | `/automation/heartbeat/{agent_id}` | 更新心跳配置 | `HeartbeatUpdateInput` | `updateHeartbeatApi` |
| POST | `/automation/heartbeat/{agent_id}/wake` | 唤醒 | `{ mode?, text? }`（默认 `mode=now`） | `wakeHeartbeatApi` |

返回的心跳时间字段（`next_run_at` / `last_heartbeat_at` / `last_ack_at`）会被前端转换为时间戳。

---

## 15. Execution WorkGraph

| 方法 | 路径 | 说明 | 参数 | 前端函数 |
|------|------|------|------|---------|
| GET | `/executions/latest` | 读取当前或最近一次 managed WorkGraph 的安全只读投影 | query: `session_key`（必填） | `getLatestExecutionApi` |

该接口只公开 **managed WorkGraph**。managed 的最低条件是 durable Execution 拥有 active Plan，且该 Plan 至少包含一个 Work Item。查询顺序为：

1. 同一 authenticated owner / `session_key` 下最近的未终结 managed Execution；
2. 若不存在，返回最近一次 managed Execution，供 UI 回看 terminal 结果；
3. 若该 session 从未创建 managed WorkGraph，返回 `data: null`。

普通 runtime-only round、Goal-only continuation 和 planless Execution 都不属于公共 WorkGraph：它们不能让该接口返回非空，也不能覆盖已经保留的最近一次 managed WorkGraph。

非空响应为 `ExecutionView`，包含 Execution/Plan、进度、完整用户可见 Work Items，以及只读 `graph`：

```jsonc
{
  "data": {
    "id": "execution-id",
    "session_key": "session-key",
    "scope_kind": "dm",
    "status": "active",
    "plan": { "id": "plan-id", "revision": 1, "status": "active" },
    "progress": { "total": 2, "running": 1, "waiting": 1 },
    "work_items": [/* responsibility / attempt / delivery projection */],
    "graph": {
      "nodes": [/* agent | subagent | tool | gate */],
      "edges": [/* dependency | dispatch | coordination | spawn | invoke | guard | review | loop_back | retry */],
      "runtime_node_total": 12,
      "runtime_edge_total": 11,
      "runtime_nodes_truncated": false,
      "runtime_edges_truncated": false
    }
  }
}
```

`graph` 只包含有界、脱敏的 NodeRun 摘要与结构化 Artifact 引用，不暴露 command、lease、runtime capability identity、凭证或完整 Tool I/O。`runtime_*_total` 与 `runtime_*_truncated` 只描述 visibility 判定后本应进入主图的 runtime 节点/边是否完整；节点检查器内的 `detail` 历史不占主图配额，也不触发 partial。前端不得把真实截断结果展示为完整实时图。该端点没有创建 Plan、分派、重试或状态推进能力。

错误状态：缺少 `session_key` 或领域参数无效返回 `422`；Execution 服务未装配返回 `503`；读取失败返回 `500`。认证规则沿用全局约定。

成功的 Plan、Assignment、Attempt、Submission、Review、block/resume/takeover、Goal binding、terminal reconciliation 与 Runtime Graph 写入会向已绑定同一 authenticated owner + `session_key` 的会话连接发送 `execution_invalidated`；每次成功 `bind_session` 也会发送一个空 identity fence，以恢复断线期间错过的 ephemeral 通知。事件 `data` 固定包含 `execution_id` 与 `version`；撤销、替换、尚无可读图，或 Runtime Graph 写入没有推进 Execution aggregate revision 时，相应值可以是空字符串或 `0`，客户端始终必须按 envelope 的 `session_key` 重新读取本端点，不能把 payload 当作增量图。密集或幂等通知可合并，事件本身不授予 mutation 能力。

---

## 16. Goal 目标

Composer 的 Goal 模式不再调用 `POST /goals` 后自行拼接聊天，而是发送独立
WebSocket `set_goal`；用户手输的 `/goal <objective>` 虽然使用 `chat` envelope，
也会在 runtime 之前被同一个 Nexus host handler 截获。两条入口共享以下语义：

1. 先创建或显式替换当前 Goal；
2. 再持久化一条 canonical 内容为 `/goal <objective>`、`metadata.subtype=goal_set`
   且 `control_only=true` 的完成态用户控制记录；
3. 尝试返回 durable/transient `chat_ack` 和 `round_status=finished`；
4. 在第一次控制响应发送尝试之后，通过 Goal continuation 状态机继续执行。

这条控制记录不会进入模型，也不会等待 assistant/result；但它会使新会话进入
started 状态、推进 message count，并让 Goal objective 立即提供标题兜底。Goal SQL
与 owner workspace ledger 不是同一事务；Goal 是状态真相，ACK 的
`user_message_committed` 精确表示控制记录是否已经 durable。客户端是否实际收到
ACK 不构成 continuation 前提；若 Goal SQL 在 commit 前失败，则不会写控制记录或
启动 continuation。若 Goal 已提交但 ledger 写入失败，则返回 transient ACK，
不会把 `/goal` 显示成 durable 消息，但 continuation 仍可从权威 Goal 继续。

Room Goal continuation 通过公区 `@` 或 directed-message wake 请求协作时，服务端会把
精确 Goal ID/objective revision 作为 host-only 调度归因写入 directed-message、handoff、
队列及恢复记录。Goal-directed wake 在 immediate/delayed 调度前建立确定性 handoff；
启动恢复会从 directed-message 事实补建被崩溃打断的缺失 handoff，但仅限仍 active 的
同一 Goal revision，旧 revision 不会被重新投递。
`send_directed_message` 的幂等键由服务端从 SDK tool-use identity 生成，模型可写的
`correlation_id` 仍只用于诊断。相同工具调用的重试复用同一 message/wake identity；
immediate 与 delayed wake 都先持久化 schedule，成功入队后 complete，运行中失败会
在线重试，已 complete 的 wake 不会因迟到重试重新运行。
该字段不是客户端输入或模型能力，也不会给协作者的 conversation round 授予 Goal
mutation authority。协作者终态后，服务端记录符合可见性要求的证据并重新调度一轮
有权限的 lead continuation；重启期间未完成的归因 handoff 会继续阻止并发续跑。
归因字段上线前留下的终态 root 仅在宿主事实可严格证明时升级：当前 Goal active，
最新非 usage 审计事件是该 root source Agent round 的 `continuation_suppressed`，
root 全部终态，并且 canonical Room history 中同 root 有非 Lead 的公开实质终态。
恢复器不会读取模型正文来猜测 Goal 归属，也不会把用户取消或普通拒绝恢复成续跑。
`Goal.status=paused` 只表示真实生命周期暂停；`status=active` 且
`empty_progress_count>0` 表示系统因上一轮无可计入进展而停止自动续跑，前端会明确
展示为“自动续跑已停止”，不能渲染成或描述成 Agent 主动暂停。

`GET /sessions`、`GET /agents/{agent_id}/sessions` 与 Room context 返回合并读模型，
不再把旧 SQL `messages` 表当作实时历史。Room-backed DM 的 Room 身份、标题与配置
来自 SQL；`message_count`、最近活动、上下文占用与 transcript lineage 从 Agent
workspace session 单调保留。群聊 conversation 的 `message_count` 从 canonical Room
ledger 重建并按 ledger 文件版本缓存；旧 SQL message row 仅作为迁移数据的兼容下限。
因此仅通过 Goal 启动的新会话在重连或刷新后仍保持标题、started 与非零消息进度。
Room-backed DM 的 Goal 标题只更新权威 SQL conversation，不再尝试修改只负责 runtime
进度的 workspace Session 投影。

| 方法 | 路径 | 说明 | 请求体 | 前端函数 |
|------|------|------|--------|---------|
| GET | `/goals/current` | 当前目标（query: `session_key`） | — | `getCurrentGoalApi` |
| POST | `/goals` | 直接创建 Goal lifecycle 状态；API 调用方可显式替换当前 Goal，且不写 host control history | `{ session_key, objective, token_budget?, replace_existing?, room_lead_agent_id?, metadata? }` | `createGoalApi`（Composer 不使用） |
| GET | `/goals/{goal_id}/usage` | 按 ID 查询目标的聚合 usage 与 finalization fence | — | `getGoalUsageApi` |
| GET | `/goals/{goal_id}/execution-binding` | owner-scoped、server-derived 的 Goal/Execution binding 状态 | — | `getGoalExecutionBindingApi` |
| PATCH | `/goals/{goal_id}` | HTTP lifecycle update，可改 objective/budget/metadata；不是 MCP terminal-only `update_goal` | `{ objective?, token_budget?, metadata? }` | `updateGoalApi` |
| POST | `/goals/{goal_id}/pause` | 暂停 | — | `pauseGoalApi` |
| POST | `/goals/{goal_id}/resume` | 恢复 | — | `resumeGoalApi` |
| POST | `/goals/{goal_id}/clear` | 清除 | — | `clearGoalApi` |
| GET | `/goals/{goal_id}/events` | 目标事件流 | — | — |

`POST /goals` 的 `replace_existing=true` 保留当前 Goal ID 与累计 usage。standalone/
reserved Goal 在同一 Goal row/event 事务内原位更新；confirmed WorkGraph 则按
successor saga 创建新的 Execution/Plan，不能理解为原位编辑既有 WorkGraph。

`room_lead_agent_id` 只用于 Room Goal 创建。服务端按认证 owner 与当前 Room 成员目录重新验证并解析负责人名称；所有 user-created Goal 的 `metadata` 都会在 Goal service 信任边界移除 owner、Execution binding、objective transition/revision 和 Room runtime 键。Room 的 creator、lead、scope 与 collaboration-required 门槛只能由验证后的服务端事实建立。

`POST /goals` 携 `replace_existing: true` 与 `PATCH /goals/{goal_id}` 携
`objective` 都进入同一 objective retarget 语义：保留 Goal ID 与累计 usage，
`standalone`/`reserved` 原位推进 objective revision；`confirmed` 进入 successor
Execution/Plan saga；`pending`/`conflict` 返回冲突或非法状态，不会旁路绑定 fence。

`GET /goals/{goal_id}/execution-binding` 从 durable Goal 与 Execution 的中央 resolver 即时计算，不新增持久化，也不允许客户端解析 Goal metadata 猜测绑定。返回 `state: standalone | reserved | pending | confirmed | conflict`；只有服务端证明一条 exact bilateral confirmed binding 时才返回 `execution_id`，reserved candidate identity 不对外暴露。clear 服务只接受 `standalone`/`reserved`，其余状态 fail closed。

Goal HTTP 的并发与绑定冲突（`goal conflict`、version/objective revision stale、Execution binding conflict）统一返回 `409`；invalid input/state（包括已确认绑定下禁止 clear）返回 `422`。

`Goal.usage` 同时暴露两套不可混用的 token 口径：

- `actual_tokens`：runtime/provider 实际处理总量，包含未缓存输入、cache creation/read、输出与 reasoning。provider terminal usage 显式携带一致的 `total_tokens` 时采用其非负值；显式 `0` 只有在所有 breakdown 也为零时才是精确值，若任一 breakdown 为正则把矛盾的零视为无效并按 breakdown 保守估算，同时设置 `actual_tokens_estimated=true`。逐 turn actual 按消息身份去重，terminal 时再用本轮累计真值对账并持久化；terminal provider usage 一旦收到，即使后续本地投影或持久化步骤失败，也不会退化为缺失值。
- `budget_tokens`：Goal 预算计量，严格为 `max(input_tokens, 0) + max(output_tokens, 0)`；cache creation/read 与 reasoning 不额外进入预算。`token_budget`、剩余预算和 `budget_limited` 均使用此值。
- `total_tokens`：为旧客户端保留的 `budget_tokens` 别名。

`GET /goals/{goal_id}/usage` 返回 `GoalUsageReport`。完成 Goal 不再出现在 `/goals/current` 中，但仍可用原 `goal_id` 查询；同 session 后续创建的新 Goal 不会继承旧 Goal 的 terminal usage。`status=complete` 只表示业务目标已完成，不代表用量已经冻结；`usage_finalized=true` 才是聚合值权威且不再接受迟到增量的唯一 fence。

DM 按当前 round 聚合 parent 与 child；Room 聚合同一 root round（包括后续 handoff）下所有 Agent slot 的 parent 与 child，并排除共享 runtime session 中属于其他 root 的工作。模型在 round 内创建 Goal 时，归属从该 round/root 的起点开始；外部 API 或 UI 激活 Goal 时，归属只从 runtime 激活边界开始，激活前的已结束工作不会回填进新 Goal。若 child 在激活前已经运行，而 runtime 无法在激活瞬间提供可信累计基线，该 child 后续只推进全局 checkpoint、不猜测归属给新 Goal，并将证据保持为 unavailable；绑定后新启动的 child 仍可精确归属。

每个 parent round 与 child task 都有按 owner、runtime session、round/root 和 source 身份隔离的持久化证据；terminal 写入、增量归属和 finalization fence 以幂等方式提交，使失败重试、idle 回收、handoff 与服务重启不会重复计数。最终 fence 只在 complete Goal 的全部必要 parent/child 证据收敛后建立；证据缺失、仍在运行或明确 unavailable 时均保持 `usage_finalized=false`，绝不会用伪造的 `0` 完成结算。fence 建立后的迟到增量会被显式拒绝。

当前 nxs child 适配器会在没有可信 child usage 时填充 `total_tokens: 0`；这个 `0` 是“未知”的占位值，不是 provider terminal 的精确零。child 只有终态消息中的正 total 才记为 authoritative terminal evidence；progress 的正 total 仍可进入 actual checkpoint，但若随后 terminal 为 `0` 或未提供 total，证据仍记为 unavailable，Goal 保持 `usage_finalized=false`。因此 provider terminal 的显式零可以精确结算，但不能把 nxs child 的占位零当作同一种证据。Claude Code 后台任务同样不会在缺少可验证累计语义时被冒充为精确增量。

`update_goal(complete)` 的结构化结果返回 `completionUsageCheckpointReport`、
`goalId` 与 `usageFinalized: false`；旧 `completionBudgetReport` 为兼容别名。
当前 round terminal 后仍可能按固定 `goal_id` 写回迟到 usage。需要精确审计的
调用方随后查询 `/goals/{goal_id}/usage`，并只把
`usage_finalized=true` 视为聚合冻结。

### App-Server 线程目标 RPC

这些 HTTP/JSON-RPC 入口是 Codex app-server 状态协议兼容面：它们直接读写 Goal
lifecycle，并可按 Goal 状态机触发 continuation，但不会追加 `/goal` host control
record，也不承担 Composer 的 started/message-count/title 语义。需要用户可见 Goal
控制记录的客户端必须使用 WebSocket `set_goal` 或文本 `/goal <objective>`。

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/app-server/thread/goal/set` | 设置线程目标 |
| POST | `/app-server/thread/goal/get` | 获取线程目标 |
| POST | `/app-server/thread/goal/clear` | 清除线程目标 |

App-Server Goal 保留 `tokensUsed` 作为预算兼容字段，同时返回 `budgetTokens`、`actualTokens` 与 `actualTokensEstimated`。WebSocket `thread/goal/*` RPC 的并发与绑定冲突使用 JSON-RPC server-error code `-32009`（不是 HTTP 409），并在 `error.data.reason_code` 返回稳定分类：`conflict`、`version_stale`、`revision_stale` 或 `execution_binding_conflict`；invalid state 仍返回 `-32600`，未知服务错误才返回 `-32603`。

WebSocket 使用独立 JSON-RPC envelope，不是上述 HTTP 路径的别名：

```json
{"jsonrpc":"2.0","id":"request-1","method":"thread/goal/get","params":{"threadId":"agent:..."}}
```

| `method` | `params` | `result` |
| --- | --- | --- |
| `thread/goal/set` | `{ threadId, objective?, status?, tokenBudget? }` | `{ goal: ThreadGoal }` |
| `thread/goal/get` | `{ threadId }` | `{ goal: ThreadGoal \| null }` |
| `thread/goal/clear` | `{ threadId }` | `{ cleared: boolean }` |

成功的 `set`/非空 `get` 才为该 authenticated owner + thread 注册 Goal RPC
订阅；失败请求不会订阅。后续通知为
`{ method: "thread/goal/updated", params: { threadId, turnId, goal } }` 或
`{ method: "thread/goal/cleared", params: { threadId } }`。订阅按 owner 与
thread 双重隔离。

---

## 17. Admin 订阅管理

> 仅管理员可见。

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| GET | `/admin/subscription/overview` | 订阅总览 | `getSubscriptionOverviewApi` |
| POST | `/admin/subscription/plans` | 创建套餐 | `createSubscriptionPlanApi` |
| PUT | `/admin/subscription/plans/{plan_key}` | 更新套餐 | `updateSubscriptionPlanApi` |
| PUT | `/admin/subscription/users/{user_id}` | 更新用户订阅 | `updateUserSubscriptionApi` |

### 管理员 Provider（订阅维度）

`/admin/subscription/providers/*` 与 `/settings/providers/*` 结构一致，额外管理订阅维度的 Provider 与模型：

| 方法 | 路径 |
|------|------|
| GET | `/admin/subscription/providers` |
| POST | `/admin/subscription/providers` |
| PUT | `/admin/subscription/providers/{provider}` |
| DELETE | `/admin/subscription/providers/{provider}` |
| POST | `/admin/subscription/providers/{provider}/models/fetch` |
| PUT | `/admin/subscription/providers/{provider}/models/{model_id}` |
| POST | `/admin/subscription/providers/{provider}/test` |
| POST | `/admin/subscription/providers/{provider}/models/{model_id}/test` |

对应前端：`listSubscriptionProviderConfigsApi`、`createSubscriptionProviderConfigApi`、`updateSubscriptionProviderConfigApi`、`deleteSubscriptionProviderConfigApi`、`fetchSubscriptionProviderModelsApi`、`updateSubscriptionProviderModelApi`、`testSubscriptionProviderConfigApi`、`testSubscriptionProviderModelApi`。

---

## 18. WebSocket 实时通信

### 连接

- 端点：`GET /chat/ws`（默认 `/nexus/v1/chat/ws`），协议升级为 WebSocket。
- Origin 白名单由后端 `AllowedWebSocketOrigins` 控制；未配置时兼容允许全部来源，生产环境应显式配置。
- 桌面端可使用子协议 `nexus-desktop`。
- 鉴权失败以关闭码 `4401` 通知前端，触发 `nexus:auth-required`。
- 读超时 90s，服务端每 30s 发 Ping；前端心跳间隔默认 30s、超时 10s。

### 客户端 → 服务端消息

消息体为 JSON，必含 `type` 字段。前端 `WebSocketClient.send` 支持离线排队的类型：`ping` / `bind_session` / `unbind_session` / `subscribe_room` / `unsubscribe_room` / `subscribe_workspace` / `unsubscribe_workspace` / `subscribe_app_events` / `unsubscribe_app_events`；业务消息（`chat` / `set_goal` / `interrupt` / `permission_response` / `input_queue`）不排队，连接不可用时直接丢弃。

| `type` | 说明 | 关键字段 |
|--------|------|---------|
| `ping` | 心跳 | — （回 `pong`） |
| `bind_session` | 绑定会话 | `session_key` |
| `unbind_session` | 解绑会话 | `session_key` |
| `subscribe_room` | 订阅房间事件 | `room_id` |
| `unsubscribe_room` | 取消订阅房间 | `room_id` |
| `subscribe_workspace` | 订阅工作区事件 | `agent_id` |
| `unsubscribe_workspace` | 取消订阅工作区 | `agent_id` |
| `subscribe_app_events` | 订阅应用事件 | — |
| `unsubscribe_app_events` | 取消订阅应用事件 | — |
| `chat` | 发送对话消息 | `session_key`, `agent_id?`, `room_id?`, `conversation_id?`, `content`, `attachments?`, `client_request_id`, `client_message_id`, `delivery_policy` |
| `set_goal` | Composer Goal 控制；与文本 `/goal` 共用 host handler | `session_key`, `objective`, `agent_id?`, `target_agent_ids?`, `client_request_id`, `client_message_id`, `goal_options?` |
| `interrupt` | 中断当前轮次 | `session_key`, `round_id`（DM）/ `msg_id`（Room） |
| `input_queue` | 输入队列操作 | `session_key`, `action`/`action_type`, `client_request_id?`, `client_message_id?`, `item_id?`, `content?`, `attachments?`, `ordered_ids?`, `delivery_policy` |
| `permission_response` | 权限请求响应 | 由权限运行时约定 |

> 带 `method` 字段的消息会进入 App-Server RPC 通道（`handleAppServerRPC`）。
> 当前 `thread/goal/set|get|clear` 的 params、result、订阅和通知契约见第 16 节。

### `chat` 消息字段说明

- `delivery_policy`：投递策略，由 `protocol.NormalizeChatDeliveryPolicy` 归一化（如 `queue` / `immediate`）。
- `attachments`：附件列表，经 `protocol.ChatAttachmentsFromAny` 解析。
- `client_request_id`：单次 WebSocket 发送尝试，用于匹配服务端 ACK 或错误事件。
- `client_message_id`：逻辑消息身份；`input_queue enqueue` 在 ACK 未知后重试时必须复用，用于后端持久化幂等去重。
- Room 会话额外支持 `room_id`、`conversation_id`、`agent_id`（附件归属 Agent）。
- `chat.content` 匹配大小写不敏感的 `/goal <non-empty objective>` 命令语法时先走
  Nexus host command，不进入普通 runtime；命令名和 objective 两侧空白会被裁剪，
  空 objective 返回 `用法：/goal <objective>`，携带附件的 host Slash 会被拒绝；
  `set_goal.goal_options` 支持 `token_budget?`、`replace_existing?` 与客户端可写
  `metadata?`，Room 的 `target_agent_ids` 必须解析为一个经服务端成员校验的 lead。

### 服务端 → 客户端事件

服务端通过 `WebSocketSender.SendEvent` 推送 `event_type` 标识的事件，前端 `onMessage` 回调统一消费。常见事件由 `internal/protocol` 构造，包括：

- `pong` — 心跳响应。
- `chat_ack` — 对话消息受理确认，回传 `client_request_id` / `client_message_id` 与后端生成的 canonical round/message identity。
- `input_queue_ack` — 用户入队请求持久化确认，仅向请求连接单播；回传 `client_request_id`、稳定 `client_message_id`、canonical `item_id` 与 `duplicate`。共享队列当前状态仍由 `input_queue` 快照表达。
- `round_status` — 轮次状态变更（`running` / `finished` / `interrupted` / `error`）；失败终态可在 `data.message` 携带可展示原因。
- `runtime_status` — Runtime 瞬时阶段；`status: "compacting"` 表示正在压缩上下文，`status: null` 清除该阶段。
- `execution_invalidated` — owner + session 双重隔离的 managed WorkGraph 读取失效通知；只投递给已 `bind_session` 的匹配连接，前端收到后重新调用 `GET /executions/latest`，30 秒轮询仅作为 active 图的断线恢复兜底。
- `gateway_error` — 网关错误（`error_type` 含 `chat_error` / `interrupt_error` / `input_queue_error` / `not_implemented` / `unknown_message_type` / `permission_request_not_found` 等）。
- Room / Workspace / App Event 订阅渠道推送的实时事件（房间消息、工作区文件变更、应用级事件）。
- Goal 事件广播（经 `goal_event_broadcaster` 推送到 `goalRPCSubs`）。

事件模型在 `internal/protocol/event.go` 定义，前端类型见 `web/src/types/generated/protocol.ts`（由 `tools/protocol-tsgen` 生成）。

---

## 附：路径前缀与别名

- **API 前缀**：所有路径默认带 `/nexus/v1` 前缀。下表以外，后端还为部分能力提供等价的别名路径，前端优先使用结构化路径：
  - `/capability/scheduled/*` ↔ `/scheduled/*`（定时任务扁平别名）
- **静态资源**：`mountWebAppRoutes` 托管 Vite 构建产物，`/assets/*` 长缓存（immutable），HTML 文件 `no-cache`；非 API 路径回退到 `index.html` / `app.html` / `settings.html` / `oauth-callback.html`。
