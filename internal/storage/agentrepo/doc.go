// Package agentrepo 是带 runtime 配置版本控制的 Agent 记录跨方言 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - sql.go：按 owner 隔离的 Agent SQL 读写、全局/本地 Skill 列级窄更新及版本 CAS、
//     Channel/account/pairing/version 事务删除与 runtime_version CAS。
//   - creation_request.go：owner/request 唯一 reservation、lease claim、workspace stage fence、
//     Agent/Profile/Runtime+回执原子提交与删除墓碑；不保存完整表单或 HTTP 诊断 ID。
//   - contacts.go：同 owner 双向联系人、别名与直聊 Room 绑定。
//   - model.go / scan.go：含预期版本、创建 claim/stage、独立业务标签、Skill 启用/停用集合的落库记录模型，
//     以及含当前版本的行扫描。
//   - sql.go 同时提供稳定的 runtime 版本冲突错误。
//
// SQLRepository 根据 driver 选择 SQLDialect，不在上层复制 SQLite/PostgreSQL 门面。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package agentrepo
