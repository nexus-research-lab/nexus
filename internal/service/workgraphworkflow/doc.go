// Package workgraphworkflow 提供只读内置 WorkGraph 模板，并从完成态 managed Execution 生成可恢复版本化 Draft，在用户确认后保存可复用命名 WorkGraph。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - builtins.go：系统内置的双语通用责任拓扑；只进入目录、Slash catalog 与 runtime 展开，不写数据库或携带运行事实。
//   - service.go / save_confirmation.go：完成图抽取/复用、用户确认后的直接事务保存、按完整草图内容核验的受管 command 幂等保存、读取/删除、Slash descriptor 投影与 runtime prompt 展开；旧请求不得推进新草图的保存标记，exact editor revision 冲突在写入前关闭。
//   - slash_name_availability.go：按 owner 与 exact Draft 判断命名 Slash 是否被固定命令或其他命名图占用。
//   - abstraction.go：把完整源节点/拓扑交给默认对话模型，默认保留结构关键 logical key、抽象具体任务语义，并校验主路径/terminal/关键节点与来源边界。
//   - draft_state.go / authoring.go：持久 Draft cache 恢复、一个 Session 多来源图目录，以及普通 DM/Room 的查询、提取、完整修订、版本选择和保存能力。
//   - metadata_editor.go：由 owner 的 Nexus 主智能体承载目录隐藏专用 DM，不继承源 transcript/权限；恢复已有编辑会话也先保留保存表单的元信息修改，通过 execution-orchestrator Skill + round-scoped nexus.command 对完整草图做版本化修改与应用，允许必要的 AskUserQuestion；版本选择不重写草图。
//
// 内置模板只读且拥有稳定 Slash；历史同名 owner 保存图在升级后优先，避免命令语义被替换。Draft 按 exact source Execution 去重并保存不可变版本、head CAS 与 selected preference；关闭编辑 UI 不删除隐藏会话。修改必须通过 DAG、父子结构、key 主路径与 terminal 交付校验。UI 对已生成草图与元信息确认直接执行数据库事务，不启动模型 round；普通对话继续通过 Skill/command 在用户明确确认后调用同一事务保存边界。运行、工具与交付历史始终留在源 Execution。
// 已保存命名图按 origin_workflow_id 独立恢复草稿，抽取来源仍唯一；保存查询核对完整命名内容，Apply/Confirm 同时 fence head 与 selected revision。
// 模型修订回执直接投影本次持久提交的 Draft，不从可被并发刷新替换的编辑缓存重建版本。
package workgraphworkflow
