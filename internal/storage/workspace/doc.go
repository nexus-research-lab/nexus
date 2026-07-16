// Package workspace 是 Agent 历史、transcript、输入队列与房间历史的持久化层。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - history_*.go：分阶段历史投影（compact / normalize / order / pagination / last-write round_index /
//     turn projector / rewrite_tail / 按 Agent 执行轮配对的 result_summary / external_delivery / unfinished_round）。
//   - agent_history*.go：Agent 历史门面、读取、overlay 与共享模型。
//   - transcript_*.go：transcript cache、reader、path、session、project、marker、guidance 与 root/source round 投影。
//   - input_queue.go / input_queue_codec.go / input_queue_replay.go：输入队列存取、跨派发持久幂等入队、可返回规范化提交的原子批量登记、预检版本一致的整批 guidance 认领、编解码与事件重放。
//   - room_history.go / room_directed_message.go / room_directed_message_wake.go / session_file.go / jsonl.go：
//     保留 Agent 执行身份的房间历史、定向消息、延迟唤醒、会话文件与 JSONL。
//   - paths.go / transcript_project_hash.go / value_coerce.go：路径、工程 hash、值转换。
//
// 历史投影与持久化共享未导出模型；在形成稳定边界前保留同包，避免为拆目录暴露内部状态。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package workspace
