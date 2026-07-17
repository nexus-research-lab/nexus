// Package migration 执行数据库之外、随部署版本交付的一次性数据迁移。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - workspace_files.go：工作区文件迁移账本、顺序执行与完成标记。
//   - legacy_memory.go：旧记忆会话目录与旧记忆根目录迁移。
//   - legacy_memory_skill.go：旧版内置 memory-manager Skill 精确清理迁移。
//   - retired_skills.go：已退役系统 Skill 清理迁移。
//
// 暴露接口：RunWorkspaceFiles。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package migration
