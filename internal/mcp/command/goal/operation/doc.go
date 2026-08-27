// Package operation 实现 Goal command 的固定语义操作集。
//
// L2 | 父级: internal/mcp/command/goal（L2 见其 doc.go）
//
// 成员清单：
//   - goal.go：读取、创建、重定向、终态更新及共用的 exact Goal/revision authority fence。
//   - alignment.go：结构化目标对齐审计。
//   - registry.go / result.go：工具注册、检索元数据、入参 schema 与 structured result 构造。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package operation
