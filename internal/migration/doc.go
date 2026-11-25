// Package migration 执行随部署版本交付的启动兼容修复与数据库外一次性数据迁移。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - state_layout.go：把旧状态根安全迁入 app 与 users 前置目录，支持跨版本直升。
//   - workspace_layout.go：按 owner 重排旧 workspace 并同步 Agent 路径。
//   - skipped_state_layout.go：修复 v0.1.30 根目录迁移缺口并安全合并错误窗口的新数据。
//   - permissions.go：统一桌面、普通服务与 Linux enforce 的迁移权限职责。
//   - public_skills.go：将平台/宿主 Skill 投影迁至独立 skills-public 根，不覆盖冲突数据。
//   - workspace_files.go：工作区文件迁移账本、顺序执行与完成标记。
//   - SQL 00128：把旧自定义 MCP 的隐式连接状态一次性迁成显式 enabled 可用性字段。
//   - SQL 00129 / connector_credentials.go：为 Connector connection 密文增加稳定 key_id，
//     并用 active/legacy keyring 逐条 CAS 识别或重加密，未知密钥保留待恢复。
//   - SQL 00130：为个人微信账号增加不透明 iLink 轮询游标，使重启后继续精确增量拉取。
//   - SQL 00131/00132：建立 Control 用户到本地 owner 的稳定绑定与 owner profile 投影。
//   - conversation_draft_repair.go：桌面 SQLite 升级期按 canonical 用户输入收口旧空白 Session，并以 started 标记阻止自动重扫。
//   - runtime_identity.go：Linux owner 到 OS UID/GID、私有组与用户 ACL 的启动同步。
//   - runtime_identity_hardlinks*.go：Linux 存量跨用户/项目硬链接的 fail-closed 检查。
//   - legacy_memory.go：旧记忆会话目录与旧记忆根目录迁移。
//   - legacy_memory_skill.go：旧版内置 memory-manager Skill 精确清理迁移。
//   - retired_skills.go：已退役系统 Skill 清理迁移。
//   - retired_sandbox_resources.go：旧版 macOS 桌面沙箱监督根与带租约标记 scratch 的尽力清理；失败只告警并下次重试。
//   - SQL 00160–00162 / workgraph_history*.go：冻结旧 Execution 回填集合、读取分类/恢复/连线目标索引及 canonical transcript 的可重试 Subagent 回填。
//   - SQL 00169：Heartbeat 退役，删除配置表、wake outbox 行与 00125 的 wake 列/部分索引。
//   - SQL 00159：删除无状态沙箱后已无读取方的启动记录、策略回执与 scratch 恢复表。
//   - provider_scope_recovery.go：桌面 App 本地 SQLite 的旧 Provider scope 数据补偿。
//   - state_root.go / state_root_metadata.go：桌面整体状态根复制后的数据库、transcript、Room 与 Session 删除恢复路径提交。
//   - room_files.go：旧 app/rooms 到用户 state/rooms 与 workspace/.rooms 的 owner 级迁移。
//   - room_files_hardlink_*.go：跨平台 Room 文件迁移硬链接校验。
//
// 暴露接口：RunStateLayout、RunWorkspaceLayout、MergeSkippedStateLayoutDatabase、MergeSkippedStateLayoutUsers、RunDesktopStateRootRebase、RunWorkspaceFiles、RunRoomFiles、RunDesktopLegacyConversationDraftRepair、RunRuntimeIdentitySync、RepairDesktopProviderScope、RunConnectorCredentialKeyMigration。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package migration
