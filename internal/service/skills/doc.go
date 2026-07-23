// Package skills 提供技能目录、安装、卸载与 marketplace 检索能力。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / catalog.go / registry*.go / references.go / file.go / workspace.go：Service、平台/外部 Skill 引用、目录、用户级源、文件、workspace。
//   - marketplace_*.go：外部 marketplace 检索、导入、预览、更新与候选源评分（git / skills.sh / URL）。
//   - frontmatter.go / model_skill.go：frontmatter 解析、正文投影与技能模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package skills
