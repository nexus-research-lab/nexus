// Package clientopts 组装 SDK runtime client 的启动选项、provider、MCP servers 与环境。
//
// L2 | 父级: internal/runtime（L1 见 AGENTS.md）
//
// 成员清单：
//   - agent_client.go / runtime_env.go：client 选项、nxs/Claude
//     Skill 动态发现与显式停用投影、主模型配置解析、同 Provider 后台进度模型回退、provider 协议环境、
//     按 owner 锁定的 workspace/长期记忆环境、nexuscfg / Agent-facing nexus
//     physical-round capability、按 runtime 隔离的模型上限环境、Provider 结果与 profile。
//   - mcp_servers.go：严格解析 Agent 持久化 stdio/http/sse MCP 配置并在禁止覆盖内建名称的前提下合并。
//   - web_search.go：runtime 自有的 WebSearch 配置与环境投影。
//   - log_runtime.go：runtime 日志选项。
//   - runtime_admission.go：认证转场到 Agent runtime admission 与强隔离要求的动态依赖边界。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package clientopts
