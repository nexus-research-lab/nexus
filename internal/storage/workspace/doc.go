// Package workspace 是 Agent 历史、transcript、输入队列与房间历史的持久化层。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - history_*.go / user_input_probe.go：分阶段历史投影与严格只读用户输入判定（compact / normalize / causal control ordering / pagination / last-write round_index /
//     turn projector / rewrite_tail / 按 Agent 执行轮配对的 result_summary / 同 message_id 合并 Goal 完成收据 / external_delivery / 不为完成态 host control 伪造中断 assistant 的 unfinished_round）。
//   - agent_history*.go：Agent 历史门面、读取、overlay 与共享模型。
//   - runtime_repair.go：enforce 模式下 owner runtime 权限修复与受限重试。
//   - transcript_*.go：transcript cache、重复 UUID/自指链修复、reader、path、session、project、
//     可见性安全的 marker 对齐、guidance 与 root/source round 投影。
//   - input_queue.go / input_queue_codec.go / input_queue_replay.go：输入队列存取、携带非授权 Goal collaboration attribution、完整 Execution WorkBinding 或独立 ReviewBinding 的跨派发持久幂等入队、责任项禁止 guide/合并的 capability envelope fence、可返回规范化提交的原子批量登记、预检版本一致的整批 conversation guidance 认领、按执行 scope 隔离的编解码与事件重放。
//   - room_history.go / room_directed_message.go / room_directed_message_wake.go / session_file.go / artifact_probe.go / jsonl.go：
//     保留 Agent 执行身份且按 ledger 文件版本缓存可见消息计数的房间历史、可反向修复两阶段写入且按稳定 message_id 幂等的 Goal-attributed 定向消息/Room handoff（含 terminal/Goal handback 独立阶段和审计约束的 legacy attribution repair）、immediate/delayed schedule-complete 唤醒、带最后一次上下文占用快照的会话文件、只读目录证据探测与 JSONL。
//   - permissions.go / confined_path.go / transcript_confined.go：enforce 新建权限、
//     owner/workspace 根绑定和 transcript confined-fd 访问。
//   - paths.go / transcript_path.go / transcript_project_hash.go / value_coerce.go：
//     路径、transcript 项目目录名、工程 hash、值转换。
//
// 历史投影与持久化共享未导出模型；在形成稳定边界前保留同包，避免为拆目录暴露内部状态。
// TranscriptProjectDirectoryName/Names 是迁移层复用的稳定路径边界；
// ReadTranscriptSessionMessages 用受控 session id 读取独立 Agent thread，
// ReadTranscriptLinkMessages 是 Claude Code runtime 输出链接唯一允许的双重 confined 读取入口。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package workspace
