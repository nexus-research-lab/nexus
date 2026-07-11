// Package feishudocx 封装飞书云文档 API 客户端与 Markdown 渲染。
//
// L2 | 父级: internal/connectors（L1 见 AGENTS.md）
//
// 成员清单：
//   - client.go：客户端构造、鉴权与 HTTP 传输。
//   - document.go / bitable.go / drive.go / sheet.go / wiki.go / search.go：能力、模型与目标解析。
//   - block_codec.go：SDK 对象与开放 JSON 结构转换。
//   - render_markdown.go / render_util.go：block 类型表驱动的 Markdown 渲染。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package feishudocx
