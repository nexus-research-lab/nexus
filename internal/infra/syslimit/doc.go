// Package syslimit 探测并提升进程文件句柄限制。
//
// L2 | 父级: internal/infra（L1 见 AGENTS.md）
//
// 成员清单：
//   - limit.go：OpenFilesLimitSnapshot 与跨平台入口。
//   - limit_unix.go：Unix 平台实现。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package syslimit
