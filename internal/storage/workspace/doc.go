// Package workspace 是 Agent 历史、transcript、输入队列与房间历史的持久化层。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - history_*.go：分阶段历史投影（compact / normalize / order / pagination / round_index /
//     turn projector / rewrite_tail / result_summary / external_delivery / unfinished_round）。
//   - agent_history*.go：Agent 历史门面、读取、overlay 与共享模型。
//   - transcript_*.go：transcript cache、reader、path、session、project、marker 与 guidance。
//   - input_queue.go / input_queue_codec.go / input_queue_replay.go：输入队列存取、编解码与事件重放。
//   - room_history.go / room_directed_message.go / session_file.go / jsonl.go：
//     房间历史、定向消息、会话文件与 JSONL。
//   - paths.go / transcript_project_hash.go / value_coerce.go：路径、工程 hash、值转换。
//
// 历史投影与持久化共享未导出模型；在形成稳定边界前保留同包，避免为拆目录暴露内部状态。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package workspace
