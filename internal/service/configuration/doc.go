// Package configuration 提供 Nexus 配置控制面：统一发现、授权、预检、变更、热生效核对与审计。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - model.go / catalog.go：配置域、资源 scope、业务会话与 runtime lease 身份、能力目录、变更计划、reload 状态与审计协议。
//   - actor.go / access.go：逐次重验 active runtime lease、数据库 owner-main / agent-self / room-host / room-member 身份与字段级能力边界。
//   - service.go / snapshot.go / host_snapshot.go：服务装配、按可信 scope 读取、Skills 全局/各 Agent workspace 来源目录、
//     主机白名单投影与配置健康检查。
//   - skill_change_snapshot.go / connector_change_snapshot.go：Skills target_scope/source_identity、私有来源安全元数据、owner catalog CAS、
//     目标 Agent 与 Connector 目标资源的版本、状态和写后结果绑定。
//   - change.go / change_validate.go / change_verify.go：有界分片资源锁、plan digest、CAS、幂等、
//     真人批准门槛、严格 JSON 预检、领域路由与统一写后版本证明；
//     owner-main 的 Room 删除绑定 Room version，成员 participation 绑定 Room CAS/authority epoch，提交后清理失败进入 reconcile，Room host 不获得删除能力。
//   - agent_change.go / provider_change.go / room_change.go / channel_change.go / connector_change.go / skill_change.go：
//     按领域聚合输入、校验、执行、读取、通知与专用写后核对；授权、批准、审计仍经过统一控制面。
//   - emotion_change.go / session_change.go / preferences_mutation.go：情绪、Session、Preferences 的同域操作与状态读取；
//     保留可信情绪上下文、最小 Session 投影、Preferences CAS、锁内 merge 与条件回滚。
//   - change_input.go / change_dispatch.go：共用 JSON 补丁合并与领域执行路由，不再平铺所有操作分支。
//   - member_change.go：管理员主智能体私聊的 Control 成员创建、资料/权限修改、撤销与写后核对。
//   - audit.go：同时绑定业务 session/root round 与真实 runtime lease、按 owner 与资源 scope 隔离的配置变更审计仓储。
//   - human_approval.go / sanitize.go：绑定认证 session/runtime lease 的一次性批准与带外 secret slot、私有 Skill Bearer 轮换、
//     任意配置树的凭据与内部提示词脱敏。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package configuration
