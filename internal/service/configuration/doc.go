// Package configuration 提供 Nexus 配置控制面：统一发现、授权、预检、变更、热生效核对与审计。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - model.go / catalog.go：配置域、资源 scope、业务会话与 runtime lease 身份、能力目录、变更计划、reload 状态与审计协议。
//   - actor.go / access.go：逐次重验 active runtime lease、数据库 owner-main / agent-self / room-host / room-member 身份与字段级能力边界。
//   - service.go / snapshot.go / host_snapshot.go：服务装配、按可信 scope 读取、Skills 全局/各 Agent workspace 来源目录、
//     主机白名单投影与配置健康检查。
//   - skill_change_snapshot.go / connector_change_snapshot.go：Skills target_scope/source_identity、
//     目标 Agent 与 Connector 目标资源的版本、状态和写后结果绑定。
//   - change.go / change_validate.go / change_verify.go / emotion_change.go / session_change.go：有界分片资源锁、plan digest、CAS、幂等、
//     真人批准门槛、可信 DM/Room 情绪上下文、最小化 Session 投影、严格预检与写后证明；
//     owner-main 的 Room 删除绑定 Room version，提交后清理失败进入 reconcile，Room host 不获得删除能力。
//   - change_input.go / change_dispatch.go / preferences_mutation.go：补丁适配、领域分派、Emotion version CAS、Skills 非破坏性启停、
//     Preferences version CAS、锁内 merge 与条件回滚。
//   - audit.go：同时绑定业务 session/root round 与真实 runtime lease、按 owner 与资源 scope 隔离的配置变更审计仓储。
//   - human_approval.go / sanitize.go：绑定认证 session/runtime lease 的一次性批准与带外 secret slot、任意配置树的凭据与内部提示词脱敏。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package configuration
