// Package runtimecommand 定义 Agent-facing nexus CLI 的共享 command wire、round capability 与 typed mutation receipt。
//
// L2 | 父级: internal/cli（L1 见 AGENTS.md）
//
// 成员:
//   - command.go: transport-neutral request、contract、operation 与 result。
//   - input_schema.go: 在领域 handler 前执行 Goal/Execution/Computer Use 共用的 portable JSON Schema 校验。
//   - capability.go: physical-round Actor capability registry。
//   - round_resources.go: 与 physical round 同寿命的临时资源所有权。
//   - attempt_state.go: 跨 command registry rebuild 保持的 round-local 重试计数。
//   - managed.go: Goal/Execution/Computer Use 领域与内置 Skill 的唯一绑定目录。
//   - execution/operation: WorkGraph mutation 与命名工作图保存操作。
//   - receipt.go: Goal/Execution/命名工作图保存 mutation 的宿主侧 typed receipt。
//
// [PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md（L1）
package runtimecommand
