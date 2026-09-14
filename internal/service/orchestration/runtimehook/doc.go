// Package runtimehook 适配 Execution 的子任务 hook 与 DM/Room 运行观察事件。
//
// L2 | 父级: internal/service/orchestration（L1 见 AGENTS.md）
//
// 成员清单：
//   - hook.go：宿主 runtime/Room identity 闭包、PreToolUse 准入、Subagent
//     lifecycle 转发、结构化拒绝投影与内部持久化错误日志。
//   - observer.go / observer_test.go：共用运行开始、消息、命令回执、附件和结束观察；
//     保留可选能力、独立三秒超时及失败不阻断运行的边界，验证 actor 与事件透传。
//     活动 round 的 compact 边界沿用独立证据调用，以物理 Session/Agent round 生成稳定幂等 ID，不扩展到 idle 观察。
//
// observer.go 在入口将宿主 Execution MCP 回执转换为 RuntimeCommandFact，保留可信请求与责任身份。
//
// [PROTOCOL]: 变更时更新此头部，然后检查上级 orchestration/doc.go（L2）
package runtimehook
