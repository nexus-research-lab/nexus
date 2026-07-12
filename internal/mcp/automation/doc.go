// Package automationmcp 提供 nexus_automation 内建 MCP server 入口。
// 模型直接通过工具检索发现意图级能力，定时任务不再维护重复的 Skill 路由层。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go：NewServer 按当前会话上下文构建定时任务 MCP server。
//   - contract/：服务依赖与会话上下文契约。
//   - tool/：八个面向用户意图的模型工具。
//   - internal/：参数、构建、语义默认值和结果渲染。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automationmcp
