# OpenAI Responses runtime 集成

> Status: current product integration reference

## 1. 范围与开源边界

Nexus 允许 `api_format=responses` 的 Provider 用作 `nxs` Agent runtime 的主模型
或辅助视觉模型。`nxs` 是随 Nexus 发布的闭源 runtime 可执行程序，其内部实现不
属于本开源仓库；Nexus 只依赖公开的 bridge 进程协议和 runtime capability。

职责边界如下。

- Nexus 保存 Provider、模型、凭据引用和 Agent runtime 选择。
- Nexus 把当前配置投影为 runtime 进程环境。
- 开源 bridge 负责启动进程、传递 provider-neutral `stream-json` 消息和环境更新。
- `nxs` 负责 Responses 请求、typed SSE、工具循环和 usage 归一化。

Nexus 和 bridge 都不解析或重写 `/responses` payload。

## 2. Runtime 兼容矩阵

| API format | `nxs` | `claude` |
| --- | --- | --- |
| `anthropic_messages` | 支持 | 支持 |
| `chat_completions` | 支持 | 不支持 |
| `responses` | 支持 | 不支持 |

Provider 与模型选择器必须按当前 Agent runtime 过滤。显式选择不兼容组合时，
产品应返回可操作的错误，而不是在 runtime 启动后静默降级。

## 3. 主模型环境投影

| Nexus `RuntimeConfig` | `nxs` process environment |
| --- | --- |
| `Provider` | `NEXUS_RUNTIME_PROVIDER` |
| `APIFormat=responses` | `NEXUS_API_PROVIDER=openai` |
| `APIFormat=responses` | `NEXUS_OPENAI_PROTOCOL=responses` |
| `AuthToken` | `OPENAI_API_KEY` |
| `BaseURL` | `OPENAI_BASE_URL` |
| `Model` | `OPENAI_MODEL`、`NEXUS_SUBAGENT_MODEL` |
| `ContextWindow` | `NEXUS_MAX_CONTEXT_TOKENS` |
| 模型视觉能力 | `NEXUS_MODEL_SUPPORTS_VISION` |

`NEXUS_RUNTIME_PROVIDER` 保留 Nexus 中的 Provider 身份，`NEXUS_API_PROVIDER`
选择 runtime adapter，`NEXUS_OPENAI_PROTOCOL` 再区分 Chat Completions 与
Responses。这三个概念不能合并成一个字段。

辅助视觉模型使用独立的 `NEXUS_VISION_*` 命名空间。Responses 视觉模型投影为
`NEXUS_VISION_API_PROVIDER=responses`，不会覆盖主模型路由。

## 4. 会话与热更新

- Provider 环境只在 runtime 子进程中使用，不写入 Agent workspace。
- 活跃 `nxs` 会话通过 bridge 的 `update_environment_variables` control request
  接收环境差异，并由 runtime 重建 Provider。
- Claude Code runtime 不接受 OpenAI 环境热更新；切换 runtime 或环境时需要替换
  进程。
- Prompt cache 相关环境值由 Nexus 原样传递，bridge 不校验模型兼容性或缓存策略。

## 5. Base URL 与连通性检查

Provider 模型测试接受 API base URL 或已经包含 `/responses` 的 operation URL，
并保留原始 query 参数。Azure OpenAI 与 Foundry endpoint 会在产品侧归一化为
对应的 Responses operation；旧的 deployment、image generation 或
`/chat/completions` operation URL 不会被当作 Responses base URL 复用。

连通性检查和 runtime 主链都必须保持相同的认证方式、模型和 operation 解析规则。
兼容网关只有在文本流、工具调用、工具结果续轮、图片输入、取消和 usage 都通过后，
才能视为完整的 Responses backend。

## 6. 配置与排障

1. 将 Agent runtime 设为 `nxs`。
2. 创建或编辑 LLM Provider，将 API Format 设为 `Responses (/responses)`。
3. 配置 Base URL、凭据和模型，并启用该模型。
4. 发起新会话，确认后端日志中的 Provider 与 protocol 均为 Responses 配置。
5. 若 runtime 报启动配置错误，先确认当前 Nexus 发布包包含可执行的 `nxs`，不要
   从本仓库寻找或构建其闭源实现。

本地源码开发可以通过 `NEXUS_NXS_COMMAND_PATH=/absolute/path/to/nxs` 指向一个
已获得的 runtime 可执行程序。Nexus 仓库不下载、构建或发布该二进制的源代码。

## 7. 参考

- [OpenAI Responses migration guide](https://developers.openai.com/api/docs/guides/migrate-to-responses)
- [OpenAI streaming guide](https://developers.openai.com/api/docs/guides/streaming-responses)
- [OpenAI prompt caching guide](https://developers.openai.com/api/docs/guides/prompt-caching)
