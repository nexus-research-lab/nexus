// Package workgraphworkflow 从完成态 managed Execution 生成可恢复版本化 Draft，并在用户确认后保存可复用命名 WorkGraph。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / save_dispatch.go：完成图抽取/复用、coordinator 绑定的隔离内部保存调度、受管 CLI 幂等保存、读取/删除、Slash descriptor 投影与 runtime prompt 展开。
//   - slash_name_availability.go：按 owner 与 exact Draft 判断命名 Slash 是否被固定命令或其他命名图占用。
//   - abstraction.go：把完整源节点/拓扑交给默认对话模型，默认保留结构关键 logical key、抽象具体任务语义，并校验主路径/terminal/关键节点与来源边界。
//   - draft_state.go / authoring.go：持久 Draft cache 恢复、一个 Session 多来源图目录，以及普通 DM/Room 的查询、提取、完整修订、版本选择和保存能力。
//   - metadata_editor.go：由 owner 的 Nexus 主智能体承载目录隐藏专用 DM，不继承源 transcript/权限，通过 execution-orchestrator Skill + round-scoped CLI 对完整草图做版本化修改与应用。
//
// Draft 按 exact source Execution 去重并保存不可变版本、head CAS 与 selected preference；关闭编辑 UI 不删除隐藏会话。修改必须通过 DAG、父子结构、key 主路径与 terminal 交付校验。UI 保存只在不继承源 transcript 的隐藏内部 DM 中把 exact preview_id 交给 Skill + CLI；普通对话则由同一 Skill/CLI 在用户明确确认后保存。运行、工具与交付历史始终留在源 Execution。
package workgraphworkflow
