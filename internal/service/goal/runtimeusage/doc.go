// Package runtimeusage 将 runtime 结果与消息转换为 Goal 用量证据。
//
// L2 | 父级: internal/service/goal（L1 见 AGENTS.md）
//
// 成员清单：
//   - snapshot.go：DM/Room 共用的逐 turn、累计终态与子任务观察转换。
//   - snapshot_test.go：显式零与缺失用量、assistant 回退、子任务身份与终态证据。
//
// 本包依赖 runtime、message 与 Goal；Goal 主包不依赖本包。
// 宿主负责可信会话身份、锁、作用域绑定、持久化和重试；本包只转换观察值。
//
// [PROTOCOL]: 变更时检查父级 doc.go 与 execution-orchestration-spec.md。
package runtimeusage
