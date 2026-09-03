// Package migration 执行随部署版本交付的启动兼容修复与数据库外一次性数据迁移。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - state_layout.go：把旧状态根安全迁入 app 与 users 前置目录，支持跨版本直升。
//   - workspace_layout.go：按 owner 重排旧 workspace 并同步 Agent 路径。
//   - skipped_state_layout.go：修复 v0.1.30 根目录迁移缺口并安全合并错误窗口的新数据。
//   - permissions.go：统一桌面、普通服务与 Linux enforce 的迁移权限职责。
//   - workspace_files.go：工作区文件迁移账本、顺序执行与完成标记。
//   - agent_disabled_skill_schema.go：SQLite 旧版 00056 编号冲突的启动前 schema 与 Goose 账本兼容修复。
//   - private_skill_schema.go：旧版私有 Skill 00061 与 Execution 迁移编号冲突的精确识别、账本迁移与顺序补跑。
//   - automation_permission_schema.go：旧版定时任务权限 00071 与私有 Skill 00071/权限 00086 的完整 schema 识别、账本修复与顺序补跑。
//   - goal_schema.go：旧 Goal 分支 00087-00089 与 main 联系人/Automation 迁移的完整 schema 识别、账本映射与顺序补跑。
//   - agent_creation_schema.go：旧 Automation/Agent 恢复迁移 00121-00125 与 Agent 标签 00121 的连续 schema 识别、账本映射与顺序补跑。
//   - control_schema.go：旧 Control 分支 00128/00129 与正式 Connector 迁移的完整 schema 识别、账本映射与顺序补跑。
//   - SQL 00128：把旧自定义 MCP 的隐式连接状态一次性迁成显式 enabled 可用性字段。
//   - SQL 00129 / connector_credentials.go：为 Connector connection 密文增加稳定 key_id，
//     并用 active/legacy keyring 逐条 CAS 识别或重加密，未知密钥保留待恢复。
//   - SQL 00130：为个人微信账号增加不透明 iLink 轮询游标，使重启后继续精确增量拉取。
//   - SQL 00131/00132：建立 Control 用户到本地 owner 的稳定绑定与 owner profile 投影。
//   - execution_identity.go：补齐早期已应用 00061 但缺少 Goal/Execution identity claim table 的启动前兼容修复。
//   - conversation_draft_repair.go：桌面 SQLite 升级期按 canonical 用户输入收口旧空白 Session，并以 started 标记阻止自动重扫。
//   - runtime_identity.go：Linux owner 到 OS UID/GID、私有组与用户 ACL 的启动同步。
//   - runtime_identity_hardlinks*.go：Linux 存量跨用户/项目硬链接的 fail-closed 检查。
//   - legacy_memory.go：旧记忆会话目录与旧记忆根目录迁移。
//   - legacy_memory_skill.go：旧版内置 memory-manager Skill 精确清理迁移。
//   - retired_skills.go：已退役系统 Skill 清理迁移。
//   - provider_scope_recovery.go：桌面 App 本地 SQLite 的旧 Provider scope 数据补偿。
//   - state_root.go / state_root_metadata.go：桌面整体状态根复制后的数据库、transcript、Room 与 Session 删除恢复路径提交。
//   - room_files.go：旧 app/rooms 到用户 state/rooms 与 workspace/.rooms 的 owner 级迁移。
//   - room_files_hardlink_*.go：跨平台 Room 文件迁移硬链接校验。
//
// 暴露接口：RepairLegacyAgentDisabledSkillSchema、RepairLegacyPrivateSkillMigrationCollision、RepairLegacyAutomationPermissionMigrationCollision、RepairLegacyGoalMigrationCollision、RepairLegacyAgentCreationMigrationCollision、RepairLegacyControlMigrationCollision、RepairLegacyExecutionIdentityClaimSchema、RunStateLayout、RunWorkspaceLayout、MergeSkippedStateLayoutDatabase、MergeSkippedStateLayoutUsers、RunDesktopStateRootRebase、RunWorkspaceFiles、RunRoomFiles、RunDesktopLegacyConversationDraftRepair、RunRuntimeIdentitySync、RepairDesktopProviderScope、RunConnectorCredentialKeyMigration。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package migration
