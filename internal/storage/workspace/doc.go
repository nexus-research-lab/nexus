// Package workspace 是 Agent 历史、transcript、输入队列与房间历史的持久化层。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - history_*.go：历史投影（compact / normalize / order / pagination / round_index /
//     turn_projection / rewrite_tail / result_summary / external_delivery / unfinished_round）。
//   - store_agent_history*.go：Agent 历史存取、overlay、transcript（cache / reader / path /
//     session / project / marker / guidance）。
//   - store_input_queue*.go：输入队列存取（codec / file / order / replay）。
//   - store_room_history.go / store_room_directed_message.go / store_session_file.go / store_jsonl.go：房间历史、定向消息、会话文件、JSONL。
//   - paths.go / transcript_project_hash.go / value_coerce.go：路径、工程 hash、值转换。
//
// 注：本包 36 文件、职责偏多，是后续按 history / transcript / inputqueue 拆子包的候选。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package workspace
