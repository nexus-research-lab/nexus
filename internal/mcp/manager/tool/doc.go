// Package tool 定义 nexus_manager 按运行时权限动态裁剪的工具。
//
// L2 | 父级: internal/mcp/manager（L1 见 AGENTS.md）
//
// 成员清单：
//   - registry.go / inspect.go：权限工具目录。
//   - agents.go / rooms.go / sessions.go / workspace.go：脱敏只读查询。
//   - schema.go / helpers.go：严格输入 Schema 与薄 transport 适配。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package tool
