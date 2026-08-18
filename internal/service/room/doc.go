// Package room 提供 Room 持久化管理与查询能力。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 文件按职责前缀分组：
//   - service.go / crud.go / conversation_crud.go / conversation_fork.go / member.go / query.go：Room 服务装配、房间、同 Agent DM conversation 分支、不会进入普通目录且可按好友对恢复的联系人内部通道、带 configuration_version CAS 与 authority epoch 的持久成员参与闸门、按 canonical Room/workspace 历史补全的 conversation 消息计数，以及至少保留一条的 conversation 数据操作；每个 Room 最多保留一个显式未开始 draft。
//   - cleanup.go / runtime.go / empty_conversation_prune.go：持久化资源清理、runtime session 关闭，以及带持久引用保护的历史空白 conversation 维护。
//   - agent_resolution.go / host.go / skills.go：成员、房主设置和 Room skill 归一化。
//   - attachments.go：Room conversation 公共附件上传。
//   - private_domain.go / privateview/：Agent 私域投影与稳定事件游标分页查询。
//
// 实时聊天、round、queue、协作消息和 runtime 执行位于同级 realtime 子包；
// 它依赖本包的持久化 Service，本包不反向依赖 realtime，保持单向边界。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package room
