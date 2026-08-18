// Package runtimecommand 定义 Agent-facing nexus CLI 的共享 command wire、round capability 与 typed mutation receipt。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员:
//   - command.go: transport-neutral request、contract、operation 与 result。
//   - capability.go: physical-round Actor capability registry。
//   - round_resources.go: 与 physical round 同寿命的临时资源所有权。
//   - attempt_state.go: 跨 command registry rebuild 保持的 round-local 重试计数。
//   - managed.go: Goal/Execution 领域与内置 Skill 的唯一绑定目录。
//   - receipt.go: Goal/Execution mutation 的宿主侧 typed receipt。
//
// [PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md（L1）
package runtimecommand
