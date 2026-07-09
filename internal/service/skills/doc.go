// Package skills 提供技能目录、安装、卸载与 marketplace 检索能力。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / service_catalog.go / service_registry*.go / service_file.go / service_workspace.go：Service、目录、注册表、文件、workspace。
//   - marketplace_*.go：外部 marketplace 检索、导入、预览、更新、源配置（git / skills.sh / URL）。
//   - frontmatter.go / model_skill.go：frontmatter 解析与技能模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package skills
