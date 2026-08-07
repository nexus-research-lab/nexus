// Package deletion 提供跨数据库与文件系统删除操作的持久清单和幂等收口。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - coordinator.go：删除任务登记、失败保留、完成收口与待恢复任务查询。
//   - references.go：Session 作用域的 Goal、自动化目标、投递路由与执行图级联清理。
//
// 删除任务先于主记录落库；主记录消失后仍能依赖 payload 重放文件与运行时清理。
// 完成后删除任务本身，失败则保留 attempts/last_error 供启动恢复器继续执行。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package deletion
