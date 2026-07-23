// Package room 提供 Room 持久化管理与查询能力。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 文件按职责前缀分组：
//   - service.go / crud.go / conversation_crud.go / member.go / query.go：Room 服务装配、房间和 conversation 数据操作。
//   - cleanup.go / runtime.go：持久化资源清理与 runtime session 关闭。
//   - agent_resolution.go / host.go / skills.go：成员、房主设置和 Room skill 归一化。
//   - attachments.go：Room conversation 公共附件上传。
//   - private_domain.go / privateview/：Agent 私域投影查询。
//
// 实时聊天、round、queue、协作消息和 runtime 执行位于同级 realtime 子包；
// 它依赖本包的持久化 Service，本包不反向依赖 realtime，保持单向边界。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package room
