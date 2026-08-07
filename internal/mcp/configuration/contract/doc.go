// Package contract 定义 nexus_config MCP 与配置服务之间的可信 runtime 上下文和最小接口。
//
// L2 | 父级: internal/mcp/configuration
//
// 成员清单：
//   - contract.go：服务端注入的 Agent/DM/Room 业务身份、真实 runtime lease、Actor 投影与四工具所需服务接口。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 doc.go
package contract
