// Package agent 提供 Agent 业务能力：CRUD、运行时提示词构建、workspace/skills 就绪。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / crud.go / prompt.go / ready.go /
//     skills.go / workspace.go：Service、分阶段 Agent 更新及各业务切面。
//   - prompt_build.go / prompt_default.go：BuildRuntimePrompt 运行时附加提示词与默认模板。
//   - repository.go / factory_record.go：持久化与记录构造。
//   - emotion_state.go / runtime_settings.go：runtime 情绪态与 nxs settings 投影。
//   - policy_name.go / scope_owner.go / workspace_path.go：命名策略、归属与 workspace 路径。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package agent
