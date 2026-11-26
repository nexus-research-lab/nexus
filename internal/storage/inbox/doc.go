// Package inbox 持久化常驻对话的 inbox 事件。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - model.go：事件类型（用户消息 / 定时任务到点）、payload 与状态词汇。
//   - repository.go：按 dedupe_key 幂等追加（进程内串行分配 seq，单实例前提）、按 seq 列出未完成事件，以及只走条件更新的状态迁移。
//
// 暴露接口：Event、Payload、Principal、TaskSource、Kind*、Status*、Repository、NewRepository。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package inbox
