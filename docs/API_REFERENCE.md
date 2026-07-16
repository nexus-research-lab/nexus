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
- [15. Goal 目标](#15-goal-目标)
- [16. Admin 订阅管理](#16-admin-订阅管理)
- [17. WebSocket 实时通信](#17-websocket-实时通信)
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
| GET | `/system/version` | 系统版本信息（project/version/git_commit/build_date/goos/goarch/target/release_url） | `getSystemVersionApi` |
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

---

## 4. Agent 管理

| 方法 | 路径 | 说明 | 请求体 / 参数 | 前端函数 |
|------|------|------|---------------|---------|
| GET | `/agents` | Agent 列表 | — | `getAgents` |
| GET | `/agents/runtime/statuses` | Agent 运行时状态批量查询 | — | — |
| POST | `/agents` | 创建 Agent | `{ name, options, avatar, description, vibe_tags }` | `createAgentApi` |
| GET | `/agents/validate/name` | 校验名称（query: `name`, `exclude_agent_id`） | — | `validateAgentNameApi` |
| GET | `/agents/{agent_id}` | Agent 详情 | — | — |
| PATCH | `/agents/{agent_id}` | 更新 Agent | `{ name, options, avatar, description, vibe_tags }` | `updateAgentApi` |
| DELETE | `/agents/{agent_id}` | 删除 Agent | — | `deleteAgentApi` |
| GET | `/agents/{agent_id}/sessions` | Agent 的会话列表 | — | `getAgentSessionsApi` |
| GET | `/agents/{agent_id}/private-domain/threads` | 私域线程列表 | — | — |
| GET | `/agents/{agent_id}/private-domain/threads/{thread_id}/events` | 私域线程事件 | — | — |

### Agent 技能挂载

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| GET | `/agents/{agent_id}/skills` | Agent 已安装技能 | `getAgentSkillsApi` |
| POST | `/agents/{agent_id}/skills` | 安装技能（body: `{ skill_name }`） | `installSkillApi` |
| DELETE | `/agents/{agent_id}/skills/{skill_name}` | 卸载技能 | `uninstallSkillApi` |

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
| GET | `/agents/{agent_id}/workspace/file` | 读文件内容 | query: `path` | `getWorkspaceFileContentApi` |
| PUT | `/agents/{agent_id}/workspace/file` | 写文件内容 | `{ path, content }` | `updateWorkspaceFileContentApi` |
| POST | `/agents/{agent_id}/workspace/upload` | 上传文件 | FormData: `file`, `path?` | `uploadWorkspaceFileApi` |
| GET | `/agents/{agent_id}/workspace/download` | 下载文件 | query: `path`, `disposition=attachment\|inline` | `downloadWorkspaceFileApi` / `getWorkspaceFilePreviewUrl` |
| POST | `/agents/{agent_id}/workspace/reveal` | 在文件夹中定位 | `{ path }` | 桌面端调用 |
| POST | `/agents/{agent_id}/workspace/entry` | 新建文件/目录 | `{ path, entry_type, content }` | `createWorkspaceEntryApi` |
| PATCH | `/agents/{agent_id}/workspace/entry` | 重命名 | `{ path, new_path }` | `renameWorkspaceEntryApi` |
| DELETE | `/agents/{agent_id}/workspace/entry` | 删除条目 | query: `path` | `deleteWorkspaceEntryApi` |

桌面端调用 `reveal`；浏览器端通过 `download` 接口下载文件。

长期记忆由内置 `nxs` SDK 子进程维护为 Agent 工作区中的 `MEMORY.md` 索引与 `memory/` 主题文件。Nexus 不参与提取或召回；只提供只读工作区投影供 Web 展示，正文编辑仍使用通用工作区文件接口。

---

## 7. Skill 技能

### 全局技能市场

| 方法 | 路径 | 说明 | 前端函数 |
|------|------|------|---------|
| GET | `/skills` | 全部技能（query: `agent_id`,`category_key`,`source_type`,`scope`,`q`） | `getAvailableSkillsApi` |
| GET | `/skills/{skill_name}` | 技能详情（query: `agent_id`） | `getSkillDetailApi` |
| POST | `/skills/import/local` | 导入本地技能（FormData: `file` 或 `local_path`） | `importLocalSkillApi` |
| POST | `/skills/import/git` | 从 Git 仓库导入（body: `{ url, branch, path }`） | `importGitSkillApi` |
| GET | `/skills/search/external` | 搜索社区技能（query: `q`,`include_readme`） | `searchExternalSkillsApi` |
| GET | `/skills/external/preview` | 社区技能预览（query: `detail_url`） | `getExternalSkillPreviewApi` |
| POST | `/skills/import/skills-sh` | 从社区来源导入 | `importExternalSkillApi` |
| GET | `/skills/sources` | 社区来源配置列表 | `listExternalSkillSourcesApi` |
| PATCH | `/skills/sources/{source_id}` | 更新来源配置 | `updateExternalSkillSourceApi` |
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

## 15. Goal 目标

| 方法 | 路径 | 说明 | 请求体 | 前端函数 |
|------|------|------|--------|---------|
| GET | `/goals/current` | 当前目标（query: `session_key`） | — | `getCurrentGoalApi` |
| POST | `/goals` | 创建目标；UI 可显式原位替换当前目标 | `{ session_key, objective, token_budget?, replace_existing?, metadata? }` | `createGoalApi` |
| PATCH | `/goals/{goal_id}` | 更新目标 | `{ objective?, token_budget?, metadata? }` | `updateGoalApi` |
| POST | `/goals/{goal_id}/pause` | 暂停 | — | `pauseGoalApi` |
| POST | `/goals/{goal_id}/resume` | 恢复 | — | `resumeGoalApi` |
| POST | `/goals/{goal_id}/clear` | 清除 | — | `clearGoalApi` |
| GET | `/goals/{goal_id}/events` | 目标事件流 | — | — |

### App-Server 线程目标 RPC

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/app-server/thread/goal/set` | 设置线程目标 |
| POST | `/app-server/thread/goal/get` | 获取线程目标 |
| POST | `/app-server/thread/goal/clear` | 清除线程目标 |

---

## 16. Admin 订阅管理

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

## 17. WebSocket 实时通信

### 连接

- 端点：`GET /chat/ws`（默认 `/nexus/v1/chat/ws`），协议升级为 WebSocket。
- Origin 白名单由后端 `AllowedWebSocketOrigins` 控制；未配置时兼容允许全部来源，生产环境应显式配置。
- 桌面端可使用子协议 `nexus-desktop`。
- 鉴权失败以关闭码 `4401` 通知前端，触发 `nexus:auth-required`。
- 读超时 90s，服务端每 30s 发 Ping；前端心跳间隔默认 30s、超时 10s。

### 客户端 → 服务端消息

消息体为 JSON，必含 `type` 字段。前端 `WebSocketClient.send` 支持离线排队的类型：`ping` / `bind_session` / `unbind_session` / `subscribe_room` / `unsubscribe_room` / `subscribe_workspace` / `unsubscribe_workspace` / `subscribe_app_events` / `unsubscribe_app_events`；业务消息（`chat` / `interrupt` / `permission_response` / `input_queue`）不排队，连接不可用时直接丢弃。

| `type` | 说明 | 关键字段 |
|--------|------|---------|
| `ping` | 心跳 | — （回 `pong`） |
| `bind_session` | 绑定会话 | `session_key` |
| `unbind_session` | 解绑会话 | `session_key` |
| `subscribe_room` | 订阅房间事件 | `room_id` |
| `unsubscribe_room` | 取消订阅房间 | `room_id` |
| `subscribe_workspace` | 订阅工作区事件 | `agent_id?` |
| `unsubscribe_workspace` | 取消订阅工作区 | — |
| `subscribe_app_events` | 订阅应用事件 | — |
| `unsubscribe_app_events` | 取消订阅应用事件 | — |
| `chat` | 发送对话消息 | `session_key`, `agent_id?`, `room_id?`, `conversation_id?`, `content`, `attachments?`, `client_request_id`, `client_message_id`, `delivery_policy` |
| `interrupt` | 中断当前轮次 | `session_key`, `round_id`（DM）/ `msg_id`（Room） |
| `input_queue` | 输入队列操作 | `session_key`, `action`/`action_type`, `client_request_id?`, `client_message_id?`, `item_id?`, `content?`, `attachments?`, `ordered_ids?`, `delivery_policy` |
| `permission_response` | 权限请求响应 | 由权限运行时约定 |

> 带 `method` 字段的消息会进入 App-Server RPC 通道（`handleAppServerRPC`），用于 Goal 等线程级 RPC。

### `chat` 消息字段说明

- `delivery_policy`：投递策略，由 `protocol.NormalizeChatDeliveryPolicy` 归一化（如 `queue` / `immediate`）。
- `attachments`：附件列表，经 `protocol.ChatAttachmentsFromAny` 解析。
- `client_request_id`：单次 WebSocket 发送尝试，用于匹配服务端 ACK 或错误事件。
- `client_message_id`：逻辑消息身份；`input_queue enqueue` 在 ACK 未知后重试时必须复用，用于后端持久化幂等去重。
- Room 会话额外支持 `room_id`、`conversation_id`、`agent_id`（附件归属 Agent）。

### 服务端 → 客户端事件

服务端通过 `WebSocketSender.SendEvent` 推送 `event_type` 标识的事件，前端 `onMessage` 回调统一消费。常见事件由 `internal/protocol` 构造，包括：

- `pong` — 心跳响应。
- `chat_ack` — 对话消息受理确认，回传 `client_request_id` / `client_message_id` 与后端生成的 canonical round/message identity。
- `input_queue_ack` — 用户入队请求持久化确认，仅向请求连接单播；回传 `client_request_id`、稳定 `client_message_id`、canonical `item_id` 与 `duplicate`。共享队列当前状态仍由 `input_queue` 快照表达。
- `round_status` — 轮次状态变更（`running` / `completed` / `error` 等）。
- `runtime_status` — Runtime 瞬时阶段；`status: "compacting"` 表示正在压缩上下文，`status: null` 清除该阶段。
- `gateway_error` — 网关错误（`error_type` 含 `chat_error` / `interrupt_error` / `input_queue_error` / `not_implemented` / `unknown_message_type` / `permission_request_not_found` 等）。
- Room / Workspace / App Event 订阅渠道推送的实时事件（房间消息、工作区文件变更、应用级事件）。
- Goal 事件广播（经 `goal_event_broadcaster` 推送到 `goalRPCSubs`）。

事件模型在 `internal/protocol/event.go` 定义，前端类型见 `web/src/types/generated/protocol.ts`（由 `tools/protocol-tsgen` 生成）。

---

## 附：路径前缀与别名

- **API 前缀**：所有路径默认带 `/nexus/v1` 前缀。下表以外，后端还为部分能力提供等价的别名路径，前端优先使用结构化路径：
  - `/capability/scheduled/*` ↔ `/scheduled/*`（定时任务扁平别名）
- **静态资源**：`mountWebAppRoutes` 托管 Vite 构建产物，`/assets/*` 长缓存（immutable），HTML 文件 `no-cache`；非 API 路径回退到 `index.html` / `app.html` / `settings.html` / `oauth-callback.html`。
