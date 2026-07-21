# OpenAI Responses runtime 集成

## 1. 目标与边界

Nexus 允许 `api_format=responses` 的 LLM Provider 作为 `nxs` Agent runtime 的主模型或辅助视觉模型。Claude Code runtime 不支持该格式，继续在 Provider 解析阶段明确拒绝。

Nexus 是宿主和配置真相源，不解析 Responses request、typed SSE、reasoning item 或 tool item。Responses wire、无状态历史重放、session-stable `prompt_cache_key`、缓存 usage 和 terminal event 校验都归 `nxs` 所在的 `nexus-agent-sdk-go` 负责。bridge 继续传递 provider-neutral stream-json 消息以及既有环境更新 control request，不增加 Responses 专用 wire。

## 2. Runtime 兼容矩阵

| API format | `nxs` | `claude` |
| --- | --- | --- |
| `anthropic_messages` | 支持 | 支持 |
| `chat_completions` | 支持 | 拒绝 |
| `responses` | 支持 | 拒绝 |

Provider 和模型下拉必须按当前 Agent runtime 过滤。Responses Provider 可出现在 `nxs` 选项中，但不能出现在 Claude Code 选项中；显式选择不兼容组合时，错误消息应提示切换到 `nxs`。

## 3. 主模型环境投影

| Nexus `RuntimeConfig` | `nxs` process env |
| --- | --- |
| `Provider=<产品 Provider ID>` | `NEXUS_RUNTIME_PROVIDER=<同一 ID>` |
| `APIFormat=responses` | `NEXUS_API_PROVIDER=openai` |
| `APIFormat=responses` | `NEXUS_OPENAI_PROTOCOL=responses` |
| `AuthToken` | `OPENAI_API_KEY` |
| `BaseURL` | `OPENAI_BASE_URL` |
| `Model` | `OPENAI_MODEL`、`NEXUS_SUBAGENT_MODEL` |
| `ContextWindow` | `NEXUS_MAX_CONTEXT_TOKENS` |
| 模型视觉能力 | `NEXUS_MODEL_SUPPORTS_VISION` 与多模态 content flags |

`NEXUS_API_PROVIDER` 保持 `openai`，以免把 Nexus 产品 Provider ID、SDK adapter 和 HTTP 协议混成同一概念；`NEXUS_OPENAI_PROTOCOL` 单独选择 Chat Completions 或 Responses。两种 OpenAI 协议都显式投影 protocol 值，避免宿主环境中的旧值污染新会话。

## 4. 辅助视觉模型

辅助视觉模型使用独立的 `NEXUS_VISION_*` 命名空间，不覆盖主模型路由：

- `anthropic_messages` → `NEXUS_VISION_API_PROVIDER=anthropic-compatible`
- `chat_completions` → `NEXUS_VISION_API_PROVIDER=openai`
- `responses` → `NEXUS_VISION_API_PROVIDER=responses`

Responses 不能投影为普通 `openai`，否则 SDK 会把辅助视觉请求错误路由到 `/chat/completions`。

## 5. 会话、缓存与热更新

- Nexus 不生成 `previous_response_id`，也不保存 Responses item；当前 SDK 使用 `store=false` 和本地完整历史重放。
- Nexus 不生成缓存 key。SDK 为 OpenAI 官方端点生成 session-stable `prompt_cache_key`，并从 Responses usage 归一化 cache read/write token。
- 兼容网关默认不接收 OpenAI 私有缓存控制；确认兼容后，可在启动 Nexus 时显式设置 `NEXUS_OPENAI_PROMPT_CACHE=1`。宿主允许继续透传 `NEXUS_OPENAI_PROMPT_CACHE_MODE=implicit|explicit`、`NEXUS_OPENAI_PROMPT_CACHE_TTL=30m` 与旧模型的 `NEXUS_OPENAI_PROMPT_CACHE_RETENTION=24h|in_memory`；具体合法组合和 breakpoint 归 SDK 校验，bridge 不解释这些值。
- Provider 环境改变时，bridge 通过既有 `update_environment_variables` control request 把差异交给 `nxs`，SDK 随后重建 provider；没有 Responses 专用 control subtype。

## 6. 本地联调

当前 Nexus worktree 必须配合包含 Responses adapter 的 `nexus-agent-sdk-go` worktree 或已发布的新版本 `nxs`。尚未更新的打包版 `nxs` 不具备该协议实现。

先在 SDK worktree 构建本地 runtime：

```bash
go build -o ./bin/nxs-responses ./cmd/nxs
```

当前 Responses worktree 的 `.env` 已指向上一步产物，并固定使用 Web `3003`、Backend `8013`，因此可以直接运行：

```bash
make dev
```

若在其它 worktree 复用这份说明，则必须显式配置 `NEXUS_NXS_COMMAND_PATH`，并选择不冲突的 `BACKEND_PORT`/`WEB_PORT`。`BACKEND_PORT` 会同时传给 Go backend 和 Vite 的 `/nexus/v1` 代理。

在 Settings 中完成以下配置：

1. 将 Agent Runtime 设为 `nxs`。
2. 创建或编辑 LLM Provider，将 API Format 设为 `Responses (/responses)`。
3. 配置 Base URL、token 和模型，启用模型并设为默认，或在 Agent 上显式选择。
4. 新建 DM 发起请求。启动日志应同时出现 `api_provider_env=openai` 与 `openai_protocol_env=responses`。
5. 上游应收到 `POST <base_url>/responses`，流式响应必须是 Responses typed SSE，而不是 Chat Completions `data.choices[].delta`。

Provider 的 `base_url` 可以是 API base，也可以是已包含 `/responses` 的完整 operation URL；模型测试解析路径时不得重复追加 operation path，并必须把原 URL 的 query 参数保留在最终 URL 末尾。Responses 探测与后端轻量请求都显式使用 `store=false`，探测请求的 `max_output_tokens` 不得低于 16。Azure OpenAI/Foundry 使用 API key 时，模型测试、后端轻量请求和 nxs Agent 主链都同时发送兼容的 `api-key` 鉴权头。Azure OpenAI 的资源根 `https://<resource>.openai.azure.com/openai`、显式 `/openai/v1` 和 Foundry project endpoint 都归一化为最终的 `/openai/v1/responses`；旧的 `/deployments/...`、`/images/generations` 与 `/chat/completions` operation URL 不能复用为 Responses base URL。

建议至少验证文本流、function call 参数流、工具结果续轮、图片输入、取消、compact 后续轮，以及 usage 中 cached token。兼容网关只有在这些能力逐项通过后，才能视为完整 Responses backend。

不启动 Nexus 服务也可以运行真实进程级本地回归。该测试从 Nexus `RuntimeConfig` 生成 bridge options，启动指定 nxs，并用本地 mock 断言最终请求 `/v1/responses`：

```bash
NEXUS_TEST_NXS_RESPONSES_COMMAND=/absolute/path/to/nxs-responses \
  go test ./internal/runtime/clientopts -run TestNexusResponsesRuntimeProcessIntegration -count=1 -v
```

## 7. 官方参考

- [Migrate to the Responses API](https://developers.openai.com/api/docs/guides/migrate-to-responses)
- [Streaming Responses](https://developers.openai.com/api/docs/guides/streaming-responses)
- [Prompt caching](https://developers.openai.com/api/docs/guides/prompt-caching)
