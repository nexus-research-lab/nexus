// Package executionmcp exposes the model-facing Execution Orchestration tools.
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员:
//   - contract: trusted runtime identity and the orchestration service port.
//   - tool: fixed semantic adapters；prepare_plan_execution 以单个 strict Nexus Plan Document v1
//     YAML string 和 none/current/inherit Goal binding intent seal durable non-authoritative
//     proposal；current 只消费 exact round Goal authority，plan_execution 只用 proposal id+digest
//     在 exact authority/Execution/Plan/Goal fence 下原子 materialize；Goal successor/predecessor
//     identity 与 activation 一起 seal，commit 重新校验 sealed trusted Goal state。
//   - server.go: SDK MCP server assembly.
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package executionmcp
