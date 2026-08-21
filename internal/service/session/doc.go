// Package session 编排文件会话与 Room SQL 会话的统一视图。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / query.go / history.go / external_identity.go：Service、查询、历史消息，以及
//     供目录首屏、Automation 校验和首次投递物化共用的统一 Session 解析、按字段所有权合并 Room SQL/workspace 纯读投影、可取消的历史分页、当前 generation 大内容按需 detail、按当前 Goal 聚合真相刷新完成收据、
//     与消息页共享派生 generation 的可取消 Round Navigator、外部 IM 账号短标识、当前配对、任务引用影响与安全删除事实投影。
//   - mutation.go / runtime_settings.go / model.go / util.go：增删改、目录隐藏 Session option、Session Connector/模型/权限设置提交与后台 runtime 预备通知、模型、辅助。
//   - recovery.go：持久 tombstone 的启动/周期恢复，同时重放任务停用与跨域引用清理。
//   - runtime.go / context_usage.go / subagent_task.go / subagent_tool_run.go / workspace.go：运行时、
//     Session 元数据上下文快照恢复、父会话可见子任务生命周期、独立 transcript 聚合及脱敏 ToolRun 历史投影、workspace。
//   - repository.go：持久化。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package session
