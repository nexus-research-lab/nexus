# Engineering Structure File Audit - 2026-06-16

本文件是文件级结构审计结论，配套全量登记表：

- `docs/specs/engineering-structure-file-register-2026-06-16.csv`
- 口径：当前存在的 tracked 代码文件，不包含已删除文件、构建产物、依赖目录和运行数据。
- 2026-06-22 复查：登记表已移除不存在的路径；因后续本地提交持续新增文件，具体结论仍以当前 `git ls-files` / 工作区为准。
- 覆盖：Go、TS/TSX、Swift、scripts。
- 总量：1454 个代码文件，包含 tracked 文件和当前本地新增未跟踪代码文件。
- 规模信号：生产代码 8 个文件超过 800 行，21 个文件在 500-799 行；全量含测试时 26 个超过 800 行，35 个在 500-799 行。

## 审计标准

结构判断不只看行数，重点看以下问题：

1. 文件是否只承担一个清晰职责。
2. 文件名是否能准确表达职责。
3. 同一文件里是否混入入口编排、业务规则、持久化、协议转换、HTTP/API 调用、UI 展示和工具函数。
4. Go 包边界是否符合现有约定：服务在 `internal/service/*`，协议真相源在 `internal/protocol`，仓储在 `internal/storage/*`，runtime 在 `internal/runtime`。
5. TS/TSX 是否符合页面薄、feature 聚合、hook 控制状态、组件负责展示、lib/api 负责请求的边界。
6. app / web / Docker 是否存在启动、构建、运行时 IO、缓存和网络请求路径上的性能优化空间。
7. 拆分必须行为不变，先做同包/同目录文件拆分，再考虑目录迁移。

## 当前结论

不是所有大文件都是屎山，但目前确实有几类高风险：

1. Provider 服务层、Provider storage repository、workspace service / history / input queue / history normalize / live manager、workspace memory engine、runtime manager、runtime executor round、room input queue / execution / chat / round state、websocket handler 和 imagegen service 已完成同包拆分；frontend conversation hook、settings provider panel、editor panel、composer panel 和 presentation preview 已完成第一层 helper 拆分。
2. 四块目标功能里，goal 当前结构最好；定时器经过拆分后可控；IM 通道的 adapter、router、ingress、pairing、login、DingTalk 已完成第一轮拆分；连接器 service 层、Feishu Docx client/tool 和 markdown renderer 已完成第一轮拆分。
3. 前端比后端更需要系统整理，大文件集中在 settings、conversation、editor preview、shared sidebar。
4. app / web / Docker 性能有优化空间，但不是一个“先大改”的问题：当前已有 HTTP timeout、nginx gzip/cache、Vite route lazy、Docker BuildKit cache 和构建上下文本地产物排除；更适合按缓存、bundle、镜像 profile 分阶段优化。
5. 当前不建议大规模搬目录。更稳妥的方式是先保持 package/public API 不变，在同目录按职责拆文件。

## 已落地的本地结构整理

这些是已在本地完成的行为不变拆分，尚未推送：

| 原文件 | 当前处理 |
| --- | --- |
| `internal/service/connectors/service.go` | 拆为 `service_connection.go`、`service_oauth_flow.go`、`service_device_flow.go`、`service_listing.go`、`service_oauth_config.go`、`credential_payload.go`、`model.go`，入口文件保留构造和依赖 |
| `internal/service/channels/adapters/feishu.go` | 拆为 `feishu_channel.go`、`feishu_api.go`、`feishu_ingress.go`、`feishu_callback_security.go`、`feishu_model.go` |
| `internal/service/channels/adapters/wecom_bot.go` | 拆为 `wecom_bot_channel.go`、`wecom_bot_socket.go`、`wecom_bot_ingress.go`、`wecom_bot_frame.go` |
| `internal/service/channels/adapters/personal_weixin.go` | 拆为 `personal_weixin_channel.go`、`personal_weixin_client.go`、`personal_weixin_model.go`、`personal_weixin_util.go` |
| `internal/service/automation/service_task.go` | 拆为 `service_task_crud.go`、`service_task_query.go`、`service_task_run.go`、`service_task_support.go` |
| `internal/service/automation/service_heartbeat.go` | 拆为 `service_heartbeat_runtime.go`、`service_heartbeat_dispatch.go`、`service_heartbeat_status.go` |
| `internal/service/goal/service_progress.go` | 去掉重复 stale-version retry 包装，集中到 `retryGoalProgressMutation` |
| `internal/service/goal/service_continuation_test.go` | 拆为 `service_steering_test.go`、`service_continuation_plan_test.go`、`service_continuation_progress_test.go`，按 guidance steering、continuation planning 和 progress/completion miss 隔离 |
| `internal/runtime/mcp/connectors/tool/feishu_docx.go` | 拆为 `feishu_docx_document_tools.go`、`feishu_docx_sheet_tools.go`、`feishu_docx_bitable_tools.go`、`feishu_docx_drive_wiki_tools.go`、`feishu_docx_client.go` |
| `internal/connectors/feishudocx/client.go` | 拆为 `client.go`、`client_document.go`、`client_drive.go`、`client_transport.go`、`client_block_codec.go` |
| `internal/connectors/feishudocx/render_markdown.go` | 拆为 `target_document.go`、`render_block.go`、`render_inline.go`、`render_table.go`、`render_util.go`，入口文件只保留 markdown renderer 主流程；过薄 media helper 已收回 |
| `internal/service/provider/service.go` | 拆为 core、listing、mutation、runtime config、image config、model selection、normalization、visibility、record、store view、id 等职责文件 |
| `internal/service/provider/service_models.go` | 拆为 `service_model_mutation.go`、`service_model_discovery.go`、`service_model_check.go`、`service_model_http.go`、`service_model_card.go`、`service_model_record.go`、`service_model_codec.go`、`service_model_sanitize.go` |
| `internal/service/provider/service_test.go` | 拆为 `service_catalog_runtime_test.go`、`service_visibility_mutation_test.go`、`service_model_discovery_test.go`、`service_model_mutation_test.go`、`service_provider_http_test.go`、`service_test_helpers_test.go`，按 catalog/runtime、visibility/admin、模型发现、模型变更、HTTP payload/redaction 和共享夹具隔离 |
| `internal/storage/workspace/store_agent_history.go` | 拆为 facade、model、overlay、read、transcript cache、reader、project、guidance、marker、path、session、util 等职责文件 |
| `internal/storage/workspace/store_agent_history_test.go` | 拆为 `store_agent_history_round_marker_test.go`、`store_agent_history_goal_context_test.go`、`store_agent_history_transcript_projection_test.go`、`store_agent_history_overlay_test.go`，按 round marker、Goal 隐藏上下文、transcript 投影和 overlay 控制行隔离测试 |
| `internal/storage/workspace/store_input_queue.go` | 拆为 `store_input_queue_file.go`、`store_input_queue_replay.go`、`store_input_queue_codec.go`、`store_input_queue_order.go`，入口文件只保留队列对外操作 |
| `internal/storage/workspace/history_normalize.go` | 拆为 `history_pagination.go`、`history_result_summary.go`、`history_external_delivery.go`、`history_unfinished_round.go`、`history_order.go`，入口文件只保留历史归一化 facade |
| `internal/storage/workspace/history_normalize_test.go` | 拆出 `history_pagination_test.go`、`history_unfinished_round_test.go`，原文件只保留 normalize 合并/过滤基础场景 |
| `internal/storage/workspace/store_session_file.go` | 拆为 `store_jsonl.go`、`history_compact.go`、`value_coerce.go`，入口文件只保留 session meta 文件 CRUD |
| `internal/service/workspace/manager_live.go` | 拆为 `manager_live_watcher.go`、`manager_live_write.go`、`manager_live_util.go`、`manager_live_diff.go`，入口文件只保留订阅生命周期和 watcher 状态类型 |
| `internal/service/workspace/service.go` | 拆为 `service_model.go`、`service_agent.go`、`service_file.go`、`service_mutation.go`、`service_upload.go`、`service_path.go`，入口文件只保留 Service 核心和 live facade |
| `internal/service/workspace/service_test.go` | 拆为 `initializer_test.go`、`manager_live_test.go`、`upload_dedupe_test.go`、`service_test_helpers_test.go`，按初始化、live、上传去重和测试基础设施隔离 |
| `internal/service/workspace/initializer_workspace.go` | 拆为 `initializer_template.go`、`initializer_skill.go`、`initializer_nexusctl.go`、`initializer_util.go`，入口文件只保留初始化主编排和 workspace lock |
| `internal/runtime/manager.go` | 拆为 `manager_client.go`、`manager_session.go`、`manager_round.go`、`manager_goal_accounting.go`、`manager_interrupt.go`、`manager_streaming_input.go`、`manager_mcp.go`，入口文件只保留 manager state、构造和内部状态初始化 |
| `internal/service/channels/router.go` | 拆为 `router_registry.go`、`router_lifecycle.go`、`router_routes.go`、`router_delivery.go`，入口文件只保留 Router 类型、构造和 logger 注入 |
| `internal/service/channels/router_test.go` | 拆为 `router_delivery_test.go`、`router_test_helpers_test.go` 和保留 lifecycle/config 的 `router_test.go`，按投递路径、共享夹具和注册/配置隔离 |
| `internal/service/channels/ingress.go` | 拆为 `ingress_accept.go`、`ingress_normalize.go`、`ingress_session.go`、`ingress_permission.go`、`ingress_delivery_target.go`，入口文件只保留 IngressService 类型、构造和 setter |
| `internal/service/channels/ingress_test.go` | 拆为 `ingress_test.go`、`ingress_weixin_test.go`、`ingress_permission_test.go`、`ingress_dedupe_test.go`、`ingress_test_helpers_test.go`，按基础 accept/session、个人微信隔离、权限策略、reqID 幂等和测试夹具隔离 |
| `internal/service/channels/service_pairing.go` | 拆为 `service_pairing_ingress.go`、`service_pairing_store.go`、`service_pairing_view.go`，入口文件只保留 pairing CRUD |
| `internal/service/channels/service_login.go` | 拆为 `service_login_flow.go`、`service_login_weixin.go`、`service_login_store.go`、`service_login_session.go`，入口文件只保留 login model/store/session 类型 |
| `internal/service/channels/adapters/dingtalk.go` | 拆为 `dingtalk_delivery.go`、`dingtalk_stream.go`、`dingtalk_callback.go`、`dingtalk_util.go`，入口文件只保留 DingTalkChannel 类型、构造和基础配置 |
| `internal/service/channels/adapters/delivery_test.go` | 将 DingTalk delivery/token/stream 测试移到 `dingtalk_delivery_test.go`，避免跨平台 delivery 测试继续混杂 |
| `internal/app/server/routes_web.go` | 补齐 Go 静态托管缓存策略：`/assets/*` 使用 immutable cache，HTML fallback 使用 no-cache |
| `web/vite.config.ts` / `deploy/nginx.conf` | 将 Vite 构建配置内局部 helper/变量改为 TS 常规 camelCase；nginx 哈希静态资源缓存调整为 1 年并关闭静态资源 access log |
| `internal/service/auth/service.go` | 拆为 `service_session.go`、`service_user.go`、`service_principal.go`、`service_desktop.go`、`service_cookie.go`、`service_token.go`、`service_validate.go`，入口文件只保留 Service 核心和状态入口 |
| `internal/storage/auth/repository.go` | 拆为 `repository_state.go`、`repository_user.go`、`repository_password.go`、`repository_session.go`、`repository_value.go`，入口文件只保留 Repository 核心和 SQL bind helper |
| `internal/service/imagegen/service.go` | 拆为 `service_config.go`、`service_provider_openai.go`、`service_provider_options.go`、`service_http.go`、`service_image_extract.go`、`service_input.go`、`service_output.go`，入口文件只保留 Service 核心和 generate/edit 主流程 |
| `internal/service/imagegen/service_test.go` | 拆为 `service_config_test.go`、`service_provider_external_test.go`、`service_test_helpers_test.go` 和保留 OpenAI/Azure 主路径的 `service_test.go` |
| `internal/storage/provider/repository.go` | 拆为 `repository_provider.go`、`repository_model.go`、`repository_usage.go`、`repository_scan.go`、`repository_dialect.go`，入口文件只保留 Repository 类型、常量和构造 |
| `internal/service/skills/marketplace_search.go` | 拆为 `marketplace_search_sources.go`、`marketplace_search_rows.go`、`marketplace_search_filter.go`，入口文件只保留搜索聚合和 source dispatch |
| `internal/service/skills/marketplace_external_test.go` | 拆出 `marketplace_external_search_test.go`，按外部搜索/source registry 与 import/git/preview 测试隔离 |
| `internal/service/skills/service_test.go` | 拆出 `service_registry_test.go`、`frontmatter_test.go`，按主服务流程、registry 迁移/隔离、frontmatter/metadata helper 隔离；未为单个小断言保留独立文件 |
| `internal/cli/command_skill.go` | 拆为 `command_skill_external.go`、`command_skill_agent.go`，入口文件只保留 skill 命令树 |
| `internal/runtime/debug_message.go` | 拆为 `debug_message_fields.go`、`debug_message_summary.go`、`debug_message_value.go`，入口文件只保留公开选项和字段构造 facade |
| `internal/message/processor_test.go` | 拆为 `processor_assistant_stream_test.go`、`processor_tool_result_test.go`、`processor_error_permission_test.go`，按 assistant/stream 状态、tool result/artifact/alias 和 error/permission 语义隔离 |
| `internal/cli/command_memory.go` | 拆为 `command_memory_query.go`、`command_memory_entry.go`、`command_memory_scope.go`，入口文件只保留 memory 命令树和 service/engine 工厂 |
| `internal/cli/command_automation.go` | 拆出 `command_automation_heartbeat.go`，按 scheduled task 命令和 heartbeat 命令隔离 |
| `internal/cli/command_room.go` | 拆出 `command_room_member.go`、`command_room_conversation.go`，按 Room 主 CRUD、成员命令和话题命令隔离 |
| `internal/workspace/memory/repository.go` / `internal/workspace/memory/repository_engine.go` | 拆为 search、file slice、entry mutation、engine entry CRUD、stable context、session summary、checkpoint、cleanup、sort helper 等职责文件，入口文件只保留核心类型 |
| `internal/infra/logx/handler_pretty.go` | 拆为 `handler_pretty_render.go`、`handler_pretty_extract.go`、`handler_pretty_color.go`、`handler_pretty_value.go`、`handler_pretty_model.go`，入口文件只保留 slog handler state 和接口实现 |
| `internal/service/channels/adapters/telegram.go` | 拆为 `telegram_delivery.go`、`telegram_polling.go`、`telegram_ingress.go`、`telegram_model.go`、`telegram_util.go`，入口文件只保留 channel 生命周期和基础配置 |
| `internal/service/channels/adapters/feishu_test.go` | 拆出 `feishu_callback_test.go`，按 callback 解码/加密/签名安全和 channel lifecycle/delivery/SDK ingress 隔离 |
| `internal/service/channels/adapters/ingress_envelope_test.go` | 拆为 `discord_ingress_envelope_test.go`、`telegram_ingress_envelope_test.go`、`dingtalk_ingress_envelope_test.go`，按平台隔离 ingress envelope 测试 |
| `internal/runtime/executor_round.go` | 拆为 `executor_round_model.go`、`executor_round_stream_diagnostics.go`、`executor_round_stream_error.go`、`executor_round_terminal.go`、`executor_round_util.go`，入口文件只保留 ExecuteRound 主执行循环 |
| `internal/runtime/executor_round_test.go` | 拆为基础请求/持久化、internal context、terminal completion、interrupt/diagnostics 四个测试文件 |
| `internal/workspace/memory/engine.go` | 拆为 `engine_query.go`、`engine_mutation.go`、`engine_capture.go`、`engine_item.go`、`engine_scope_score.go`、`engine_render.go`、`engine_message.go`、`engine_util.go`，入口文件只保留 Engine 构造、BeforeRecall 和 CommitTurn 主流程 |
| `internal/service/room/service.go` | 拆为 `service_query.go`、`service_room_crud.go`、`service_member.go`、`service_conversation_crud.go`、`service_cleanup.go`、`service_agent_resolution.go`、`service_host.go`，入口文件只保留 Service 核心、Repository 接口和 setter |
| `internal/service/room/goal_runtime_test.go` | 拆为 `goal_runtime_usage_test.go`、`goal_runtime_context_test.go`、`goal_runtime_continuation_test.go`、`goal_runtime_helpers_test.go`，按 usage accounting、slot runtime context、continuation/collaboration 和共享夹具隔离 |
| `internal/service/room/service_test.go` | 拆为 `service_lifecycle_test.go`、`service_settings_direct_test.go`、`service_cleanup_test.go`、`service_test_helpers_test.go`，按 lifecycle/runtime、skills/host/direct room、artifact cleanup 和共享夹具隔离 |
| `internal/storage/sqlite/repository_room.go` / `internal/storage/postgres/repository_room.go` | 分别保留 dialect SQL 主体、delete/load 分片；过薄的 util helper 已回收到主文件，重复 scanner、aggregate 选择和 room entity ID helper 已收敛到 `internal/storage/roomrepo` |
| `internal/protocol/model_automation.go` | 拆为 `model_automation_schedule.go`、`model_automation_target.go`、`model_automation_job.go`、`model_automation_report.go`、`model_automation_input.go`、`model_automation_heartbeat.go`，主文件只保留 automation 常量和错误 |
| `internal/service/session/service.go` | 拆为 `service_query.go`、`service_mutation.go`、`service_history.go`、`service_workspace.go`、`service_runtime.go`、`service_util.go`、`service_model.go`，入口文件只保留 Service 核心和构造 |
| `internal/service/session/service_test.go` | 拆为 `service_lifecycle_listing_test.go`、`service_history_test.go`、`service_test_helpers_test.go`，按 lifecycle/listing/title、history/meta 和共享夹具隔离 |
| `internal/service/conversation/titlegen/service.go` | 拆为 `contract.go`、`request.go`、`preview.go`、`generation.go`、`apply.go`、`title_rules.go`，入口文件只保留 Service 构造、logger 和调度入口；Request 类型和方法保持同文件 |
| `internal/service/conversation/titlegen/service_test.go` | 拆为 `service_test.go`、`generation_test.go`、`preview_test.go`、`test_helpers_test.go`，按调度、生成/重试/配置、Goal 预填充和共享夹具隔离 |
| `internal/service/dm/service_test.go` | 拆为 `service_round_runner_test.go`、`service_goal_runtime_test.go`、`service_chat_test.go`、`service_runtime_session_test.go`、`service_interrupt_test.go`、`service_input_queue_test.go`、`service_test_helpers_test.go`，按 round runner/external reply、goal runtime、chat、runtime session、interrupt、input queue 和共享夹具隔离 |
| `internal/service/dm/service_chat_test.go` | 拆出 `service_chat_runtime_options_test.go`，按聊天消息/streaming 持久化与 runtime provider/model/permission 配置隔离 |
| `internal/service/dm/service_runtime_session_test.go` | 拆出 `service_runtime_resume_test.go`，按 runtime prompt/基础 session 持久化与 stale resume/runtime fingerprint 复用决策隔离 |
| `internal/service/dm/service_goal_runtime_test.go` | 拆出 `service_goal_continuation_test.go`，按 RoundRunner goal usage accounting 与 Service hidden continuation/client context 隔离 |
| `internal/service/dm/service_interrupt_queue_test.go` | 改为 `service_interrupt_test.go` 并拆出 `service_input_queue_test.go`，按 interrupt/interrupt delivery policy 与 queue/guide input 隔离 |
| `internal/service/dm/service_test_helpers_test.go` | 拆为 `service_runtime_test_helpers_test.go`、`service_goal_test_helpers_test.go`、`service_event_test_helpers_test.go` 和环境/DB/transcript 夹具文件，避免共享 helper 继续变成单文件垃圾桶 |
| `internal/service/room/service_realtime_test.go` | 拆为 `service_realtime_chat_delivery_test.go`、`service_realtime_chat_runtime_test.go`、`service_realtime_mentions_test.go`、`service_realtime_queue_test.go`、`service_realtime_interrupt_test.go`、`service_realtime_session_test.go`、`service_realtime_helpers_test.go`，按投递/ack、runtime/permission、public mention、user queue/guidance、interrupt、SDK session 和共享夹具隔离 |
| `internal/service/room/service_realtime_chat_runtime_test.go` | 拆出 `service_realtime_runtime_policy_test.go`，按 runtime message/stream/history 与 provider/model/permission/Goal defer policy 隔离 |
| `internal/service/room/service_realtime_mentions_queue_test.go` | 改为 `service_realtime_mentions_test.go` 并拆出 `service_realtime_queue_test.go`，按 public mention 唤醒/接力/忙时排队与普通 Room user queue/guidance 隔离 |
| `internal/service/room/chat_target_test.go` / `internal/service/room/public_mentions_test.go` | 合并为 `chat_routing_test.go`，把过小的内部消息路由/目标解析测试收回到一个稳定职责文件 |
| `internal/service/room/runtime_env.go` / `internal/service/room/runtime_imagegen_default.go` / `internal/service/room/service_runtime_selection.go` | 合并为 `execution_runtime_options.go`，把只服务 SDK options 组装的小文件收敛到一个执行期 runtime 配置文件 |
| `internal/service/dm/runtime_imagegen_default.go` / `internal/service/automation/runtime_imagegen_default.go` | 删除独立小文件，分别收回到 runtime client options 与 automation service 核心 provider 配置处 |
| `internal/service/channels/adapter_inbound.go` / `internal/service/channels/channel_catalog.go` / `internal/service/channels/adapter_logging.go` | 复查后收回到 `model_channel.go` / `model_control.go` / `router.go`，避免 channels 包内只有类型别名、转发函数或单用 helper 的薄门面文件 |
| `internal/service/goal/prompt_template.go` | 复查后收回到 `service_helpers.go`，避免 goal 包内只有一个私有模板替换 helper 的薄文件 |
| `internal/service/automation/service_owner_context.go` / `internal/service/automation/service_task_delete.go` | 复查后收回到 `service_util.go` / `service_task_crud.go`，避免 automation 包内只有私有 owner-context helper 或 delete 辅助逻辑的薄文件 |
| `internal/service/automation/service_execution.go` | 拆为 `service_execution_dispatch.go`、`service_execution_observe.go`、`service_execution_overlap.go`、`service_execution_prompt.go`，入口文件只保留执行启动主流程 |
| `internal/service/automation/service_execution_test.go` | 拆为 `service_execution_run_test.go`、`service_execution_target_test.go`、`service_execution_access_test.go`，按 run ledger/overlap、room/main/script target 和权限/owner scope 隔离 |
| `internal/service/automation/service_observability.go` | 拆为 `service_observability_daily_report.go`、`service_observability_health.go`、`service_observability_util.go`，入口文件只保留任务状态入口 |
| `internal/service/automation/service_observability_test.go` | 拆出 `service_observability_daily_report_test.go`，按 task events/status 和 daily report 聚合隔离 |
| `internal/service/automation/service_recovery_test.go` | 拆为 `service_runtime_claim_test.go`、`service_recovery_runtime_test.go`、`service_recovery_cleanup_test.go`，按 runtime claim/state、bootstrap/manual/watchdog recovery 和 disable/delete cleanup 隔离 |
| `internal/runtime/mcp/automation/server_management_test.go` | 拆为 `server_disable_test.go`、`server_enable_delete_test.go`、`server_list_search_test.go`，按 disable、enable/delete、list/search MCP 工具域隔离 |
| `internal/runtime/mcp/automation/server_create_test.go` | 拆出 `server_create_delivery_test.go`，让 create 基础/默认/日程语义和 reply/delivery target 解析测试分开 |
| `internal/runtime/mcp/automation/server_observability_test.go` | 拆为 `server_daily_report_test.go`、`server_events_status_test.go`、`server_runs_test.go`，按 MCP 工具域保留 3 个测试文件；run-now、run history、recover/retry delivery 合并到一个 run 生命周期文件，避免 50/116 行小碎片 |
| `internal/service/room/input_queue.go` | 拆为 `input_queue_dispatch.go`、`input_queue_context.go`、`input_queue_store.go`、`input_queue_broadcast.go`、`input_queue_util.go`，入口文件只保留队列控制和 guide 入口 |
| `internal/service/room/execution.go` | 拆为 `execution_runtime_session.go`、`execution_runtime_diagnostics.go`、`execution_runtime_tools.go`，入口文件只保留 round / slot 主执行流程 |
| `internal/service/automation/delivery.go` / `internal/service/automation/service_execution_prompt.go` / `internal/service/room/service_context.go` / `internal/service/channels/channel_util.go` | 回收 4 个过薄 helper 文件，分别并入 delivery runtime、execution dispatch、service query 和 channel value helper，避免按函数摊文件 |
| `internal/handler/websocket/handler.go` | 拆为 `handler_connection.go`、`handler_dispatch.go`、`handler_room_subscription.go`、`handler_session_binding.go`、`handler_control.go`、`handler_error.go`、`handler_values.go`、`registry_app_event_subscription.go`，入口文件只保留构造、连接入口和 room/app 事件广播 API |
| `internal/service/room/chat.go` | 拆为 `chat_target.go`、`chat_title.go`、`chat_agent_directory.go`、`chat_persistence.go`，入口文件只保留 Room chat 主编排流程 |
| `internal/service/room/round_state.go` | 拆为 `round_registry.go`、`round_mapper.go`、`slot_state.go` 和 `slot_input.go`，入口文件只保留 active round / slot 类型；过碎的 slot interrupt helper 已合回 `slot_state.go` |
| `internal/chat/room/visible_context.go` | 拆为 `visible_prompt.go`、`visible_batch.go`、`visible_directed.go`、`visible_format.go`、`visible_history.go`、`visible_value.go`，入口文件只保留动态上下文编排 |
| `web/src/hooks/agent/use-agent-conversation.ts` | 拆出 `conversation-volatile-snapshot.ts`、`conversation-runtime-state.ts`、`conversation-history.ts`、`use-conversation-stream-buffer.ts`，并把 resync / agent runtime WebSocket 特例收进 `websocket-event-handler.ts`，让主 hook 保持 facade 和状态协调 |
| `web/src/features/conversation/shared/editor/editor-panel.tsx` | 拆出 `workspace-file-preview-kind.ts`、`html-file-preview.tsx`、`media-file-preview.tsx` 和 `office-preview-fallbacks.tsx`，把文件类型判断、HTML sandbox 预览、PDF/Image/Binary 预览和 Office lazy fallback 从 panel 主体迁出 |
| `web/src/features/conversation/shared/composer-panel.tsx` | 拆出 `composer-local-attachment-model.ts`、`composer-local-attachments.tsx`、`composer-pending-queue.tsx`、`use-composer-attachments.ts` 和 `use-composer-mention.ts`，把本地附件模型/粘贴图片/大段文本粘贴转附件、附件条 UI、pending queue 拖拽排序和 @mention 状态从主 composer 迁出 |
| `web/src/shared/ui/select-menu.tsx` | 拆出 `select-menu-layer.ts` 和 `multi-select-menu.tsx`，把 portal 定位/外部关闭/重定位从组件渲染迁出，并按单选/多选两个公开组件隔离；原 `select-menu.tsx` 保留兼容导出 |
| `web/src/features/settings/provider-settings-panel.tsx` | 拆出 `provider-settings-model.ts`、`provider-settings-capability-switch.tsx`、`provider-settings-model-options-dialog.tsx`、`provider-settings-add-model-dialog.tsx`、`provider-settings-delete-usage-dialog.tsx`、`provider-settings-sidebar.tsx`、`provider-settings-model-list.tsx`、`provider-settings-config-form.tsx` 和 `use-provider-model-actions.ts`，把 provider draft/preset/model payload helper、能力开关、模型选项/新增模型/删除使用中弹窗、provider/preset 侧栏、模型列表、provider 配置字段和 model fetch/test/add/toggle/options 操作从 panel 主体迁出 |
| `web/src/features/settings/settings-panel.tsx` | 拆出 `settings-preferences-model.ts`、`settings-options.ts`、`settings-panel-ui.tsx`、`settings-default-model-row.tsx`、`settings-system-section.tsx`、`settings-appearance-section.tsx`、`settings-desktop-section.tsx`、`settings-permissions-section.tsx` 和 `settings-onboarding-row.tsx`，把偏好归一化、设置选项、通用 row primitive、默认模型行、系统版本区、外观/语言区、桌面区、权限区和 onboarding reset 行从 panel 主体迁出；system version 与 desktop bridge 状态已下放到对应 section 自己 |
| `web/src/features/conversation/shared/editor/presentation-file-preview.tsx` | 拆出 `presentation-preview-model.ts`、`presentation-slide-canvas.tsx` 和 `presentation-xml-utils.ts`，把 PPTX preview 类型/常量、SVG slide 渲染、XML/relationship/path helper 从预览加载/解析主流程迁出 |
| `desktop/macos/Sources/NexusDesktop/Update/DesktopUpdateChecker.swift` / `desktop/windows/Nexus.Desktop/Update/DesktopUpdateChecker.cs` | 拆出 `DesktopUpdateModels`，把版本比较、Release/asset DTO、状态/result/download model 和错误类型从更新流程主类迁出；主类仍保留 fetch/download/install/UI 编排，后续只在职责继续混杂时再拆 |
| `web/src/features/capability/channels/channels-directory.tsx` | 拆出 `channel-model.ts`、`channel-ui-model.tsx`、`channel-guide.tsx`、`channel-login-panel.tsx`、`channel-accounts-panel.tsx` 和 `channel-card.tsx`，把 channel model helper、图标样式、接入指南、扫码登录、账号列表和频道卡片从 directory/container 主体迁出 |
| `web/src/features/home/home-sidebar-panel.tsx` | 拆出 `home-sidebar-directory.ts`、`home-sidebar-conversation-model.ts` 和 `home-sidebar-list-rows.tsx`，把目录加载/WebSocket 订阅、conversation/unread 建模和 chat/contact row UI 从 panel 主体迁出 |
| `web/src/hooks/room-page-controller/use-room-page-controller.ts` | 拆出 `use-room-external-sessions.ts`，把 DM 外部 IM 会话加载、directory update 订阅、focus/visibility/interval 刷新、会话差异比较和 Room conversation view 映射从页面 controller 迁出；conversation snapshot 到 room context 的纯数据写回已下放到 `room-page-controller-core.ts` |
| `web/src/features/conversation/shared/message/item/use-message-item-state.ts` | 拆出 `message-item-stats.ts`、`message-item-permissions.ts`、`message-item-ordering.ts`、`message-item-activity.ts` 和 `message-item-final-projection.ts`，把 stats/result、permission 匹配、assistant entry/turn 构建、process/live activity 摘要和最终助手内容投影从 hook 主体迁出 |
| `web/src/features/conversation/room/dm/dm-chat-panel.tsx` / `web/src/features/conversation/room/group/chat/group-chat-panel.tsx` | 拆出 `use-conversation-snapshot-reporter.ts` 和 `use-conversation-history-loader.ts`，把 DM/Group 共用的快照去重上报、活跃时间基线和历史消息触顶/不足一屏补拉从页面面板迁出 |
| `web/src/features/agents/private-domain/agent-private-domain-view.tsx` | 拆出 `agent-private-domain-toolbar.tsx`、`agent-private-domain-thread-list.tsx`、`agent-private-domain-timeline.tsx`、`agent-private-domain-avatar.tsx` 和 `agent-private-domain-model.ts`，把私域联络入口、列表、时间线、头像栈和展示文案 helper 分离 |
| `web/src/features/launcher/launcher-agent-pile.tsx` | 拆出 `launcher-agent-pile-model.ts`，把 token 物理配置、颜色转换和品牌样式计算从 Matter 生命周期/DOM 渲染主体迁出 |
| `web/src/shared/ui/sidebar/sidebar-wide-panel.tsx` | 拆出 `use-sidebar-guide-center.ts`，把引导中心/tour 注册、自动启动、跨页面 tour 跳转和 guide center props 从 sidebar 布局主体迁出 |
| `web/src/features/conversation/shared/message/item/message-item.tsx` | 拆出 `message-user-section.tsx` 和 `message-assistant-section.tsx`；2026-06-22 反向复查时移除 2 行 re-export 文件，调用方直接导入真实组件，避免多一层跳转 |
| `web/src/shared/i18n/messages.ts` | 拆出 `messages.zh.ts` / `messages.en.ts` 两个完整语言表；原文件保留 Locale、TranslationKey re-export 和 MESSAGES 聚合。刻意不按 domain 继续拆，避免文案 key 跨文件查找 |

## 四块功能审计

### 1. Goal

整体评价：结构相对健康。目录已经按 service、runtime policy、continuation、objective、progress、tool、state machine、storage、MCP tool 分散，不需要大搬家。

| 文件 | 行数 | 结论 | 建议 |
| --- | ---: | --- | --- |
| `internal/service/goal/service_progress.go` | 474 | 关注 | 仍然集中处理 continuation progress、completion tool miss、room collaboration 和 metadata reset。建议后续拆为 `service_progress_continuation.go`、`service_progress_completion.go`、`service_progress_room.go`、`service_progress_metadata.go` |
| `internal/service/goal/service_continuation.go` | 283 | 合理偏长 | 计划生成、当前性检查、释放计划、prompt 构造在一起。可接受；若继续增长，拆 `service_continuation_prompt.go` |
| `internal/service/goal/service_appserver.go` | 282 | 合理 | appserver 适配边界清楚 |
| `internal/storage/goal/repository.go` | 251 | 合理 | 仓储职责清晰 |
| `internal/service/goal/service.go` | 239 | 合理 | 构造和核心入口可接受 |
| `internal/service/goalobjective/service.go` | 160 | 合理 | objective rewrite 入口清晰 |
| `internal/service/goalobjective/runtime_selection.go` | 160 | 合理 | runtime selection 单独文件是正确方向 |
| `internal/runtime/mcp/goal/tool/result.go` | 215 | 合理 | tool result 解析集中可接受 |
| `internal/runtime/mcp/goal/tool/registry_test.go` | 413 | 测试关注 | 测试长但范围单一，暂不拆 |

Goal 不建议新增多层目录。当前文件名前缀基本符合仓库约定。

### 2. 定时器 / Automation

整体评价：服务层已从明显大文件拆到可维护状态。下一轮重点是测试 helper 和 run 仓储增长控制。

| 文件 | 行数 | 结论 | 建议 |
| --- | ---: | --- | --- |
| `internal/service/automation/service_execution_*.go` | 已拆 | 合理 | 执行启动、session dispatch、观察收尾、overlap skip、cron prompt marker 已拆到同包职责文件；最大生产分片 219 行 |
| `internal/service/automation/service_observability_*.go` | 已拆 | 合理 | task status、daily report、health、observability helper 已拆到同包职责文件；最大生产分片 281 行 |
| `internal/service/automation/service_task_crud.go` | 264 | 合理 | CRUD 边界清楚 |
| `internal/service/automation/service_task_query.go` | 小文件 | 合理 | 查询职责单一 |
| `internal/service/automation/service_task_run.go` | 187 | 合理 | run/retry/recover 边界清楚 |
| `internal/service/automation/service_heartbeat_runtime.go` | 180 | 合理 | runtime state 和 wake request 放在一起可接受 |
| `internal/service/automation/service_heartbeat_dispatch.go` | 小文件 | 合理 | 发送/调度拆分方向正确 |
| `internal/service/automation/service_heartbeat_status.go` | 小文件 | 合理 | 状态视图拆分方向正确 |
| `internal/service/automation/service_delivery_*_test.go` | 已拆 | 合理 | 原 1171 行 delivery 测试已按 observation、success/inbox、retry、dead-letter 和 helpers 拆开；最大分片 399 行 |
| `internal/runtime/mcp/automation/server_*_test.go` | 已拆 | 合理 | 原 1227 行 observability 测试按 daily report、events/status、run 生命周期拆开；run 相关小文件已合并，最大分片 734 行 |
| `internal/runtime/mcp/automation/internal/builder/builder.go` | 409 | 合理 | builder 聚合可接受 |
| `internal/runtime/mcp/automation/internal/semantic/resolver.go` | 328 | 合理 | semantic resolver 职责单一 |
| `internal/storage/automation/repository_run.go` | 426 | 合理偏长 | run 仓储集中可接受，增长后按 job/run/event 拆 |

测试文件多处超过 900 行；主题边界清楚时按场景拆，纯回归矩阵不为行数硬拆。

### 3. IM 通道

整体评价：adapter 拆分已有改善，router/ingress orchestration 也已拆到可维护状态。IM 通道不建议再把不同平台塞进同一个大文件；企业微信、个人微信、飞书、钉钉、Telegram 的适配器应该各自保持清晰边界。

| 文件 | 行数 | 结论 | 建议 |
| --- | ---: | --- | --- |
| `internal/service/channels/router_*.go` | 已拆 | 合理 | 注册、生命周期、route memory、delivery/typing、snapshot 已拆到同包职责文件，最大分片 229 行 |
| `internal/service/channels/ingress_*.go` | 已拆 | 合理 | accept、normalize、session resolve、permission、delivery target 已拆到同包职责文件，最大分片 159 行 |
| `internal/service/channels/adapters/personal_weixin_*.go` | 已拆 | 合理 | Channel、iLink client、payload model、工具函数已拆到同包文件 |
| `internal/service/channels/service_pairing_*.go` | 已拆 | 合理 | CRUD、ingress resolve、SQL store、view/stats 已拆到同包职责文件，最大分片 216 行 |
| `internal/service/channels/service_login_*.go` | 已拆 | 合理 | login model、API flow、个人微信执行、login store、session state 已拆到同包职责文件，最大分片 172 行 |
| `internal/service/channels/adapters/dingtalk_*.go` | 已拆 | 合理 | core、delivery/token、stream callback、HTTP callback decode、util 已拆到同包职责文件，最大分片 227 行 |
| `internal/service/channels/adapters/telegram_*.go` | 已拆 | 合理 | channel core、delivery/typing、polling、ingress、model、util 已拆到同包职责文件；最大生产分片 123 行 |
| `internal/service/channels/adapters/feishu_*.go` | 已拆 | 合理 | 当前拆分方向正确 |
| `internal/service/channels/adapters/wecom_bot_*.go` | 已拆 | 合理 | 当前拆分方向正确 |
| `internal/service/channels/service_control_*_test.go` | 已拆 | 合理 | 原 1395 行 control 测试已按 catalog、config、login、pairing、Feishu ingress 和 helpers 拆开；最大分片 541 行 |
| `internal/service/channels/transport/http.go` / `text.go` | 小文件 | 合理 | 共享传输 helper 放这里合适 |

目录层级建议：维持 `internal/service/channels/adapters`，暂不为每个平台建子包。Go 里子包会带来未导出符号访问成本，目前同包文件拆分更稳。

### 4. 连接器

整体评价：service 层已从 catch-all 改成清晰分文件；Feishu Docx client 和 MCP tool 注册也已完成第一轮拆分，剩余关注点在 markdown render。

| 文件 | 行数 | 结论 | 建议 |
| --- | ---: | --- | --- |
| `internal/service/connectors/service_connection.go` | 413 | 合理偏长 | 连接列表、加载、refresh、connect/disconnect、SQL upsert 都在一个文件。短期可接受；后续可拆 `service_connection_refresh.go`、`service_connection_store.go` |
| `internal/service/connectors/service_oauth_flow.go` | 286 | 合理 | OAuth URL/callback/state 在一起可接受 |
| `internal/service/connectors/service_device_flow.go` | 153 | 合理 | device flow 单独文件清楚 |
| `internal/service/connectors/catalog.go` | 367 | 合理 | catalog 数据集中可接受 |
| `internal/service/connectors/service_*_test.go` | 已拆 | 合理 | 原 1290 行测试已按 catalog/state、OAuth config、device flow、connection credentials、OAuth callback 和 helpers 拆开；最大分片 432 行 |
| `internal/connectors/feishudocx/client_*.go` | 已拆 | 合理 | document API、drive list、SDK transport、block codec 已拆分；`client.go` 只保留构造和 document target resolve |
| `internal/connectors/feishudocx/render_*.go` | 已拆 | 合理 | 原 534 行 `render_markdown.go` 已拆为 document target、renderer core、block accessor、inline style、table 和 markdown util；最大生产分片 170 行 |
| `internal/runtime/mcp/connectors/tool/feishu_docx_*.go` | 已拆 | 合理 | 13 个 MCP tool 已按 document/search/write、sheet、bitable、drive/wiki、client/helper 分文件 |
| `internal/storage/connectors/store_oauth_client.go` | 158 | 合理 | store 文件清楚 |

不建议把 connectors service 再套多层目录。当前同 package 文件拆分已经符合 Go 习惯。

## 全仓高风险生产文件

这些文件不属于四块功能全部范围，但结构风险很高，应该纳入后续治理。

| 文件 | 行数 | 风险 | 建议 |
| --- | ---: | --- | --- |
| `web/src/hooks/agent/use-agent-conversation.ts` | 1164 | 中 | volatile storage、pending permission、runtime snapshot helper、历史分页加载、流式消息 rAF 合批和 WebSocket resync 特例已拆；2026-06-22 复查后不再按行数继续拆，剩余主要是 hook facade、WebSocket 绑定、ack 超时和运行态协调，继续抽 hook 会制造大 context 传递 |
| `web/src/features/settings/provider-settings-panel.tsx` | 682 | 中 | provider draft/model/options/preset helpers、能力开关、模型相关弹窗、provider/preset 侧栏、模型列表、配置字段和 model 操作 hook 已拆；2026-06-22 复查后暂不继续拆，剩余主要是 provider 配置保存/删除与页面装配，硬拆成大 hook 会增加 props 跳转 |
| `internal/storage/workspace/store_agent_history_*.go` | 已拆 | 合理 | 原 1642 行文件已拆为 facade、overlay、transcript path/cache/reader/project/guidance/marker 等文件；最大生产分片 256 行 |
| `internal/storage/workspace/store_input_queue_*.go` | 已拆 | 合理 | 原 634 行文件已拆为入口、file、replay、codec、order；最大生产分片 285 行 |
| `internal/storage/workspace/history_*.go` | 已拆 | 合理 | 原 735 行 `history_normalize.go` 已拆为 normalize facade、pagination、result summary、external delivery、unfinished round、order；最大生产分片 203 行 |
| `internal/storage/workspace/store_session_file.go` | 已拆 | 合理 | 原 567 行文件已拆为 session meta CRUD、JSONL 读写、history compact、value coercion；最大生产分片 254 行 |
| `internal/service/workspace/manager_live_*.go` | 已拆 | 合理 | 原 679 行文件已拆为订阅生命周期、fsnotify watcher、API/agent write flush、listener/path util、diff stats；最大生产分片 238 行 |
| `internal/service/workspace/service_*.go` | 已拆 | 合理 | 原 640 行 `service.go` 已拆为 model、agent workspace、file read/list/download、mutation、upload、path helper；最大生产分片 164 行 |
| `internal/service/workspace/initializer_*.go` | 已拆 | 合理 | 原 627 行 `initializer_workspace.go` 已拆为 workspace 编排、template、skill、nexusctl shim、util；最大生产分片 203 行 |
| `web/src/shared/i18n/messages.ts` | 已拆 | 合理 | 原 1580 行文案表已拆为 zh/en 两个完整语言表和统一 facade；不继续按 domain 拆，避免文案维护过散 |
| `web/src/shared/ui/select-menu.tsx` | 已拆 | 合理 | 原 574 行文件已拆成单选入口、multi-select 实现、layer hook 和 model helper；单选入口保留兼容导出 |
| `web/src/shared/ui/sidebar/sidebar-wide-panel.tsx` | 521 | 中 | guide center / tour orchestration 已拆；剩余是 Nexus 入口、一级 tab、collapsed/expanded layout 和 resize，重复渲染可读性优先 |
| `web/src/features/conversation/shared/editor/editor-panel.tsx` | 517 | 中 | 文件类型判断、HTML sandbox、PDF/Image/Binary preview 和 Office fallback 已拆；剩余是预览容器和文本编辑加载/保存，暂不拆 |
| `web/src/features/conversation/shared/editor/spreadsheet-preview-model.ts` | 512 | 中 | 纯 Excel workbook 到预览模型的转换工具，长但职责单一，暂不拆 |
| `web/src/features/conversation/room/members/create-room-dialog.tsx` | 518 | 合理偏长 | 2026-06-22 复查后暂不拆；当前是一个自包含创建/管理 Room 弹窗，房间信息、成员选择和 skill 选择共享同一组表单状态，拆子组件会引入大量 props |
| `internal/service/provider/service_*.go` | 已拆 | 合理 | 原 `service.go` 已拆到多个职责文件，入口文件只保留构造、依赖和 core 类型 |
| `internal/storage/sqlite/repository_room_*.go` / `internal/storage/postgres/repository_room_*.go` | 已拆 | 合理偏长 | 2026-06-22 复查后暂不继续拆；主文件为 740 / 741 行，保留各 dialect 的写路径和事务，delete/load 是明确职责分片，过薄的 util helper 已合回主文件；scan/aggregate/ID helper 保持在 `roomrepo`，不强行合并 dialect |
| `web/src/features/settings/settings-panel.tsx` | 86 | 合理 | 入口已收敛为 settings scaffold，不再列为结构风险 |
| `web/src/features/conversation/shared/editor/presentation-file-preview.tsx` | 254 | 合理 | PPTX 类型/常量、slide canvas 和 XML helper 已拆；主文件只保留预览加载/解析入口 |
| `internal/service/provider/service_model_*.go` | 已拆 | 合理 | 原 `service_models.go` 已按模型变更、发现、连通性检查、HTTP、模型卡解析、codec/redaction 分文件 |
| `web/src/features/conversation/shared/composer-panel.tsx` | 673 | 中 | 本地附件、pending queue、@mention 和 footer/action menu 已拆；2026-06-22 复查后暂不继续拆，剩余主要是发送流程、IME/input history、goal/loop 模式和输入区渲染，继续拆会把单一输入状态机打散 |
| `web/src/features/capability/channels/channels-directory.tsx` | 232 | 合理 | guide、login panel、accounts panel、card 和 model helper 已拆；入口只剩 directory 编排 |
| `web/src/features/conversation/shared/message/item/use-message-item-state.ts` | 460 | 已拆 | stats/result、permission 匹配、assistant entry/turn 构建、process/live activity 摘要和 final projection 已拆；hook 现在主要负责组合 hooks 和复制/停止事件 |
| `desktop/macos/Sources/NexusDesktop/Update/DesktopUpdateChecker.swift` | 914 | 中 | 已拆出 update models；2026-06-22 复查后暂不继续拆。主类集中 release fetch、download/install、UI prompt、state persistence 和 trust check，底部 private extension 已隔离 asset 查找、release notes、SHA256、process runner 和 path helper；拆成 `DesktopUpdateUtils` 只会制造杂项文件 |
| `desktop/windows/Nexus.Desktop/Update/DesktopUpdateChecker.cs` | 793 | 中 | 已拆出 update models；2026-06-22 复查后暂不继续拆。主类集中 release fetch、download/install、UI prompt、state persistence 和启动安装器流程，底部静态 helper 已隔离 asset 查找、release notes、SHA256 和 path helper；拆成 `DesktopUpdateUtils` 只会制造杂项文件 |
| `desktop/windows/Nexus.Desktop/WebView/WebViewHost.cs` | 710 | 中 | 2026-06-22 复查后暂不继续拆。Windows host 与 mac host 边界一致，主体集中 WebView2 初始化、navigation policy、resume probe、desktop bridge lifecycle、cookie 注入和诊断 metadata；拆成 probe/router/helper 会变成只服务一个 host 的薄文件 |
| `desktop/macos/Sources/NexusDesktop/WebView/WebViewHost.swift` | 577 | 中 | 2026-06-22 复查后暂不继续拆。`WebViewConfigurationFactory` 已隔离配置创建；host 主体集中 navigation policy、popup/open panel、resume probe、cookie 注入、bridge probe 和诊断 metadata，拆 route/probe helper 会增加只服务一个 host 的小文件 |
| `web/src/features/conversation/room/group/chat/group-chat-panel.tsx` | 500 | 中 | 2026-06-22 复查后暂不继续拆。快照上报、历史加载、composer handler、thread panel data 和 feed 已拆，入口保留 room conversation 页面编排、goal 创建和 composer/scroll/error/provider warning 装配 |
| `web/src/features/home/home-sidebar-panel.tsx` | 418 | 已拆 | directory hook、conversation/unread model、chat/contact row UI 已拆；入口只保留 Chat/Contacts 容器、创建/删除和导航编排 |
| `internal/runtime/manager_*.go` | 已拆 | 合理 | 原 994 行 manager 已拆为 core、client adapter、session、round、goal accounting、interrupt、streaming input、MCP restart check；最大生产分片 303 行 |
| `internal/workspace/memory/engine_*.go` / `repository_*.go` | 已拆 | 合理 | engine 和 repository 均已按 query、mutation、capture、scope/score、search、file slice、entry CRUD、session summary、checkpoint、cleanup 等职责拆分；最大生产分片 188 行 |
| `web/src/pages/landing/landing-page.tsx` | 242 | 合理 | 落地页已降到普通页面规模，不再列为结构风险 |
| `internal/handler/websocket/handler_*.go` | 已拆 | 合理 | 原 646 行 handler 已拆为 connection、dispatch、room subscription、session binding、control、error、values；入口文件降到 169 行 |
| `internal/service/room/input_queue_*.go` / `execution_*.go` | 已拆 | 合理 | input queue 已拆成入口、dispatch、context、store、broadcast、util；execution 已拆出 runtime session、diagnostics、tool policy；入口主流程分别降到 152 / 478 行 |
| `internal/service/room/chat_*.go` / `round_*.go` / `slot_*.go` | 已拆 | 合理 | chat 主流程、target/title/directory/persistence、round registry/mapper、slot state/input 已拆分；`chat.go` / `round_state.go` 降到 337 / 98 行 |
| `internal/chat/room/visible_*.go` | 已拆 | 合理 | 原 614 行 `visible_context.go` 已拆为 prompt、batch、directed、format、history、value helper；最大生产分片 194 行 |
| `internal/service/room/service_*.go` | 已拆 | 合理 | 原 822 行 service 已拆为 query、room CRUD、member、conversation、cleanup、agent resolution、host settings；最大新分片 216 行 |
| `internal/service/auth/service_*.go` | 已拆 | 合理 | login/session、user mutation、request principal、desktop local、cookie、token、validate 已拆到同包职责文件，最大分片 272 行 |
| `internal/storage/auth/repository_*.go` | 已拆 | 合理 | 原 604 行仓储已拆为 state、user、password credential、session、value helper；最大生产分片 210 行 |
| `internal/service/imagegen/service_*.go` | 已拆 | 合理 | 原 792 行入口已拆为 config、provider call/options、HTTP retry、image extract、input normalize、output write；最大生产分片 181 行 |
| `internal/storage/provider/repository_*.go` | 已拆 | 合理 | 原 784 行仓储已拆为 provider CRUD/query、model 仓储、runtime usage、scan codec、dialect helper；最大生产分片 247 行 |
| `internal/service/conversation/titlegen/*.go` | 已拆 | 合理 | 原 657 行 `service.go` 已拆为 contract、model、request、preview、generation、apply、title rules；最大生产分片 214 行 |

## 前端结构建议

前端不建议靠更深目录解决所有问题。优先按以下方式拆：

1. `features/*` 下保留业务入口组件。
2. 同 feature 内新增 `components/`、`hooks/`、`model/` 或 `utils/`，不要把所有 helper 放到入口 tsx 顶部。
3. 超大 hook 先拆纯函数 reducer/helpers，再拆副作用 hook。
4. shared UI 文件超过 500 行时，优先抽 positioning hook、state hook、primitive component。
5. `messages.ts` 这类文案表优先按语言保持完整 key 集；不要为了行数再按 domain 打散。

当前没有必须继续机械拆分的前端文件。复查后按以下规则观察：

1. `provider-settings-panel.tsx`、`composer-panel.tsx`、`use-agent-conversation.ts`、`editor-panel.tsx` 和 `sidebar-wide-panel.tsx` 只在出现新的可命名职责边界时再拆。
2. 只 re-export、只转发 props 或只包一层命名的薄文件优先合回调用方或同职责文件。
3. `spreadsheet-preview-model.ts` 这类纯转换器不按行数拆；除非出现第二种输入/输出格式或可复用转换阶段。

450-500 行生产文件复查：

- `room-surface-layout.tsx`：Room surface 布局编排，thread state、chat、workspace、history/about 辅助面板的装配职责集中；只修正局部 TSX 空格，不拆。
- `settings-general-section.tsx`：General settings 容器，appearance/desktop/behavior/permissions/system 子区块和 preferences model 已拆，剩余是加载/保存/默认模型联动。
- `contacts-agent-memory-tab.tsx`：Agent memory tab 自包含，列表和 inspector 只服务本 tab，拆文件会增加跳转。
- `tour-overlay.tsx`：tour popover 定位、sticker 和 portal 渲染是一个 overlay primitive，定位 helper 留同文件更容易读。
- `use-chat-completion-notifications.ts`：聊天完成通知状态机，目录缓存、WebSocket 订阅、浏览器通知和 active target 清理彼此耦合，暂不拆。

## 测试结构复查

`internal/service/dm/*_test.go`、`internal/service/room/service_realtime_*_test.go`、`internal/runtime/mcp/automation/server_*_test.go`、`internal/runtime/*_test.go`、`internal/service/automation/*_test.go`、`internal/service/channels/service_control_pairing_test.go`、`internal/service/provider/service_catalog_runtime_test.go`、`internal/service/agent/service_test.go` 和 `internal/message/processor_assistant_stream_test.go` 已复查。它们虽然超过 500 行，但基本是同一子系统的一组场景用例或共享 fake/helper，不建议按行数拆。

后续只在两种情况拆测试：一是同一 setup 在多个文件重复到影响修改；二是一个测试文件混入两个以上不相关功能域。

## 后端结构建议

优先处理顺序：

1. `internal/service/workspace` / `internal/storage/workspace` 剩余偏大文件复核

## App / Web / Docker 性能审计

当前已确认的健康点：

1. Go HTTP server 已设置 `ReadHeaderTimeout`、`ReadTimeout`、`WriteTimeout`、`IdleTimeout`，基础慢连接保护存在。
2. nginx 已启用 gzip 和 `gzip_vary`；`/assets/` 设置 `Cache-Control: public, max-age=31536000, immutable`，HTML/OAuth fallback 设置 `Cache-Control: no-cache`。
3. Vite 已配置 route lazy、manual chunks 和桌面轻入口的 module preload 过滤。
4. 重型前端依赖大多已延迟加载：`mermaid`、`docx-preview`、`exceljs`、`x-data-spreadsheet`、`jszip`、代码高亮内容等不在普通入口同步 import。
5. Dockerfile 已使用 BuildKit cache mount 缓存 Go module、Go build、pnpm store、apt、pip；pip 安装不再禁用自身缓存。
6. root / web `.dockerignore` 已排除 web 本地构建、coverage、tsbuildinfo、Next/Vercel/Yarn/PnP 等产物，deploy 根上下文不会把这些重新带入镜像构建。

已落地的小优化：

1. Go 同源静态托管补齐缓存 header，`/assets/` 与 nginx 对齐为 1 年 immutable。这样桌面/单进程部署不经过 nginx 时，哈希资源也能被浏览器长期缓存，HTML fallback 不会被错误长期缓存。
2. nginx 补齐 HTML/OAuth fallback 的 `Cache-Control: no-cache`，避免反向代理部署下 SPA 入口页被错误长期缓存。
3. Docker runtime 阶段去掉 pip 的 `--no-cache-dir` 和显式 cache purge，让 BuildKit pip cache mount 在重复构建时真正复用下载缓存，同时不把 cache mount 写进最终镜像。
4. root `.dockerignore` 在 `!web/**` 后重新排除 `web/out`、`web/build`、`web/coverage`、`web/.next`、`web/.vercel`、`web/.cache`、`web/*.tsbuildinfo` 和包管理器调试产物，避免 deploy 构建上下文膨胀。
5. nginx `/assets/` 关闭 access log，并把 Vite 哈希静态资源缓存从 30 天延长到 1 年，减少重复下载和静态请求日志 IO。
6. deploy app 镜像拆出 `runtime-base` stage，并把 `web-builder` 放到 `runtime-base` 之后；默认 `runtime` 仍带 Web dist，`docker-compose` 的 `nexus` 服务使用 `runtime-base` 且不再显式设置 `WEB_DIST_DIR`，避免和 nginx Web 镜像重复构建前端产物。

仍建议后续评估：

1. `pnpm --dir web run build` 已通过，但 Vite 仍提示部分 chunk 超过 500K。当前 `web/dist` 约 27M、606 个文件；最大 chunk 主要是 `exceljs` 约 930K raw / 256K gzip、`cytoscape` 约 434K raw / 138K gzip、mermaid 相关图表 chunk 和 KaTeX 资源。它们多为 lazy chunk，优先级低于首屏主包，但可以继续按使用场景做更细 lazy。
2. KaTeX 同时产出 ttf/woff/woff2 字体，若目标浏览器允许，可评估只保留 woff2 或通过构建插件裁剪字体格式。
3. app runtime 镜像安装 `build-essential`、`python3-dev`、`pkg-config`、`pnpm`、`bun`、`uv` 等工具，适合 agent 能力完整性，但镜像体积和安全面偏大。后续可提供 `full` / `slim` runtime profile，而不是直接删依赖。
4. Go 静态托管目前依赖 `http.FileServer` / `http.ServeFile`，没有 brotli 预压缩资源协商；nginx 部署已有 gzip，单进程部署若追求静态资源性能，可评估构建 `.br`/`.gz` 并在 Go handler 协商。

## 不建议现在做的事

1. 不建议立刻把 Go 同包文件拆成多个子包。这样会制造导出符号和循环依赖问题。
2. 不建议把所有 storage repository 合并成 generic repository。sqlite/postgres 的差异应该通过小 helper 收敛，不要牺牲清晰度。
3. 不建议在结构治理中顺手改业务逻辑。每个拆分 PR/commit 都应该能用 diff 证明行为不变。
4. 不建议把测试文件按行数硬拆。优先抽 helper 和 fixture，保持场景可读。

## 验证门禁

任何拆分都必须至少跑：

```bash
go test ./...
git diff --check
```

四块功能的专项验证：

```bash
go test ./internal/service/goal ./internal/service/goalobjective ./internal/storage/goal ./internal/runtime/mcp/goal/...
go test ./internal/service/automation ./internal/automation ./internal/protocol ./internal/storage/automation
go test ./internal/service/channels/...
go test ./internal/service/connectors ./internal/connectors/... ./internal/storage/connectors ./internal/runtime/mcp/connectors/...
```

涉及前端拆分时再跑：

```bash
npm --prefix web run lint
npm --prefix web run typecheck
```

或者使用仓库总入口：

```bash
make check
```

## 下一步执行建议

建议按低风险同包拆分继续推进：

1. 继续复核 app / web / Docker 性能审计，重点看 bundle 细拆、Docker build profile 和运行时 IO。
2. 集中治理前端超大 TSX/hook，这部分需要更多 UI 回归验证。

每一步都应该保持 API 不变、文件同包迁移、测试先行验证。
