// Package agent 提供 Agent 业务能力：CRUD、运行时提示词构建、workspace/skills 就绪。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / crud.go / deletion_coordination.go / prompt.go / ready.go /
//     skills.go / workspace.go：Service、分阶段 Agent 更新、跨域删除协调、Skill
//     全局启用与 workspace 停用列的独立原子更新、runtime_version CAS 与计数；删除先
//     提交数据库身份撤销，再由 app 协调 Channel 与全部 DM/Room runtime 墓碑撤销，
//     后置失败显式返回 reconcile 状态。
//   - contacts.go：同 owner 普通 Agent 的双向联系人、别名与直聊 Room 绑定。
//   - prompt_build.go / prompt_default.go：BuildRuntimePrompt 运行时附加提示词、
//     默认模板与主智能体委派边界。
//   - repository.go / factory_record.go：持久化、默认平台 Skill 引用与记录构造。
//   - emotion_state.go / runtime_settings.go：带 Agent scope 锁与 version CAS 的 runtime 情绪态及 nxs settings 投影。
//   - policy_name.go / scope_owner.go / workspace_path.go：命名策略、归属与 workspace 路径。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package agent
