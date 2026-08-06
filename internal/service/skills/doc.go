// Package skills 提供全局技能库、Agent 启停、workspace 本地投影与 marketplace 检索能力。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / catalog.go / registry*.go / references.go / file.go / workspace.go / confined_files.go：
//     Service、用户全局 catalog、Agent 私有 workspace 投影、来源/存储投影、
//     Agent 使用矩阵、原子开关、平台/外部 Skill 引用、owner-scoped confined registry。
//   - marketplace_*.go：外部 marketplace 与私有 JSON 注册表的检索、凭据、导入、预览和更新（git / skills.sh / URL / private registry）。
//   - frontmatter.go / model_skill.go：frontmatter 解析、正文投影与技能模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package skills
