// Package migration 执行随部署版本交付的启动兼容修复与数据库外一次性数据迁移。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - public_skills.go：将平台/宿主 Skill 投影迁至独立 skills-public 根，不覆盖冲突数据。
//   - workspace_files.go：工作区文件迁移账本、顺序执行与完成标记。
//   - fs_helpers.go：文件迁移共享的不跟随符号链接的枚举与删除原语。
//   - SQL 00128：把旧自定义 MCP 的隐式连接状态一次性迁成显式 enabled 可用性字段。
//   - SQL 00129 / connector_credentials.go：为 Connector connection 密文增加稳定 key_id，
//     并用 active/legacy keyring 逐条 CAS 识别或重加密，未知密钥保留待恢复。
//   - SQL 00130：为个人微信账号增加不透明 iLink 轮询游标，使重启后继续精确增量拉取。
//   - SQL 00131/00132：建立 Control 用户到本地 owner 的稳定绑定与 owner profile 投影。
//   - runtime_identity.go：Linux owner 到 OS UID/GID、私有组与用户 ACL 的启动同步。
//   - retired_sandbox_resources.go：旧版 macOS 桌面沙箱监督根与带租约标记 scratch 的尽力清理；失败只告警并下次重试。
//   - retired_state_entries.go：孤立 session-memory project、app 内旧记忆入口、空 app/debug、零字节 nexus.sqlite、
//     根级旧 runtime/arg-files、非 Linux 空 shared-workspaces 与已删除迁移完成标记的尽力清理。
//   - app_tool_roots.go：Computer Use 状态迁到 app/computer-use，删除旧 shim 目录 app/.agents。
//   - SQL 00160–00162 / workgraph_history*.go：冻结旧 Execution 回填集合、读取分类/恢复/连线目标索引及 canonical transcript 的可重试 Subagent 回填。
//   - SQL 00169：Heartbeat 退役，删除配置表、wake outbox 行与 00125 的 wake 列/部分索引。
//   - SQL 00174：删除已无读写方的 provider_scope_recovery 账本。
//   - SQL 00159：删除无状态沙箱后已无读取方的启动记录、策略回执与 scratch 恢复表。
//   - state_root.go / state_root_metadata.go：桌面整体状态根复制后的数据库、transcript、Room 与 Session 删除恢复路径提交。
//
// 暴露接口：RunDesktopStateRootRebase、RunWorkspaceFiles、RunRuntimeIdentitySync、RunConnectorCredentialKeyMigration、RunWorkGraphHistoryBackfill。
//
// 2026-10 之前的一次性迁移已删除：更早版本须先升级到 2026-10 之后的发布版，再升级到当前版本。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package migration
