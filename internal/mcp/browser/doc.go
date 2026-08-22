// Package browser 把完整浏览器操作暴露为单个 Nexus MCP 工具。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go：从 Browser service 生成单动作与批处理 schema、调用服务并渲染文字/截图/PDF 结果。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package browser
