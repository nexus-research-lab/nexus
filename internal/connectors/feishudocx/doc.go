// Package feishudocx 封装飞书云文档 API 客户端、模型与 Markdown 渲染。
//
// L2 | 父级: internal/connectors（L1 见 AGENTS.md）
//
// 成员清单：
//   - client*.go：API 客户端（bitable / document / drive / sheet / wiki / search / transport / block codec）。
//   - model_*.go：文档 / 表格 / 多维表格 / 云盘知识库模型。
//   - render_*.go：block / inline / table / markdown 渲染。
//   - target_document.go：目标文档解析。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package feishudocx
