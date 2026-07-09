// Package agent 提供 Agent 业务能力：CRUD、运行时提示词构建、workspace/skills 就绪。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / service_crud.go / service_prompt.go / service_ready.go /
//     service_skills.go / service_workspace.go：Service 及各业务切面。
//   - builder_prompt.go / builder_prompt_default.go：BuildRuntimePrompt 运行时附加提示词（含默认模板）。
//   - repository.go / factory_record.go：持久化与记录构造。
//   - emotion_state.go / policy_name.go / scope_owner.go / paths_workspace.go：情绪态、命名策略、归属、workspace 路径。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package agent
