// Package workgraphworkflow 从完成态 managed Execution 生成临时结构草图，并在用户确认后调度隐藏 Agent round 保存可复用命名 WorkGraph。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：完成图预览、owner/session-scoped 临时 preview、受管 CLI 幂等保存、读取/删除、Slash descriptor 投影与 runtime prompt 展开。
//   - abstraction.go：使用默认后台模型自动选择源 logical-key 子集、抽象通用语义，并校验主路径/terminal 与来源边界。
//   - metadata_editor.go：从源 transcript 最近完成助手轮次派生隐藏短期 DM 分支，通过唯一受限 MCP 对完整草图做版本化修改与应用。
//
// 编辑分支可修改文案、节点和依赖，但必须通过 DAG、父子结构、key 主路径与 terminal 交付校验；HTTP 不直接持久化，只用 HiddenFromUser 内部 round 把 exact preview_id 交给 Skill + CLI。运行、工具与交付历史始终留在源 Execution。
package workgraphworkflow
