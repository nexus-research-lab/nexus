// Package llm 提供后端轻量 LLM 调用能力，不依赖 Agent SDK。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - client.go：Client 及按 provider/请求格式构造请求（含 think 关闭分派）。
//   - client_response.go：响应解析。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package llm
