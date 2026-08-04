// Package migration 执行随部署版本交付的启动兼容修复与数据库外一次性数据迁移。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - state_layout.go：把旧状态根安全迁入 app 与 users 前置目录，支持跨版本直升。
//   - workspace_layout.go：按 owner 重排旧 workspace 并同步 Agent 路径。
//   - skipped_state_layout.go：修复 v0.1.30 根目录迁移缺口并安全合并错误窗口的新数据。
//   - workspace_files.go：工作区文件迁移账本、顺序执行与完成标记。
//   - agent_disabled_skill_schema.go：SQLite 旧版 00056 编号冲突的启动前 schema 与 Goose 账本兼容修复。
//   - conversation_draft_repair.go：桌面 SQLite 升级期按 canonical 用户输入收口旧空白 Session，并以 started 标记阻止自动重扫。
//   - runtime_identity.go：Linux owner 到 OS UID/GID、私有组与用户 ACL 的启动同步。
//   - runtime_identity_hardlinks*.go：Linux 存量跨用户/项目硬链接的 fail-closed 检查。
//   - legacy_memory.go：旧记忆会话目录与旧记忆根目录迁移。
//   - legacy_memory_skill.go：旧版内置 memory-manager Skill 精确清理迁移。
//   - retired_skills.go：已退役系统 Skill 清理迁移。
//   - provider_scope_recovery.go：桌面 App 本地 SQLite 的旧 Provider scope 数据补偿。
//   - state_root.go / state_root_metadata.go：桌面整体状态根复制后的数据库、transcript 与 Room 路径提交。
//   - room_files.go：旧 app/rooms 到用户 state/rooms 与 workspace/.rooms 的 owner 级迁移。
//   - room_files_hardlink_*.go：跨平台 Room 文件迁移硬链接校验。
//
// 暴露接口：RepairLegacyAgentDisabledSkillSchema、RunStateLayout、RunWorkspaceLayout、MergeSkippedStateLayoutDatabase、MergeSkippedStateLayoutUsers、RunDesktopStateRootRebase、RunWorkspaceFiles、RunRoomFiles、RunDesktopLegacyConversationDraftRepair、RunRuntimeIdentitySync、RepairDesktopProviderScope。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package migration
