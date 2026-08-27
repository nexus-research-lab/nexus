// Package tool 把持久 Channel 授权适配成单一、严格且不暴露 scope 的 action MCP 工具。
//
// L2 | 父级: internal/mcp/channelauthorization（L1 见 AGENTS.md）
//
// 成员：
//   - registry.go：owner-main 私有 DM 可见性与四个 action 的薄适配。
//   - schema.go / helpers.go：严格 schema 与纯 transport 渲染。
//
// 验证码刻意不出现在任何 schema 中。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级 doc.go（L2）
package tool
