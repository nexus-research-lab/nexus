// Package runtime 装配 Nexus 进程向 Agent runtime 暴露的宿主能力。
//
// L2 | 父级: internal/app/server（L2 见 ../doc.go）
//
// 成员清单：
//   - command.go / builtin_tools.go / *_mcp.go：round-scoped nexus MCP、内建工具与显式选择的第三方 Connector MCP（含 RichMail）。
//   - configuration.go：nexuscfg loopback 配置 broker。
//   - *_authorization.go / mcp_authority.go：真人授权与可信 runtime 身份边界。
//   - human_tool_approval.go：高风险工具人工批准路由。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 ../doc.go（L2）
package runtime
