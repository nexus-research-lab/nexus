// Package runtime 装配 Nexus 进程向 Agent runtime 暴露的宿主能力。
//
// L2 | 父级: internal/app（L2 见 ../doc.go）
//
// 成员清单：
//   - builtin_tools.go：同时装配绑定当前 Agent/round 的文件交付工具，不接受模型选择产出身份。
//   - command.go / builtin_tools.go / *_mcp.go：round-scoped nexus MCP、内建工具与显式选择的第三方 Connector MCP（含 RichMail 与复用 OAuth 授权的 GitHub 远程 MCP）。
//   - command.go：从 SDK 可信 metadata 绑定原始工具 identity，分发 subagent 原生控制。
//   - configuration.go：nexuscfg loopback 配置 broker。
//   - *_authorization.go / mcp_authority.go：真人授权与可信 runtime 身份边界。
//   - human_tool_approval.go：高风险工具人工批准路由。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 ../doc.go（L2）
package runtime
