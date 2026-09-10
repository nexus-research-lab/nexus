// Package command 定义 round-scoped nexus.command MCP 工具的请求协议、可信 Actor 与领域操作适配。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员:
//   - contract.go / input_schema.go：领域请求、contract、operation、result 与字段类型、字符长度、数值及数组上下界校验。
//   - actor.go：模型不能覆盖的 physical-round Actor capability。
//   - tool.go：单一 nexus MCP server 中 command 工具的 schema、request_id 格式与条件必填说明、纠错和回执适配。
//   - subagent/：子任务 contract、同轮去重与宿主原生控制回连。
//   - goal/：Goal 服务窄契约与固定语义操作。
//   - execution/：Execution/WorkGraph 服务窄契约与固定语义操作。
//
// physical-round 共用上下文、重试状态和 typed receipt 位于父级 internal/mcp；
// 托管 Goal/Execution Skill 绑定目录位于 internal/service/agent。
//
// [PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md（L1）
package command
