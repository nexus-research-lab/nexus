// Package clientopts 组装 SDK runtime client 的启动选项、provider 解析与环境。
//
// L2 | 父级: internal/runtime（L1 见 AGENTS.md）
//
// 成员清单：
//   - agent_client.go / runtime_env.go / runtime_profile.go：client 选项、模型上限环境与 profile。
//   - model_provider.go：运行时 Provider、模型能力与上下文上限解析结果。
//   - log_runtime.go：runtime 日志选项。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package clientopts
