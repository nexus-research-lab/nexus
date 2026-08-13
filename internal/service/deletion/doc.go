// Package deletion 提供 Session 作用域次级数据的统一删除入口。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - references.go：Session 作用域的 Goal、自动化目标、投递路由与执行图级联清理；
//     单个 Session 删除会停用并保留 Automation 任务等待重绑；Agent/Room 整体删除才级联删除任务。
//
// 各领域先清理 runtime、文件与次级引用，最后删除仍可供重试的主记录。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package deletion
