// Package trace 把 SDK 消息解析为调试日志字段与单行摘要。
//
// L2 | 父级: internal/runtime（L1 见 AGENTS.md）
//
// 成员清单：
//   - message.go：SDKMessageLogOptions、DefaultSDKMessageLogOptions、BuildSDKMessageLogFields 入口。
//   - fields.go：各消息类型的日志字段构建。
//   - summary.go：BuildSDKMessageLogSummary 单行摘要与各 summarize* 逻辑。
//   - value.go：RawMap / RawString / FirstNonEmpty 等 raw SDK 值解析工具（供 runtime、exec 复用）。
//
// 本包只依赖 SDK 协议类型，不反向依赖 runtime/exec，可被两者安全引用。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package trace
